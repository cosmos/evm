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
	// Web3 namespace
	Web3ClientVersion types.RpcName = "web3_clientVersion"
	Web3Sha3          types.RpcName = "web3_sha3"

	// Net namespace  
	NetVersion   types.RpcName = "net_version"
	NetPeerCount types.RpcName = "net_peerCount"
	NetListening types.RpcName = "net_listening"

	// Eth namespace - client subcategory
	EthChainId     types.RpcName = "eth_chainId"
	EthSyncing     types.RpcName = "eth_syncing" 
	EthCoinbase    types.RpcName = "eth_coinbase"
	EthAccounts    types.RpcName = "eth_accounts"
	EthBlockNumber types.RpcName = "eth_blockNumber"
	EthMining      types.RpcName = "eth_mining"
	EthHashrate    types.RpcName = "eth_hashrate"

	// Eth namespace - fee_market subcategory
	EthGasPrice             types.RpcName = "eth_gasPrice"
	EthBlobBaseFee          types.RpcName = "eth_blobBaseFee"
	EthMaxPriorityFeePerGas types.RpcName = "eth_maxPriorityFeePerGas"
	EthFeeHistory           types.RpcName = "eth_feeHistory"

	// Eth namespace - state subcategory
	EthGetBalance          types.RpcName = "eth_getBalance"
	EthGetStorageAt        types.RpcName = "eth_getStorageAt"
	EthGetTransactionCount types.RpcName = "eth_getTransactionCount"
	EthGetCode             types.RpcName = "eth_getCode"
	EthGetProof            types.RpcName = "eth_getProof"

	// Eth namespace - block subcategory
	EthGetBlockByHash                types.RpcName = "eth_getBlockByHash"
	EthGetBlockByNumber              types.RpcName = "eth_getBlockByNumber"
	EthGetBlockTransactionCountByHash   types.RpcName = "eth_getBlockTransactionCountByHash"
	EthGetBlockTransactionCountByNumber types.RpcName = "eth_getBlockTransactionCountByNumber"
	EthGetUncleCountByBlockHash         types.RpcName = "eth_getUncleCountByBlockHash"
	EthGetUncleCountByBlockNumber       types.RpcName = "eth_getUncleCountByBlockNumber"
	EthGetUncleByBlockHashAndIndex      types.RpcName = "eth_getUncleByBlockHashAndIndex"
	EthGetUncleByBlockNumberAndIndex    types.RpcName = "eth_getUncleByBlockNumberAndIndex"
	EthGetBlockReceipts                 types.RpcName = "eth_getBlockReceipts"

	// Eth namespace - transaction subcategory
	EthGetTransactionByHash                types.RpcName = "eth_getTransactionByHash"
	EthGetTransactionByBlockHashAndIndex   types.RpcName = "eth_getTransactionByBlockHashAndIndex"
	EthGetTransactionByBlockNumberAndIndex types.RpcName = "eth_getTransactionByBlockNumberAndIndex"
	EthGetTransactionReceipt               types.RpcName = "eth_getTransactionReceipt"
	EthGetTransactionCountByHash           types.RpcName = "eth_getTransactionCountByHash"
	EthPendingTransactions                 types.RpcName = "eth_pendingTransactions"

	// Eth namespace - filter subcategory
	EthNewFilter                   types.RpcName = "eth_newFilter"
	EthNewBlockFilter              types.RpcName = "eth_newBlockFilter"
	EthNewPendingTransactionFilter types.RpcName = "eth_newPendingTransactionFilter"
	EthGetFilterChanges            types.RpcName = "eth_getFilterChanges"
	EthGetFilterLogs               types.RpcName = "eth_getFilterLogs"
	EthUninstallFilter             types.RpcName = "eth_uninstallFilter"
	EthGetLogs                     types.RpcName = "eth_getLogs"

	// Eth namespace - execute subcategory
	Call        types.RpcName = "eth_call"
	EstimateGas types.RpcName = "eth_estimateGas"

	// Eth namespace - submit subcategory
	EthSendTransaction    types.RpcName = "eth_sendTransaction"
	EthSendRawTransaction types.RpcName = "eth_sendRawTransaction"

	// Eth namespace - sign subcategory (deprecated in many clients)
	EthSign            types.RpcName = "eth_sign"
	EthSignTransaction types.RpcName = "eth_signTransaction"

	// Eth namespace - other/deprecated methods
	EthProtocolVersion  types.RpcName = "eth_protocolVersion"
	EthGetCompilers     types.RpcName = "eth_getCompilers"
	EthCompileSolidity  types.RpcName = "eth_compileSolidity"
	EthGetWork          types.RpcName = "eth_getWork"
	EthSubmitWork       types.RpcName = "eth_submitWork"
	EthSubmitHashrate   types.RpcName = "eth_submitHashrate"
	EthCreateAccessList types.RpcName = "eth_createAccessList"

	// Personal namespace (deprecated)
	PersonalListAccounts  types.RpcName = "personal_listAccounts"
	PersonalEcRecover     types.RpcName = "personal_ecRecover"
	PersonalListWallets   types.RpcName = "personal_listWallets"
	PersonalNewAccount    types.RpcName = "personal_newAccount"
	PersonalImportRawKey  types.RpcName = "personal_importRawKey"
	PersonalUnlockAccount types.RpcName = "personal_unlockAccount"
	PersonalLockAccount   types.RpcName = "personal_lockAccount"
	PersonalSign          types.RpcName = "personal_sign"
	PersonalSignTypedData types.RpcName = "personal_signTypedData"

	// Miner namespace (deprecated)
	MinerStart        types.RpcName = "miner_start"
	MinerStop         types.RpcName = "miner_stop"
	MinerSetEtherbase types.RpcName = "miner_setEtherbase"
	MinerSetExtra     types.RpcName = "miner_setExtra"
	MinerSetGasPrice  types.RpcName = "miner_setGasPrice"
	MinerSetGasLimit  types.RpcName = "miner_setGasLimit"
	MinerGetHashrate  types.RpcName = "miner_getHashrate"

	// TxPool namespace
	TxPoolContent types.RpcName = "txpool_content"
	TxPoolInspect types.RpcName = "txpool_inspect"
	TxPoolStatus  types.RpcName = "txpool_status"

	// Debug namespace - tracing subcategory
	DebugTraceTransaction types.RpcName = "debug_traceTransaction"
	DebugTraceBlock       types.RpcName = "debug_traceBlock"
	DebugTraceCall        types.RpcName = "debug_traceCall"
	DebugIntermediateRoots types.RpcName = "debug_intermediateRoots"

	// Debug namespace - database subcategory
	DebugDbGet             types.RpcName = "debug_dbGet"
	DebugDbAncient         types.RpcName = "debug_dbAncient"
	DebugChaindbCompact    types.RpcName = "debug_chaindbCompact"
	DebugGetModifiedAccounts types.RpcName = "debug_getModifiedAccounts"
	DebugDumpBlock         types.RpcName = "debug_dumpBlock"

	// Debug namespace - profiling subcategory
	DebugBlockProfile         types.RpcName = "debug_blockProfile"
	DebugCpuProfile           types.RpcName = "debug_cpuProfile"
	DebugGoTrace              types.RpcName = "debug_goTrace"
	DebugMemStats             types.RpcName = "debug_memStats"
	DebugMutexProfile         types.RpcName = "debug_mutexProfile"
	DebugSetBlockProfileRate  types.RpcName = "debug_setBlockProfileRate"
	DebugSetMutexProfileFraction types.RpcName = "debug_setMutexProfileFraction"

	// Debug namespace - diagnostics subcategory
	DebugBacktraceAt    types.RpcName = "debug_backtraceAt"
	DebugStacks         types.RpcName = "debug_stacks"
	DebugGetBadBlocks   types.RpcName = "debug_getBadBlocks"
	DebugPreimage       types.RpcName = "debug_preimage"
	DebugFreeOSMemory   types.RpcName = "debug_freeOSMemory"
	DebugSetHead        types.RpcName = "debug_setHead"

	// Engine API namespace (not applicable for Cosmos chains)
	EngineNewPayloadV1        types.RpcName = "engine_newPayloadV1"
	EngineNewPayloadV2        types.RpcName = "engine_newPayloadV2"
	EngineNewPayloadV3        types.RpcName = "engine_newPayloadV3"
	EngineForkchoiceUpdatedV1 types.RpcName = "engine_forkchoiceUpdatedV1"
	EngineForkchoiceUpdatedV2 types.RpcName = "engine_forkchoiceUpdatedV2"
	EngineForkchoiceUpdatedV3 types.RpcName = "engine_forkchoiceUpdatedV3"
	EngineGetPayloadV1        types.RpcName = "engine_getPayloadV1"
	EngineGetPayloadV2        types.RpcName = "engine_getPayloadV2"
	EngineGetPayloadV3        types.RpcName = "engine_getPayloadV3"

	// Trace namespace (OpenEthereum/Erigon specific, not in standard execution-apis)
	TraceCall        types.RpcName = "trace_call"
	TraceCallMany    types.RpcName = "trace_callMany"
	TraceTransaction types.RpcName = "trace_transaction"
	TraceBlock       types.RpcName = "trace_block"

	// Admin namespace (Geth specific administrative methods)
	AdminAddPeer    types.RpcName = "admin_addPeer"
	AdminNodeInfo   types.RpcName = "admin_nodeInfo"
	AdminPeers      types.RpcName = "admin_peers"
	AdminDatadir    types.RpcName = "admin_datadir"
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

