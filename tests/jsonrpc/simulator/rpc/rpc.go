package rpc

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/google/go-cmp/cmp"
	"github.com/status-im/keycard-go/hexutils"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/contracts"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

// GethVersion is the version of the Geth client used in the tests
// Update it when go-ethereum of go.mod is updated
const GethVersion = "1.15.10"

type CallRPC func(rCtx *RpcContext) (*types.RpcResult, error)

const (
	// Existing methods
	SendRawTransaction                  types.RpcName = "eth_sendRawTransaction"
	GetBlockNumber                      types.RpcName = "eth_blockNumber"
	GetGasPrice                         types.RpcName = "eth_gasPrice"
	GetMaxPriorityFeePerGas             types.RpcName = "eth_maxPriorityFeePerGas"
	GetChainId                          types.RpcName = "eth_chainId"
	GetBalance                          types.RpcName = "eth_getBalance"
	GetBlockByHash                      types.RpcName = "eth_getBlockByHash"
	GetBlockByNumber                    types.RpcName = "eth_getBlockByNumber"
	GetBlockReceipts                    types.RpcName = "eth_getBlockReceipts"
	GetTransactionByHash                types.RpcName = "eth_getTransactionByHash"
	GetTransactionByBlockHashAndIndex   types.RpcName = "eth_getTransactionByBlockHashAndIndex"
	GetTransactionByBlockNumberAndIndex types.RpcName = "eth_getTransactionByBlockNumberAndIndex"
	GetTransactionReceipt               types.RpcName = "eth_getTransactionReceipt"
	GetTransactionCount                 types.RpcName = "eth_getTransactionCount"
	GetTransactionCountByHash           types.RpcName = "eth_getTransactionCountByHash"
	GetBlockTransactionCountByHash      types.RpcName = "eth_getBlockTransactionCountByHash"
	GetCode                             types.RpcName = "eth_getCode"
	GetStorageAt                        types.RpcName = "eth_getStorageAt"
	NewFilter                           types.RpcName = "eth_newFilter"
	GetFilterLogs                       types.RpcName = "eth_getFilterLogs"
	NewBlockFilter                      types.RpcName = "eth_newBlockFilter"
	GetFilterChanges                    types.RpcName = "eth_getFilterChanges"
	UninstallFilter                     types.RpcName = "eth_uninstallFilter"
	GetLogs                             types.RpcName = "eth_getLogs"
	EstimateGas                         types.RpcName = "eth_estimateGas"
	Call                                types.RpcName = "eth_call"

	// Web3 methods
	Web3ClientVersion types.RpcName = "web3_clientVersion"
	Web3Sha3          types.RpcName = "web3_sha3"

	// Net methods
	NetVersion     types.RpcName = "net_version"
	NetPeerCount   types.RpcName = "net_peerCount"
	NetListening   types.RpcName = "net_listening"

	// Additional Eth methods
	EthProtocolVersion types.RpcName = "eth_protocolVersion"
	EthSyncing         types.RpcName = "eth_syncing"
	EthAccounts        types.RpcName = "eth_accounts"
	EthMining          types.RpcName = "eth_mining"
	EthHashrate        types.RpcName = "eth_hashrate"
	EthCoinbase        types.RpcName = "eth_coinbase"
	EthGetProof        types.RpcName = "eth_getProof"
	EthFeeHistory      types.RpcName = "eth_feeHistory"

	// Personal methods
	PersonalListAccounts types.RpcName = "personal_listAccounts"
	PersonalEcRecover    types.RpcName = "personal_ecRecover"
	PersonalListWallets  types.RpcName = "personal_listWallets"

	// TxPool methods
	TxPoolContent types.RpcName = "txpool_content"
	TxPoolInspect types.RpcName = "txpool_inspect"
	TxPoolStatus  types.RpcName = "txpool_status"

	// Debug methods
	DebugTraceBlockByNumber types.RpcName = "debug_traceBlockByNumber"
	DebugFreeOSMemory       types.RpcName = "debug_freeOSMemory"
	DebugSetGCPercent       types.RpcName = "debug_setGCPercent"
	DebugGetRawBlock        types.RpcName = "debug_getRawBlock"
	DebugGetRawHeader       types.RpcName = "debug_getRawHeader"
	DebugGetRawReceipts     types.RpcName = "debug_getRawReceipts"
	DebugGetRawTransaction  types.RpcName = "debug_getRawTransaction"
	DebugPrintBlock         types.RpcName = "debug_printBlock"

	// Miner methods
	MinerStart        types.RpcName = "miner_start"
	MinerStop         types.RpcName = "miner_stop"
	MinerSetEtherbase types.RpcName = "miner_setEtherbase"
	MinerSetExtra     types.RpcName = "miner_setExtra"
	MinerSetGasPrice  types.RpcName = "miner_setGasPrice"
	MinerSetGasLimit  types.RpcName = "miner_setGasLimit"
	MinerGetHashrate  types.RpcName = "miner_getHashrate"

	// Engine API methods (expected to fail)
	EngineNewPayloadV1        types.RpcName = "engine_newPayloadV1"
	EngineForkchoiceUpdatedV1 types.RpcName = "engine_forkchoiceUpdatedV1"
	EngineGetPayloadV1        types.RpcName = "engine_getPayloadV1"

	// Not implemented methods
	EthCreateAccessList types.RpcName = "eth_createAccessList"
	TraceCall           types.RpcName = "trace_call"
	TraceCallMany       types.RpcName = "trace_callMany"
	TraceTransaction    types.RpcName = "trace_transaction"
	AdminAddPeer        types.RpcName = "admin_addPeer"
	AdminNodeInfo       types.RpcName = "admin_nodeInfo"
)

