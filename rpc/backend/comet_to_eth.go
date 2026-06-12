package backend

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/pkg/errors"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"

	rpctypes "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RPCBlockFromCometBlock returns a JSON-RPC compatible Ethereum block from a
// given CometBFT block and its block result.
func (b *Backend) RPCHeaderFromCometBlock(
	resBlock *cmtrpctypes.ResultBlock,
	blockRes *cmtrpctypes.ResultBlockResults,
) (map[string]interface{}, error) {
	ethBlock, err := b.EthBlockFromCometBlock(resBlock, blockRes)
	if err != nil {
		return nil, fmt.Errorf("failed to get rpc block from comet block: %w", err)
	}

	return rpctypes.RPCMarshalHeader(ethBlock.Header(), resBlock.BlockID.Hash), nil
}

// RPCBlockFromCometBlock returns a JSON-RPC compatible Ethereum block from a
// given CometBFT block and its block result.
func (b *Backend) RPCBlockFromCometBlock(
	resBlock *cmtrpctypes.ResultBlock,
	blockRes *cmtrpctypes.ResultBlockResults,
	fullTx bool,
) (map[string]interface{}, error) {
	msgs, _ := b.EthMsgsFromCometBlock(resBlock, blockRes)
	ethBlock, err := b.EthBlockFromCometBlock(resBlock, blockRes)
	if err != nil {
		return nil, fmt.Errorf("failed to get rpc block from comet block: %w", err)
	}

	return rpctypes.RPCMarshalBlock(ethBlock, resBlock, msgs, true, fullTx, b.ChainConfig())
}

// BlockNumberFromComet returns the BlockNumber from BlockNumberOrHash
func (b *Backend) BlockNumberFromComet(blockNrOrHash rpctypes.BlockNumberOrHash) (rpctypes.BlockNumber, error) {
	switch {
	case blockNrOrHash.BlockHash == nil && blockNrOrHash.BlockNumber == nil:
		return rpctypes.EthEarliestBlockNumber, fmt.Errorf("types BlockHash and BlockNumber cannot be both nil")
	case blockNrOrHash.BlockHash != nil:
		blockNumber, err := b.BlockNumberFromCometByHash(*blockNrOrHash.BlockHash)
		if err != nil {
			return rpctypes.EthEarliestBlockNumber, err
		}
		return rpctypes.NewBlockNumber(blockNumber), nil
	case blockNrOrHash.BlockNumber != nil:
		return *blockNrOrHash.BlockNumber, nil
	default:
		return rpctypes.EthEarliestBlockNumber, nil
	}
}

// BlockNumberFromCometByHash returns the block height of given block hash
func (b *Backend) BlockNumberFromCometByHash(blockHash common.Hash) (*big.Int, error) {
	resHeader, err := b.RPCClient.HeaderByHash(b.Ctx, blockHash.Bytes())
	if err != nil {
		return nil, err
	}

	if resHeader == nil || resHeader.Header == nil {
		return nil, errors.Errorf("header not found for hash %s", blockHash.Hex())
	}

	return big.NewInt(resHeader.Header.Height), nil
}

// EthMsgsFromCometBlock returns all real MsgEthereumTxs from a
// CometBFT block. It also ensures consistency over the correct txs indexes
// across RPC endpoints. The second return value holds additional fields for
// derived (non-MsgEthereumTx) transactions; nil entries correspond to native
// EVM transactions.
func (b *Backend) EthMsgsFromCometBlock(
	resBlock *cmtrpctypes.ResultBlock,
	blockRes *cmtrpctypes.ResultBlockResults,
) ([]*evmtypes.MsgEthereumTx, []*rpctypes.TxResultAdditionalFields) {
	var result []*evmtypes.MsgEthereumTx
	var txsAdditional []*rpctypes.TxResultAdditionalFields
	block := resBlock.Block

	txResults := blockRes.TxsResults

	for i, tx := range block.Txs {
		// Check if tx exists on EVM by cross checking with blockResults:
		//  - Include unsuccessful tx that exceeds block gas limit
		//  - Include unsuccessful tx that failed when committing changes to stateDB
		//  - Exclude unsuccessful tx with any other error but ExceedBlockGasLimit
		if !rpctypes.TxSucessOrExpectedFailure(txResults[i]) {
			b.Logger.Debug("invalid tx result code", "cosmos-hash", hexutil.Encode(tx.Hash()))
			continue
		}

		decodedTx, err := b.ClientCtx.TxConfig.TxDecoder()(tx)
		// assumption is that if regular MsgEthereumTx is found, there won't be a derived one too
		shouldCheckForDerivedCosmosEVMTx := true
		if err == nil {
			for _, msg := range decodedTx.GetMsgs() {
				ethMsg, ok := msg.(*evmtypes.MsgEthereumTx)
				if ok {
					shouldCheckForDerivedCosmosEVMTx = false
					result = append(result, ethMsg)
					txsAdditional = append(txsAdditional, nil)
				}
			}
		} else {
			b.Logger.Debug("failed to decode transaction in block", "height", block.Height, "error", err.Error())
		}

		if shouldCheckForDerivedCosmosEVMTx {
			ethMsgs, additionals := b.parseDerivedTxFromBlockResults(txResults, i, decodedTx, block)
			for idx, ethMsg := range ethMsgs {
				if ethMsg != nil {
					result = append(result, ethMsg)
					txsAdditional = append(txsAdditional, additionals[idx])
				}
			}
		}
	}

	return result, txsAdditional
}