func RpcEthBlockNumber(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthBlockNumber); result != nil {
		return result, nil
	}
	blockNumber, err := rCtx.EthCli.BlockNumber(context.Background())
	if err != nil {
		return nil, err
	}

	// Block number 0 is valid for fresh chains
	status := types.Ok

	result := &types.RpcResult{
		Method:   EthBlockNumber,
		Status:   status,
		Value:    blockNumber,
		Category: "Core Eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGasPrice(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGasPrice); result != nil {
		return result, nil
	}

	gasPrice, err := rCtx.EthCli.SuggestGasPrice(context.Background())
	if err != nil {
		return nil, err
	}

	// gasPrice should never be nil or zero in a proper EVM implementation
	if gasPrice == nil {
		return nil, fmt.Errorf("gasPrice is nil")
	}
	if gasPrice.Cmp(big.NewInt(0)) == 0 {
		return nil, fmt.Errorf("gasPrice is zero")
	}

	status := types.Ok

	result := &types.RpcResult{
		Method:   EthGasPrice,
		Status:   status,
		Value:    gasPrice.String(),
		Category: "Core Eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthMaxPriorityFeePerGas(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthMaxPriorityFeePerGas); result != nil {
		return result, nil
	}

	maxPriorityFeePerGas, err := rCtx.EthCli.SuggestGasTipCap(context.Background())
	if err != nil {
		return nil, err
	}

	// Zero maxPriorityFeePerGas is valid (legacy transactions don't use tips)
	status := types.Ok

	result := &types.RpcResult{
		Method:   EthMaxPriorityFeePerGas,
		Status:   status,
		Value:    maxPriorityFeePerGas.String(),
		Category: "EIP-1559",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthChainId(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthChainId); result != nil {
		return result, nil
	}

	chainId, err := rCtx.EthCli.ChainID(context.Background())
	if err != nil {
		return nil, err
	}

	// chainId should never be zero
	if chainId.Cmp(big.NewInt(0)) == 0 {
		return nil, fmt.Errorf("chainId is zero")
	}

	status := types.Ok

	result := &types.RpcResult{
		Method:   EthChainId,
		Status:   status,
		Value:    chainId.String(),
		Category: "Core Eth",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetBalance(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetBalance); result != nil {
		return result, nil
	}

	balance, err := rCtx.EthCli.BalanceAt(context.Background(), rCtx.Acc.Address, nil)
	if err != nil {
		return nil, err
	}

	// Zero balance is valid for unused addresses
	status := types.Ok

	result := &types.RpcResult{
		Method:   EthGetBalance,
		Status:   status,
		Value:    balance.String(),
		Category: "Account & State",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetTransactionCount(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetTransactionCount); result != nil {
		return result, nil
	}

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}

	// Zero nonce is valid for unused addresses
	status := types.Ok

	return &types.RpcResult{
		Method:   EthGetTransactionCount,
		Status:   status,
		Value:    nonce,
		Category: "Account & State",
	}, nil
}

func RpcEthGetBlockByHash(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetBlockByHash); result != nil {
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
		Method: EthGetBlockByHash,
		Status: types.Ok,
		Value:  utils.MustBeautifyBlock(types.NewRpcBlock(block)),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetBlockByNumber(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetBlockByNumber); result != nil {
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
		Method: EthGetBlockByNumber,
		Status: types.Ok,
		Value:  utils.MustBeautifyBlock(types.NewRpcBlock(blk)),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthSendRawTransactionTransferValue(rCtx *RpcContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainId, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthChainId,
		Status: types.Ok,
		Value:  rCtx.ChainId.String(),
	})

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthGetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.EthCli.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.EthCli.SuggestGasPrice(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthGasPrice,
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
		Method: EthGetBalance,
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
		Method: EthSendRawTransaction,
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

func RpcEthSendRawTransactionDeployContract(rCtx *RpcContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainId, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthChainId,
		Status: types.Ok,
		Value:  rCtx.ChainId.String(),
	})

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthGetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.EthCli.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.EthCli.SuggestGasPrice(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthGasPrice,
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
		Method: EthSendRawTransaction,
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

func RpcEthSendRawTransactionTransferERC20(rCtx *RpcContext) (*types.RpcResult, error) {
	// testedRPCs is a slice of RpcResult that will be appended to rCtx.AlreadyTestedRPCs
	// if the transaction is successfully sent
	var testedRPCs []*types.RpcResult
	var err error
	// Create a new transaction
	if rCtx.ChainId, err = rCtx.EthCli.ChainID(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthChainId,
		Status: types.Ok,
		Value:  rCtx.ChainId.String(),
	})

	nonce, err := rCtx.EthCli.PendingNonceAt(context.Background(), rCtx.Acc.Address)
	if err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthGetTransactionCount,
		Status: types.Ok,
		Value:  nonce,
	})

	if rCtx.MaxPriorityFeePerGas, err = rCtx.EthCli.SuggestGasTipCap(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthMaxPriorityFeePerGas,
		Status: types.Ok,
		Value:  rCtx.MaxPriorityFeePerGas.String(),
	})
	if rCtx.GasPrice, err = rCtx.EthCli.SuggestGasPrice(context.Background()); err != nil {
		return nil, err
	}
	testedRPCs = append(testedRPCs, &types.RpcResult{
		Method: EthGasPrice,
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
		Method: EthSendRawTransaction,
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

func RpcEthGetBlockReceipts(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetBlockReceipts); result != nil {
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
		Method: EthGetBlockReceipts,
		Status: types.Ok,
		Value:  utils.MustBeautifyReceipts(receipts),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetTransactionByHash(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetTransactionByHash); result != nil {
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
		Method: EthGetTransactionByHash,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetTransactionByBlockHashAndIndex(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetTransactionByBlockHashAndIndex); result != nil {
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
		Method: EthGetTransactionByBlockHashAndIndex,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetTransactionByBlockNumberAndIndex(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetTransactionByBlockNumberAndIndex); result != nil {
		return result, nil
	}

	if len(rCtx.BlockNumsIncludingTx) == 0 {
		return nil, errors.New("no blocks with transactions")
	}

	// TODO: Random pick
	blkNum := rCtx.BlockNumsIncludingTx[0]
	var tx gethtypes.Transaction
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &tx, string(EthGetTransactionByBlockNumberAndIndex), blkNum, "0x0"); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: EthGetTransactionByBlockNumberAndIndex,
		Status: types.Ok,
		Value:  utils.MustBeautifyTransaction(&tx),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetTransactionCountByHash(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetTransactionCountByHash); result != nil {
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
	if err = rCtx.EthCli.Client().CallContext(context.Background(), &count, string(EthGetTransactionCountByHash), blk.Hash()); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: EthGetTransactionCountByHash,
		Status: types.Ok,
		Value:  count,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetTransactionReceipt(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetTransactionReceipt); result != nil {
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
		Method: EthGetTransactionReceipt,
		Status: types.Ok,
		Value:  utils.MustBeautifyReceipt(receipt),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetBlockTransactionCountByHash(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetBlockTransactionCountByHash); result != nil {
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
		Method: EthGetBlockTransactionCountByHash,
		Status: types.Ok,
		Value:  count,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetCode(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetCode); result != nil {
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
		Method: EthGetCode,
		Status: types.Ok,
		Value:  hexutils.BytesToHex(code),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetStorageAt(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetStorageAt); result != nil {
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

	// Zero storage is valid - most storage slots are empty
	status := types.Ok

	result := &types.RpcResult{
		Method:   EthGetStorageAt,
		Status:   status,
		Value:    hexutils.BytesToHex(storage),
		Category: "Account & State",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthNewFilter(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthNewFilter); result != nil {
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
	if err = rCtx.EthCli.Client().CallContext(context.Background(), &rpcId, string(EthNewFilter), args); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: EthNewFilter,
		Status: types.Ok,
		Value:  rpcId,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	rCtx.FilterId = rpcId
	rCtx.FilterQuery = fErc20Transfer

	return result, nil
}

func RpcEthGetFilterLogs(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetFilterLogs); result != nil {
		return result, nil
	}

	if rCtx.FilterId == "" {
		return nil, errors.New("no filter id, must create a filter first")
	}

	if _, err := RpcEthSendRawTransactionTransferERC20(rCtx); err != nil {
		return nil, errors.New("transfer ERC20 must be succeeded before checking filter logs")
	}

	var logs []gethtypes.Log
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &logs, string(EthGetFilterLogs), rCtx.FilterId); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: EthGetFilterLogs,
		Status: types.Ok,
		Value:  utils.MustBeautifyLogs(logs),
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthNewBlockFilter(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthNewBlockFilter); result != nil {
		return result, nil
	}

	var rpcId string
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &rpcId, string(EthNewBlockFilter)); err != nil {
		return nil, err
	}

	result := &types.RpcResult{
		Method: EthNewBlockFilter,
		Status: types.Ok,
		Value:  rpcId,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)
	rCtx.BlockFilterId = rpcId

	return result, nil
}

func RpcEthGetFilterChanges(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetFilterChanges); result != nil {
		return result, nil
	}

	if rCtx.BlockFilterId == "" {
		return nil, errors.New("no block filter id, must create a block filter first")
	}

	// TODO: Make it configurable
	time.Sleep(3 * time.Second) // wait for a new block to be mined

	var changes []interface{}
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &changes, string(EthGetFilterChanges), rCtx.BlockFilterId); err != nil {
		return nil, err
	}

	status := types.Ok
	// Empty results are valid - no warnings needed

	result := &types.RpcResult{
		Method:   EthGetFilterChanges,
		Status:   status,
		Value:    changes,
		Category: "Filter",
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthUninstallFilter(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthUninstallFilter); result != nil {
		return result, nil
	}

	if rCtx.FilterId == "" {
		return nil, errors.New("no filter id, must create a filter first")
	}

	var res bool
	if err := rCtx.EthCli.Client().CallContext(context.Background(), &res, string(EthUninstallFilter), rCtx.FilterId); err != nil {
		return nil, err
	}
	if !res {
		return nil, errors.New("uninstall filter failed")
	}

	if err := rCtx.EthCli.Client().CallContext(context.Background(), &res, string(EthUninstallFilter), rCtx.FilterId); err != nil {
		return nil, err
	}
	if res {
		return nil, errors.New("uninstall filter should be failed because it was already uninstalled")
	}

	result := &types.RpcResult{
		Method: EthUninstallFilter,
		Status: types.Ok,
		Value:  rCtx.FilterId,
	}
	rCtx.AlreadyTestedRPCs = append(rCtx.AlreadyTestedRPCs, result)

	return result, nil
}

func RpcEthGetLogs(rCtx *RpcContext) (*types.RpcResult, error) {
	if result := rCtx.AlreadyTested(EthGetLogs); result != nil {
		return result, nil
	}

	if _, err := RpcEthNewFilter(rCtx); err != nil {
		return nil, errors.New("failed to create a filter")
	}

	if _, err := RpcEthSendRawTransactionTransferERC20(rCtx); err != nil {
		return nil, errors.New("transfer ERC20 must be succeeded before checking filter logs")
	}

	// set from block because of limit
	logs, err := rCtx.EthCli.FilterLogs(context.Background(), rCtx.FilterQuery)
	if err != nil {
		return nil, err
	}

	status := types.Ok
	// Empty results are valid - no warnings needed

	result := &types.RpcResult{
		Method:   EthGetLogs,
		Status:   status,
		Value:    utils.MustBeautifyLogs(logs),
		Category: "Filter",
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
					Method: EthGetTransactionReceipt,
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
			Category: "personal",
		}, nil
	}
	return &types.RpcResult{
		Method:   PersonalListAccounts,
		Status:   types.Ok,
		Value:    result,
		Category: "personal",
	}, nil
}

func RpcPersonalEcRecover(rCtx *RpcContext) (*types.RpcResult, error) {
	// Test with known data
	var result string
	err := rCtx.EthCli.Client().Call(&result, "personal_ecRecover",
		"0xdeadbeaf",
		"0xf9ff74c86aefeb5f6019d77280bbb44fb695b4d45cfe97e6eed7acd62905f4a85034d5c68ed25a2e7a8eeb9baf1b8401e4f865d92ec48c1763bf649e354d900b1c")
	if err != nil {
		return &types.RpcResult{
			Method:   PersonalEcRecover,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "personal",
		}, nil
	}
	return &types.RpcResult{
		Method:   PersonalEcRecover,
		Status:   types.Ok,
		Value:    result,
		Category: "personal",
	}, nil
}

func RpcPersonalListWallets(rCtx *RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, "personal_listWallets")
	if err != nil {
		return &types.RpcResult{
			Method:   PersonalListWallets,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "personal",
		}, nil
	}
	return &types.RpcResult{
		Method:   PersonalListWallets,
		Status:   types.Ok,
		Value:    result,
		Category: "personal",
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

func RpcTxPoolContent(rCtx *RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, "txpool_content")
	if err != nil {
		return &types.RpcResult{
			Method:   TxPoolContent,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "TxPool",
		}, nil
	}
	return &types.RpcResult{
		Method:   TxPoolContent,
		Status:   types.Ok,
		Value:    result,
		Category: "TxPool",
	}, nil
}

func RpcTxPoolInspect(rCtx *RpcContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.EthCli.Client().Call(&result, "txpool_inspect")
	if err != nil {
		return &types.RpcResult{
			Method:   TxPoolInspect,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "TxPool",
		}, nil
	}
	return &types.RpcResult{
		Method:   TxPoolInspect,
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

func RpcDeprecated(methodName types.RpcName, category string, reason string) (*types.RpcResult, error) {
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Deprecated,
		ErrMsg:   reason,
		Category: category,
	}, nil
}

// Missing function implementations
func RpcEthCoinbase(rCtx *RpcContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.EthCli.Client().Call(&result, "eth_coinbase")
	if err != nil {
		return &types.RpcResult{
			Method:   EthCoinbase,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}

	return &types.RpcResult{
		Method:   EthCoinbase,
		Status:   types.Ok,
		Value:    result,
		Category: "eth",
	}, nil
}

func RpcEthCall(rCtx *RpcContext) (*types.RpcResult, error) {
	// Simple eth_call test 
	callMsg := ethereum.CallMsg{
		To:   &rCtx.Acc.Address,
		Data: []byte{},
	}
	
	result, err := rCtx.EthCli.CallContract(context.Background(), callMsg, nil)
	if err != nil {
		return &types.RpcResult{
			Method:   Call,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}

	return &types.RpcResult{
		Method:   Call,
		Status:   types.Ok,
		Value:    "0x" + hex.EncodeToString(result),
		Category: "eth",
	}, nil
}

func RpcEthEstimateGas(rCtx *RpcContext) (*types.RpcResult, error) {
	// Simple gas estimation test
	callMsg := ethereum.CallMsg{
		From:  rCtx.Acc.Address,
		To:    &rCtx.Acc.Address,
		Value: big.NewInt(0),
	}
	
	gasLimit, err := rCtx.EthCli.EstimateGas(context.Background(), callMsg)
	if err != nil {
		return &types.RpcResult{
			Method:   EstimateGas,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: "eth",
		}, nil
	}

	return &types.RpcResult{
		Method:   EstimateGas,
		Status:   types.Ok,
		Value:    fmt.Sprintf("0x%x", gasLimit),
		Category: "eth",
	}, nil
}