type RpcContext struct {
	Conf                  *config.Config
	EthCli                *ethclient.Client
	Acc                   *types.Account
	ChainId               *big.Int
	MaxPriorityFeePerGas  *big.Int
	GasPrice              *big.Int
	ProcessedTransactions []common.Hash
	BlockNumsIncludingTx  []uint64
	AlreadyTestedRPCs     []*types.RpcResult
	ERC20Abi              *abi.ABI
	ERC20ByteCode         []byte
	ERC20Addr             common.Address
	FilterQuery           ethereum.FilterQuery
	FilterId              string
	BlockFilterId         string
}

func NewContext(conf *config.Config) (*RpcContext, error) {
	// Connect to the Ethereum client
	ethCli, err := ethclient.Dial(conf.RpcEndpoint)
	if err != nil {
		return nil, err
	}

	ecdsaPrivKey, err := crypto.HexToECDSA(conf.RichPrivKey)
	if err != nil {
		return nil, err
	}

	return &RpcContext{
		Conf:   conf,
		EthCli: ethCli,
		Acc: &types.Account{
			Address: crypto.PubkeyToAddress(ecdsaPrivKey.PublicKey),
			PrivKey: ecdsaPrivKey,
		},
	}, nil
}

func (rCtx *RpcContext) AlreadyTested(rpc types.RpcName) *types.RpcResult {
	for _, testedRPC := range rCtx.AlreadyTestedRPCs {
		if rpc == testedRPC.Method {
			return testedRPC
		}
	}
	return nil

}