func (b *Backend) parseDerivedTxFromBlockResults(
	txResults []*abci.ExecTxResult,
	i int,
	tx sdk.Tx,
	block *tmtypes.Block,
) ([]*evmtypes.MsgEthereumTx, []*rpctypes.TxResultAdditionalFields) {
	results, additionals, err := rpctypes.ParseTxBlockResult(txResults[i], tx, i, block.Height)
	if err != nil {
		b.Logger.Error(err.Error())
		return nil, nil
	}
	if len(results) == 0 {
		b.Logger.Debug("derived ethereum tx not found in msgs: block %d, index %d", block.Height, i)
		return nil, nil
	}

	ethMsgs := make([]*evmtypes.MsgEthereumTx, 0, len(additionals))
	derivedAdditionals := make([]*rpctypes.TxResultAdditionalFields, 0, len(additionals))
	for idx, additional := range additionals {
		if additional == nil || results[idx] == nil {
			continue
		}
		ethMsgs = append(ethMsgs, b.parseDerivedTxFromAdditionalFields(additional))
		derivedAdditionals = append(derivedAdditionals, additional)
	}
	return ethMsgs, derivedAdditionals
}

func (b *Backend) parseDerivedTxFromAdditionalFields(
	additional *rpctypes.TxResultAdditionalFields,
) *evmtypes.MsgEthereumTx {
	recipient := additional.Recipient
	gas := gasForDerivedEthTx(additional)

	t := ethtypes.NewTx(&ethtypes.LegacyTx{
		Nonce:    additional.Nonce,
		Data:     additional.Data,
		Gas:      gas,
		To:       &recipient,
		GasPrice: nil,
		Value:    additional.Value,
		V:        big.NewInt(0),
		R:        big.NewInt(0),
		S:        big.NewInt(0),
	})
	ethMsg := &evmtypes.MsgEthereumTx{}
	ethMsg.FromEthereumTx(t)
	ethMsg.From = additional.Sender.Bytes()
	return ethMsg
}

// gasForDerivedEthTx returns the gas value to use for a derived Ethereum transaction.
//
// GasLimit is preferred when available, as it reflects the originally declared
// transaction gas. For older transactions where GasLimit was not emitted and is
// zero, GasUsed is used as a fallback for backward compatibility.
func gasForDerivedEthTx(additional *rpctypes.TxResultAdditionalFields) uint64 {
	const gasFallbackMultiplier = 2

	if additional.GasLimit != nil && *additional.GasLimit > 0 {
		return *additional.GasLimit
	}

	if additional.GasUsed > 0 {
		return additional.GasUsed * gasFallbackMultiplier
	}

	return 0
}

// RPCBlockFromCometBlock returns a JSON-RPC compatible Ethereum block from a
// given CometBFT block and its block result.
func (b *Backend) EthBlockFromCometBlock(
	resBlock *cmtrpctypes.ResultBlock,
	blockRes *cmtrpctypes.ResultBlockResults,
) (*ethtypes.Block, error) {
	cmtBlock := resBlock.Block

	// 1. get base fee
	baseFee, err := b.BaseFee(blockRes)
	if err != nil {
		// handle the error for pruned node.
		b.Logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", cmtBlock.Height, "error", err)
	}

	// 2. get miner
	miner, err := b.MinerFromCometBlock(resBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to get miner(block proposer) address from comet block")
	}

	// 3. get block gasLimit
	ctx := rpctypes.ContextWithHeight(cmtBlock.Height)
	gasLimit, err := rpctypes.BlockMaxGasFromConsensusParams(ctx, b.ClientCtx, cmtBlock.Height)
	if err != nil {
		b.Logger.Error("failed to query consensus params", "error", err.Error())
	}

	// 4. create blockHeader without transactions, receipts, withdrawals, ...
	ethHeader := rpctypes.MakeHeader(cmtBlock.Header, gasLimit, miner, baseFee)

	// 5. get MsgEthereumTxs (exclude derived txs from the ETH block body)
	msgs, additionals := b.EthMsgsFromCometBlock(resBlock, blockRes)
	var txs []*ethtypes.Transaction
	for i, ethMsg := range msgs {
		if additionals[i] == nil {
			txs = append(txs, ethMsg.AsTransaction())
		}
	}

	// 6. create ethBlock body with transactions
	body := &ethtypes.Body{
		Transactions: txs,
		Uncles:       []*ethtypes.Header{},
		Withdrawals:  []*ethtypes.Withdrawal{},
	}

	// 7. receipts
	receipts, err := b.ReceiptsFromCometBlock(resBlock, blockRes, msgs, additionals)
	if err != nil {
		return nil, fmt.Errorf("failed to get receipts from comet block: %w", err)
	}

	// 8. Gas Used
	gasUsed := uint64(0)
	for _, txsResult := range blockRes.TxsResults {
		// workaround for cosmos-sdk bug. https://github.com/cosmos/cosmos-sdk/issues/10832
		if ShouldIgnoreGasUsed(txsResult) {
			// block gas limit has exceeded, other txs must have failed with same reason.
			break
		}
		gasUsed += uint64(txsResult.GetGasUsed()) // #nosec G115 -- checked for int overflow already
	}
	ethHeader.GasUsed = gasUsed

	// 9. create eth block
	ethBlock := ethtypes.NewBlock(ethHeader, body, receipts, trie.NewStackTrie(nil))
	return ethBlock, nil
}

func (b *Backend) MinerFromCometBlock(
	resBlock *cmtrpctypes.ResultBlock,
) (common.Address, error) {
	cmtBlock := resBlock.Block

	req := &evmtypes.QueryValidatorAccountRequest{
		ConsAddress: sdk.ConsAddress(cmtBlock.Header.ProposerAddress).String(),
	}

	var validatorAccAddr sdk.AccAddress

	ctx := rpctypes.ContextWithHeight(cmtBlock.Height)
	res, err := b.QueryClient.ValidatorAccount(ctx, req)
	if err != nil {
		b.Logger.Debug(
			"failed to query validator operator address",
			"height", cmtBlock.Height,
			"cons-address", req.ConsAddress,
			"error", err.Error(),
		)
		// use zero address as the validator operator address
		validatorAccAddr = sdk.AccAddress(common.Address{}.Bytes())
	} else {
		validatorAccAddr, err = sdk.AccAddressFromBech32(res.AccountAddress)
		if err != nil {
			return common.Address{}, err
		}
	}

	return common.BytesToAddress(validatorAccAddr), nil
}

// derivedTxLogsFromEvents finds EVM logs for a derived tx by scanning tx_log events and
// matching each log's TxHash to the given hash. Returns nil, nil when no matching logs are
// found — valid for a successful derived tx that emits no EVM events.
func derivedTxLogsFromEvents(events []abci.Event, txHash common.Hash, blockNumber uint64) ([]*ethtypes.Log, error) {
	var result []*ethtypes.Log
	for _, event := range events {
		if event.Type != evmtypes.EventTypeTxLog {
			continue
		}
		for _, attr := range event.Attributes {
			if attr.Key != evmtypes.AttributeKeyTxLog {
				continue
			}
			var log evmtypes.Log
			if err := json.Unmarshal([]byte(attr.Value), &log); err != nil {
				return nil, err
			}
			if common.HexToHash(log.TxHash) != txHash {
				continue
			}
			l := log.ToEthereum()
			l.BlockNumber = blockNumber
			result = append(result, l)
		}
	}
	return result, nil
}