func RpcGetBlockNumber(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetBlockNumber); result != nil {
		return result, nil
	}
	blockNumber, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return nil, err
	}

	var warnings []string
	if blockNumber == 0 {
		warnings = append(warnings, "blockNumber is zero")
	}

	status := types.Ok
	if len(warnings) > 0 {
		status = types.Warning
	}

	result := &types.RpcResult{
		Method:   GetBlockNumber,
		Status:   status,
		Value:    blockNumber,
		Warnings: warnings,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetGasPrice(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetGasPrice); result != nil {
		return result, nil
	}

	gasPrice, err := rCtx.EthCli.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, err
	}

	var warnings []string
	if gasPrice == nil || gasPrice.Cmp(big.NewInt(0)) == 0 {
		warnings = append(warnings, "gasPrice is nil or zero")
	}

	status := types.Ok
	if len(warnings) > 0 {
		status = types.Warning
	}

	result := &types.RpcResult{
		Method:   GetGasPrice,
		Status:   status,
		Value:    gasPrice.String(),
		Warnings: warnings,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetMaxPriorityFeePerGas(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetMaxPriorityFeePerGas); result != nil {
		return result, nil
	}

	maxPriorityFeePerGas, err := rCtx.EthCli.SuggestGasTipCap(context.Background())
	if err != nil {
		return nil, err
	}

	var warnings []string
	if maxPriorityFeePerGas.Cmp(big.NewInt(0)) == 0 {
		warnings = append(warnings, "maxPriorityFeePerGas is nil or zero")
	}

	status := types.Ok
	if len(warnings) > 0 {
		status = types.Warning
	}

	result := &types.RpcResult{
		Method:   GetMaxPriorityFeePerGas,
		Status:   status,
		Value:    maxPriorityFeePerGas.String(),
		Warnings: warnings,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetChainId(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetChainId); result != nil {
		return result, nil
	}

	chainId, err := rCtx.EthCli.ChainID(context.Background())
	if err != nil {
		return nil, err
	}

	var warnings []string
	if chainId.Cmp(big.NewInt(0)) == 0 {
		warnings = append(warnings, "chainId is nil")
	}

	status := types.Ok
	if len(warnings) > 0 {
		status = types.Warning
	}

	result := &types.RpcResult{
		Method:   GetChainId,
		Status:   status,
		Value:    chainId.String(),
		Warnings: warnings,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetBalance(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetBalance); result != nil {
		return result, nil
	}

	balance, err := rCtx.EthCli.BalanceAt(context.Background(), rCtx.Acc.Address, nil)
	if err != nil {
		return nil, err
	}

	var warnings []string
	if balance.Cmp(big.NewInt(0)) == 0 {
		warnings = append(warnings, "balance is zero")
	}

	status := types.Ok
	if len(warnings) > 0 {
		status = types.Warning
	}

	result := &types.RpcResult{
		Method:   GetBalance,
		Status:   status,
		Value:    balance.String(),
		Warnings: warnings,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetTransactionCount(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetTransactionCount); result != nil {
		return result, nil
	}

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}

	var warnings []string
	if nonce == 0 {
		warnings = append(warnings, "nonce is zero")
	}

	status := types.Ok
	if len(warnings) > 0 {
		status = types.Warning
	}

	return &types.RpcResult{
		Method:   GetTransactionCount,
		Status:   status,
		Value:    nonce,
		Warnings: warnings,
	}, nil
}

func RpcGetBlockByHash(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetBlockByHash); result != nil {
		return result, nil
	}

	blkNum, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return nil, err
	}

	blk, err := rCtx.EthCli.BlockByNumber(context.Background(), new(big.Int).SetUint64(blkNum))
	if err != nil {
		return nil, err
	}

	block, err := rCtx.EthCli.BlockByHash(context.Background(), blk.Hash())
	if err != nil {
		return nil, err
	}

	if !cmp.Equal(blk, block) {
		return nil, errors.New("implementation error: blockByNumber and blockByHash return different blocks")
	}

	result := &types.RpcResult{
		Method: GetBlockByHash,
		Status: types.Ok,
		Value:  utils.MustBeautifyBlock(types.NewRpcBlock(block)),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetBlockByNumber(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetBlockByNumber); result != nil {
		return result, nil
	}

	blkNum, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return nil, err
	}

	blk, err := rCtx.EthCli.BlockByNumber(context.Background(), new(big.Int).SetUint64(blkNum))
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: GetBlockByNumber,
		Status: types.Ok,
		Value:  utils.MustBeautifyBlock(types.NewRpcBlock(blk)),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcSendRawTransactionTransferValue(rCtx *RpcContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainId, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetChainId,
		Status: types.Ok,
		Value:  rCtx.ChainId.String(),
	})

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.EthCli.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.EthCli.SuggestGasPrice(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetGasPrice,
		Status: types.Ok,
		Value:  rCtx.GasPrice.String(),
	})

	randomRecipient := utils.MustCreateRandomAccount().Address
	value := new(big.Int).SetUint64(1)
	balanceBeforeSend, err := rCtx.EthCli.BalanceAt(context.Background(), rCtx.Acc.Address, nil)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetBalance,
		Status: types.Ok,
		Value:  balanceBeforeSend.String(),
	})

	if balanceBeforeSend.Cmp(value) < 0 {
		return nil, errors.New("insufficient balanceBeforeSend")
	}

	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   rCtx.ChainId,
		Nonce:     nonce,
		GasTipCap: rCtx.MaxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(rCtx.GasPrice, big.NewInt(1000000000)),
		Gas:       21000, // fixed gas limit for transfer
		To:        &randomRecipient,
		Value:     value,
	})

	// TODO: Make signer using types.MakeSigner with chain params
	signer := gethtypes.NewLondonSigner(rCtx.ChainId)
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.EthCli.SendTransaction(context.Background(), signedTx); err != nil {
		return nil, err
	}
	result := &types.RpcResult{
		Method: SendRawTransaction,
		Status: types.Ok,
		Value:  signedTx.Hash().Hex(),
	}
	testedRPCs = append(testedRPCs, result)

	// wait for the transaction to be mined
	tout, _ := time.ParseDuration(rCtx.Conf.Timeout)
	if err = WaitForTx(rCtx, signedTx.Hash(), tout); err != nil {
		return nil, err
	}

	balance, err := rCtx.EthCli.BalanceAt(context.Background(), rCtx.Acc.Address, nil)
	if err != nil {
		return nil, err
	}
	// check if the balance decreased by the value of the transaction (+ gas fee)
	if new(big.Int).Sub(balanceBeforeSend, balance).Cmp(value) < 0 {
		return nil, errors.New("balanceBeforeSend mismatch, maybe the transaction was not mined or implementation is incorrect")
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, testedRPCs...)

	return result, nil
}

func RpcSendRawTransactionDeployContract(rCtx *RpcContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainId, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetChainId,
		Status: types.Ok,
		Value:  rCtx.ChainId.String(),
	})

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.EthCli.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.EthCli.SuggestGasPrice(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetGasPrice,
		Status: types.Ok,
		Value:  rCtx.GasPrice.String(),
	})

	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   rCtx.ChainId,
		Nonce:     nonce,
		GasTipCap: rCtx.MaxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(rCtx.GasPrice, big.NewInt(1000000000)),
		Gas:       10000000,
		Data:      common.FromHex(hex.EncodeToString(contracts.ContractByteCode)),
	})

	// TODO: Make signer using types.MakeSigner with chain params
	signer := gethtypes.NewLondonSigner(rCtx.ChainId)
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.EthCli.SendTransaction(context.Background(), signedTx); err != nil {
		return nil, err
	}
	result := &types.RpcResult{
		Method: SendRawTransaction,
		Status: types.Ok,
		Value:  signedTx.Hash().Hex(),
	}
	testedRPCs = append(testedRPCs, result)

	// wait for the transaction to be mined
	tout, _ := time.ParseDuration(rCtx.Conf.Timeout)
	if err = WaitForTx(rCtx, signedTx.Hash(), tout); err != nil {
		return nil, err
	}

	if rCtx.ERC20Addr == (common.Address{}) {
		return nil, errors.New("contract address is empty, failed to deploy")
	}

	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, testedRPCs...)

	return result, nil
}

func RpcSendRawTransactionTransferERC20(rCtx *RpcContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainId, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetChainId,
		Status: types.Ok,
		Value:  rCtx.ChainId.String(),
	})

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.EthCli.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.EthCli.SuggestGasPrice(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: GetGasPrice,
		Status: types.Ok,
		Value:  rCtx.GasPrice.String(),
	})

	randomRecipient := utils.MustCreateRandomAccount().Address
	data, err := rCtx.ERC20Abi.Pack("transfer", randomRecipient, new(big.Int).SetUint64(1))
	if err != nil {
		log.Fatalf("Failed to pack transaction data: %v", err)
	}

	// Erc20 transfer
	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   rCtx.ChainId,
		Nonce:     nonce,
		GasTipCap: rCtx.MaxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(rCtx.GasPrice, big.NewInt(1000000000)),
		Gas:       10000000,
		To:        &rCtx.ERC20Addr,
		Data:      data,
	})

	// TODO: Make signer using types.MakeSigner with chain params
	signer := gethtypes.NewLondonSigner(rCtx.ChainId)
	signedTx, err := gethtypes.SignTx(tx, signer, rCtx.Acc.PrivKey)
	if err != nil {
		return nil, err
	}

	if err = rCtx.EthCli.SendTransaction(context.Background(), signedTx); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: SendRawTransaction,
		Status: types.Ok,
		Value:  signedTx.Hash().Hex(),
	}
	testedRPCs = append(testedRPCs, result)

	// wait for the transaction to be mined
	tout, _ := time.ParseDuration(rCtx.Conf.Timeout)
	if err = WaitForTx(rCtx, signedTx.Hash(), tout); err != nil {
		return nil, err
	}

	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, testedRPCs...)

	return result, nil
}