func (b *Backend) ReceiptsFromCometBlock(
	resBlock *cmtrpctypes.ResultBlock,
	blockRes *cmtrpctypes.ResultBlockResults,
	msgs []*evmtypes.MsgEthereumTx,
	additionals []*rpctypes.TxResultAdditionalFields,
) ([]*ethtypes.Receipt, error) {
	baseFee, err := b.BaseFee(blockRes)
	if err != nil {
		// handle the error for pruned node.
		b.Logger.Error("failed to fetch Base Fee from prunned block. Check node prunning configuration", "height", resBlock.Block.Height, "error", err)
	}

	blockHeight := uint64(resBlock.Block.Height) // #nosec G115
	blockHash := common.BytesToHash(resBlock.BlockID.Hash)
	receipts := make([]*ethtypes.Receipt, len(msgs))
	cumulatedGasUsed := uint64(0)
	for i, ethMsg := range msgs {
		var additional *rpctypes.TxResultAdditionalFields
		if additionals != nil && i < len(additionals) {
			additional = additionals[i]
		}

		// Derived txs must be looked up by the event hash (additional.Hash); native txs use ethMsg.Hash().
		var lookupHash common.Hash
		if additional != nil {
			lookupHash = additional.Hash
		} else {
			lookupHash = ethMsg.Hash()
		}

		txResult, _, err := b.GetTxByEthHash(lookupHash)
		if err != nil {
			return nil, fmt.Errorf("tx not found: hash=%s, error=%s", lookupHash, err.Error())
		}

		cumulatedGasUsed += txResult.GasUsed

		var effectiveGasPrice *big.Int
		if baseFee != nil {
			effectiveGasPrice = rpctypes.EffectiveGasPrice(ethMsg.Raw.Transaction, baseFee)
		} else {
			effectiveGasPrice = ethMsg.Raw.GasFeeCap()
		}

		var status uint64
		if txResult.Failed {
			status = ethtypes.ReceiptStatusFailed
		} else {
			status = ethtypes.ReceiptStatusSuccessful
		}

		contractAddress := common.Address{}
		if ethMsg.Raw.To() == nil {
			contractAddress = crypto.CreateAddress(ethMsg.GetSender(), ethMsg.Raw.Nonce())
		}

		var logs []*ethtypes.Log
		if additional != nil {
			// Derived tx: MsgIndex is math.MaxUint32 (sentinel). Parse logs from tx_log events
			// by matching TxHash instead of using the protobuf-encoded Data field.
			logs, err = derivedTxLogsFromEvents(
				blockRes.TxsResults[txResult.TxIndex].Events,
				additional.Hash,
				blockHeight,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to parse derived tx logs: %w", err)
			}
		} else {
			msgIndex := int(txResult.MsgIndex) // #nosec G115 -- checked for int overflow already
			logs, err = evmtypes.DecodeMsgLogs(
				blockRes.TxsResults[txResult.TxIndex].Data,
				msgIndex,
				blockHeight,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to convert tx result to eth receipt: %w", err)
			}
		}

		bloom := ethtypes.CreateBloom(&ethtypes.Receipt{Logs: logs})

		// Derived txs use the event hash as the canonical TxHash in the receipt.
		var txHash common.Hash
		if additional != nil {
			txHash = additional.Hash
		} else {
			txHash = ethMsg.Hash()
		}

		receipt := &ethtypes.Receipt{
			// Consensus fields: These fields are defined by the Yellow Paper
			Type:              ethMsg.Raw.Type(),
			PostState:         nil,
			Status:            status, // convert to 1=success, 0=failure
			CumulativeGasUsed: cumulatedGasUsed,
			Bloom:             bloom,
			Logs:              logs,

			// Implementation fields: These fields are added by geth when processing a transaction.
			TxHash:            txHash,
			ContractAddress:   contractAddress,
			GasUsed:           txResult.GasUsed,
			EffectiveGasPrice: effectiveGasPrice,
			BlobGasUsed:       uint64(0),     // TODO: fill this field
			BlobGasPrice:      big.NewInt(0), // TODO: fill this field

			// Inclusion information: These fields provide information about the inclusion of the
			// transaction corresponding to this receipt.
			BlockHash:        blockHash,
			BlockNumber:      big.NewInt(resBlock.Block.Height),
			TransactionIndex: uint(txResult.EthTxIndex), // #nosec G115 -- checked for int overflow already
		}

		receipts[i] = receipt
	}

	return receipts, nil
}

// BlockBloom is an alias for BlockBloomFromCometBlock kept for interface compatibility.
func (b *Backend) BlockBloom(blockRes *cmtrpctypes.ResultBlockResults) (ethtypes.Bloom, error) {
	return b.BlockBloomFromCometBlock(blockRes)
}

// BlockBloomFromCometBlock query block bloom filter from block results
func (b *Backend) BlockBloomFromCometBlock(blockRes *cmtrpctypes.ResultBlockResults) (ethtypes.Bloom, error) {
	for _, event := range blockRes.FinalizeBlockEvents {
		if event.Type != evmtypes.EventTypeBlockBloom {
			continue
		}

		for _, attr := range event.Attributes {
			if attr.Key == evmtypes.AttributeKeyEthereumBloom {
				return ethtypes.BytesToBloom([]byte(attr.Value)), nil
			}
		}
	}
	return ethtypes.Bloom{}, errors.New("block bloom event is not found")
}