func RpcGetBlockReceipts(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetBlockReceipts); result != nil {
		return result, nil
	}

	if len(rCtx.BlockNumsIncludingTx) == 0 {
		return nil, errors.New("no blocks with transactions")

	}

	// TODO: Random pick
	// pick a block with transactions
	blkNum := rCtx.BlockNumsIncludingTx[0]
	rpcBlockNum := rpc.BlockNumber(blkNum)
	receipts, err := rCtx.EthCli.BlockReceipts(context.Background(), rpc.BlockNumberOrHash{BlockNumber: &rpcBlockNum})
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: GetBlockReceipts,
		Status: types.Ok,
		Value:  utils.MustBeautifyReceipts(receipts),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetTransactionByHash(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetTransactionByHash); result != nil {
		return result, nil
	}

	if len(rCtx.ProcessedTransactions) == 0 {
		return nil, errors.New("no transactions")
	}

	// TODO: Random pick
	txHash := rCtx.ProcessedTransactions[0]
	tx, _, err := rCtx.EthCli.TransactionByHash(context.Background(), txHash)
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: GetTransactionByHash,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetTransactionByBlockHashAndIndex(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetTransactionByBlockHashAndIndex); result != nil {
		return result, nil
	}

	if len(rCtx.BlockNumsIncludingTx) == 0 {
		return nil, errors.New("no blocks with transactions")
	}

	// TODO: Random pick
	blkNum := rCtx.BlockNumsIncludingTx[0]
	blk, err := rCtx.EthCli.BlockByNumber(context.Background(), new(big.Int).SetUint64(blkNum))
	if err != nil {
		return nil, err
	}

	if len(blk.Transactions()) == 0 {
		return nil, errors.New("no transactions in the block")
	}

	tx, err := rCtx.EthCli.TransactionInBlock(context.Background(), blk.Hash(), 0)
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: GetTransactionByBlockHashAndIndex,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetTransactionByBlockNumberAndIndex(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetTransactionByBlockNumberAndIndex); result != nil {
		return result, nil
	}

	if len(rCtx.BlockNumsIncludingTx) == 0 {
		return nil, errors.New("no blocks with transactions")
	}

	// TODO: Random pick
	blkNum := rCtx.BlockNumsIncludingTx[0]
	var tx gethtypes.Transaction
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &tx, string(GetTransactionByBlockNumberAndIndex), blkNum, "0x0"); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: GetTransactionByBlockNumberAndIndex,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(&tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetTransactionCountByHash(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetTransactionCountByHash); result != nil {
		return result, nil
	}

	if len(rCtx.BlockNumsIncludingTx) == 0 {
		return nil, errors.New("no transactions")
	}

	// get block
	blkNum := rCtx.BlockNumsIncludingTx[0]
	blk, err := rCtx.EthCli.BlockByNumber(context.Background(), new(big.Int).SetUint64(blkNum))
	if err != nil {
		return nil, err
	}

	var count uint64
	if err = rCtx.EthCli.Client().CallContext(context.Background(), &count, string(GetTransactionCountByHash), blk.Hash()); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: GetTransactionCountByHash,
		Status: types.Ok,
		Value:  count,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetTransactionReceipt(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetTransactionReceipt); result != nil {
		return result, nil
	}

	if len(rCtx.ProcessedTransactions) == 0 {
		return nil, errors.New("no transactions")
	}

	txHash := rCtx.ProcessedTransactions[0]
	receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), txHash)
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: GetTransactionReceipt,
		Status: types.Ok,
		Value:  utils.MustBeautifyReceipt(receipt),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetBlockTransactionCountByHash(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetBlockTransactionCountByHash); result != nil {
		return result, nil
	}

	if len(rCtx.BlockNumsIncludingTx) == 0 {
		return nil, errors.New("no blocks with transactions")
	}

	blkNum := rCtx.BlockNumsIncludingTx[0]
	blk, err := rCtx.EthCli.BlockByNumber(context.Background(), new(big.Int).SetUint64(blkNum))
	if err != nil {
		return nil, err
	}

	count, err := rCtx.EthCli.TransactionCount(context.Background(), blk.Hash())
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: GetBlockTransactionCountByHash,
		Status: types.Ok,
		Value:  count,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetCode(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetCode); result != nil {
		return result, nil
	}

	if rCtx.ERC20Addr == (common.Address{}) {
		return nil, errors.New("no contract address, must be deployed first")
	}

	code, err := rCtx.EthCli.CodeAt(context.Background(), rCtx.ERC20Addr, nil)
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: GetCode,
		Status: types.Ok,
		Value:  hexutils.BytesToHex(code),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetStorageAt(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetStorageAt); result != nil {
		return result, nil
	}

	if rCtx.ERC20Addr == (common.Address{}) {
		return nil, errors.New("no contract address, must be deployed first")
	}

	key := utils.MustCalculateSlotKey(rCtx.Acc.Address, 4)
	storage, err := rCtx.EthCli.StorageAt(context.Background(), rCtx.ERC20Addr, key, nil)
	if err != nil {
		return nil, err
	}

	var warnings []string
	status := types.Ok
	// check storage is zero
	if utils.IsZeroBytes(storage) {
		warnings = append(warnings, "storage is zero bytes, should try another slot")
		status = types.Warning
	}

	result := &types.RpcResult{
		Method:   GetStorageAt,
		Status:   status,
		Value:    hexutils.BytesToHex(storage),
		Warnings: warnings,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcNewFilter(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(NewFilter); result != nil {
		return result, nil
	}

	fErc20Transfer := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(rCtx.BlockNumsIncludingTx[0] - 1),
		Addresses: []common.Address{rCtx.ERC20Addr},
		Topics: [][]common.Hash{
			{rCtx.ERC20Abi.Events["Transfer"].ID}, // Filter for Transfer event
		},
	}
	args, err := utils.ToFilterArg(fErc20Transfer)
	if err != nil {
		return nil, err
	}
	var rpcId string
	if err = rCtx.EthCli.Client().CallContext(context.Background(), &rpcId, string(NewFilter), args); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: NewFilter,
		Status: types.Ok,
		Value:  rpcId,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	rCtx.FilterId = rpcId
	rCtx.FilterQuery = fErc20Transfer

	return result, nil
}

func RpcGetFilterLogs(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetFilterLogs); result != nil {
		return result, nil
	}

	if rCtx.FilterId == "" {
		return nil, errors.New("no filter id, must create a filter first")
	}

	if _, err := RpcSendRawTransactionTransferERC20(rCtx); err != nil {
		return nil, errors.New("transfer ERC20 must be succeeded before checking filter logs")
	}

	var logs []gethtypes.Log
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &logs, string(GetFilterLogs), rCtx.FilterId); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: GetFilterLogs,
		Status: types.Ok,
		Value:  utils.MustBeautifyLogs(logs),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcNewBlockFilter(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(NewBlockFilter); result != nil {
		return result, nil
	}

	var rpcId string
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &rpcId, string(NewBlockFilter)); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: NewBlockFilter,
		Status: types.Ok,
		Value:  rpcId,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	rCtx.BlockFilterId = rpcId

	return result, nil
}

func RpcGetFilterChanges(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetFilterChanges); result != nil {
		return result, nil
	}

	if rCtx.BlockFilterId == "" {
		return nil, errors.New("no block filter id, must create a block filter first")
	}

	// TODO: Make it configurable
	time.Sleep(3 * time.Second) // wait for a new block to be mined

	var changes []interface{}
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &changes, string(GetFilterChanges), rCtx.BlockFilterId); err != nil {
		return nil, err
	}

	status := types.Ok
	warnings := []string{}
	if len(changes) == 0 {
		status = types.Warning
		warnings = append(warnings, "no new blocks")
	}

	result := &types.RpcResult{
		Method:   GetFilterChanges,
		Status:   status,
		Value:    changes,
		Warnings: warnings,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcUninstallFilter(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(UninstallFilter); result != nil {
		return result, nil
	}

	if rCtx.FilterId == "" {
		return nil, errors.New("no filter id, must create a filter first")
	}

	var res bool
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &res, string(UninstallFilter), rCtx.FilterId); err != nil {
		return nil, err
	}
	if !res {
		return nil, errors.New("uninstall filter failed")
	}

	if err := rCtx.EthCli.Client().CallContext(context.Background(), &res, string(UninstallFilter), rCtx.FilterId); err != nil {
		return nil, err
	}
	if res {
		return nil, errors.New("uninstall filter should be failed because it was already uninstalled")
	}

	result := &types.RpcResult{
		Method: UninstallFilter,
		Status: types.Ok,
		Value:  rCtx.FilterId,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcGetLogs(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(GetLogs); result != nil {
		return result, nil
	}

	if _, err := RpcNewFilter(rCtx); err != nil {
		return nil, errors.New("failed to create a filter")
	}

	if _, err := RpcSendRawTransactionTransferERC20(rCtx); err != nil {
		return nil, errors.New("transfer ERC20 must be succeeded before checking filter logs")
	}

	// set from block because of limit
	logs, err := rCtx.EthCli.FilterLogs(context.Background(), rCtx.FilterQuery)
	if err != nil {
		return nil, err
	}

	status := types.Ok
	warnings := []string{}
	if len(logs) == 0 {
		status = types.Warning
		warnings = append(warnings, "no logs")
	}

	result := &types.RpcResult{
		Method:   GetLogs,
		Status:   status,
		Value:    utils.MustBeautifyLogs(logs),
		Warnings: warnings,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEstimateGas(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EstimateGas); result != nil {
		return result, nil
	}

	if rCtx.ERC20Addr == (common.Address{}) {
		return nil, errors.New("no contract address, must be deployed first")
	}

	data, err := rCtx.ERC20Abi.Pack("transfer", rCtx.Acc.Address, new(big.Int).SetUint64(1))
	if err != nil {
		log.Fatalf("Failed to pack transaction data: %v", err)
	}

	msg := ethereum.CallMsg{
		From: rCtx.Acc.Address,
		To:   &rCtx.ERC20Addr,
		Data: data,
	}
	gas, err := rCtx.EthCli.EstimateGas(context.Background(), msg)
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: EstimateGas,
		Status: types.Ok,
		Value:  gas,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RPCCall(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(Call); result != nil {
		return result, nil
	}

	if rCtx.ERC20Addr == (common.Address{}) {
		return nil, errors.New("no contract address, must be deployed first")
	}

	data, err := rCtx.ERC20Abi.Pack("balanceOf", rCtx.Acc.Address)
	if err != nil {
		log.Fatalf("Failed to pack transaction data: %v", err)
	}

	msg := ethereum.CallMsg{
		To:   &rCtx.ERC20Addr,
		Data: data,
	}
	res, err := rCtx.EthCli.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: Call,
		Status: types.Ok,
		Value:  hexutils.BytesToHex(res),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func WaitForTx(rCtx *RpcContext, txHash common.Hash, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond) // Check every 500ms
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout exceeded while waiting for transaction %s", txHash.Hex())
		case <-ticker.C:
			receipt, err := rCtx.EthCli.TransactionReceipt(context.Background(), txHash)
			if err != nil && !errors.Is(err, ethereum.NotFound) {
				return err
			}
			if err == nil {
				rCtx.ProcessedTransactions = append(rCtx.ProcessedTransactions, txHash)
				rCtx.BlockNumsIncludingTx = append(rCtx.BlockNumsIncludingTx, receipt.BlockNumber.Uint64())
				rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, &types.RpcResult{
					Method: GetTransactionReceipt,
					Status: types.Ok,
					Value:  utils.MustBeautifyReceipt(receipt),
				})
				if receipt.ContractAddress != (common.Address{}) {
					rCtx.ERC20Addr = receipt.ContractAddress
				}
				if receipt.Status == 0 {
					return fmt.Errorf("transaction %s failed", txHash.Hex())
				}
				return nil
			}
		}
	}
}

// Web3 method handlers
func RpcWeb3ClientVersion(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "web3_clientVersion")
	if err != nil {
		return &types.RpcResult{
			Method:   Web3ClientVersion,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "Web3",
		}, nil
	}
	return &types.RpcResult{
		Method:   Web3ClientVersion,
		Status:   types.Ok,
		Value:    result,
		Category: "Web3",
	}, nil
}

func RpcWeb3Sha3(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "web3_sha3", "0x68656c6c6f20776f726c64")
	if err != nil {
		return &types.RpcResult{
			Method:   Web3Sha3,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "Web3",
		}, nil
	}
	return &types.RpcResult{
		Method:   Web3Sha3,
		Status:   types.Ok,
		Value:    result,
		Category: "Web3",
	}, nil
}

// Net method handlers
func RpcNetVersion(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "net_version")
	if err != nil {
		return &types.RpcResult{
			Method:   NetVersion,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "Net",
		}, nil
	}
	return &types.RpcResult{
		Method:   NetVersion,
		Status:   types.Ok,
		Value:    result,
		Category: "Net",
	}, nil
}

func RpcNetPeerCount(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "net_peerCount")
	if err != nil {
		return &types.RpcResult{
			Method:   NetPeerCount,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "Net",
		}, nil
	}
	return &types.RpcResult{
		Method:   NetPeerCount,
		Status:   types.Ok,
		Value:    result,
		Category: "Net",
	}, nil
}

func RpcNetListening(rCtx *RpcContext) (*types.RpcResult, error) {
	var result bool
	err := rCtx.EthCli.Client().Call(&result, "net_listening")
	if err != nil {
		return &types.RpcResult{
			Method:   NetListening,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "Net",
		}, nil
	}
	return &types.RpcResult{
		Method:   NetListening,
		Status:   types.Ok,
		Value:    result,
		Category: "Net",
	}, nil
}

// Additional Eth method handlers
func RpcEthProtocolVersion(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "eth_protocolVersion")
	if err != nil {
		return &types.RpcResult{
			Method:   EthProtocolVersion,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "Core Eth",
		}, nil
	}
	return &types.RpcResult{
		Method:   EthProtocolVersion,
		Status:   types.Ok,
		Value:    result,
		Category: "Core Eth",
	}, nil
}

func RpcEthSyncing(rCtx *RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, "eth_syncing")
	if err != nil {
		return &types.RpcResult{
			Method:   EthSyncing,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "Core Eth",
		}, nil
	}
	return &types.RpcResult{
		Method:   EthSyncing,
		Status:   types.Ok,
		Value:    result,
		Category: "Core Eth",
	}, nil
}

func RpcEthAccounts(rCtx *RpcContext) (*types.RpcResult, error) {
	var result []string
	err := rCtx.EthCli.Client().Call(&result, "eth_accounts")
	if err != nil {
		return &types.RpcResult{
			Method:   EthAccounts,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "Core Eth",
		}, nil
	}
	return &types.RpcResult{
		Method:   EthAccounts,
		Status:   types.Ok,
		Value:    result,
		Category: "Core Eth",
	}, nil
}

// Personal method handlers
func RpcPersonalListAccounts(rCtx *RpcContext) (*types.RpcResult, error) {
	var result []string
	err := rCtx.EthCli.Client().Call(&result, "personal_listAccounts")
	if err != nil {
		return &types.RpcResult{
			Method:   PersonalListAccounts,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "Personal",
		}, nil
	}
	return &types.RpcResult{
		Method:   PersonalListAccounts,
		Status:   types.Ok,
		Value:    result,
		Category: "Personal",
	}, nil
}

// TxPool method handlers
func RpcTxPoolStatus(rCtx *RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, "txpool_status")
	if err != nil {
		return &types.RpcResult{
			Method:   TxPoolStatus,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "TxPool",
		}, nil
	}
	return &types.RpcResult{
		Method:   TxPoolStatus,
		Status:   types.Ok,
		Value:    result,
		Category: "TxPool",
	}, nil
}

// Mining method handlers
func RpcEthMining(rCtx *RpcContext) (*types.RpcResult, error) {
	var result bool
	err := rCtx.EthCli.Client().Call(&result, "eth_mining")
	if err != nil {
		return &types.RpcResult{
			Method:   EthMining,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "Mining",
		}, nil
	}
	return &types.RpcResult{
		Method:   EthMining,
		Status:   types.Ok,
		Value:    result,
		Category: "Mining",
	}, nil
}

// Not implemented method handlers
func RpcNotImplemented(methodName types.RpcName, category string) (*types.RpcResult, error) {
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.NotImplemented,
		ErrMsg:   "Expected to be not implemented",
		Category: category,
	}, nil
}

func RpcSkipped(methodName types.RpcName, category string, reason string) (*types.RpcResult, error) {
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Skipped,
		ErrMsg:   reason,
		Category: category,
	}, nil
}
