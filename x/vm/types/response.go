package types

import (
	"strconv"
	"strings"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/gogoproto/proto"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PatchTxResponses fills the evm tx index and log indexes in the tx result.
// Note: txIndex starts at 0 and is incremented for each Ethereum transaction found.
// This ensures proper indexing when multiple EVM transactions are included in a block.
// For expected-failure txs, rawTxs and txDecoder are used to decode the original
// transaction and emit ethereum_tx events with the correct hash and txIndex; this was
// handled via `ante/evm/11_emit_event.go` previously.
func PatchTxResponses(input []*abci.ExecTxResult, rawTxs [][]byte, txDecoder sdk.TxDecoder) []*abci.ExecTxResult {
	var (
		txIndex  uint64
		logIndex uint64
	)

	for idx, res := range input {
		var (
			anteEvents []abci.Event
			txMsgData  sdk.TxMsgData
			dataDirty  bool
		)

		if res.Code != 0 {
			if !isExpectedFailure(res) {
				continue
			}
			// expected failure: decode tx to get eth hash for event emission
			if txDecoder != nil && idx < len(rawTxs) {
				decodedTx, err := txDecoder(rawTxs[idx])
				if err != nil {
					panic(err)
				}
				for _, m := range decodedTx.GetMsgs() {
					ethMsg, ok := m.(*MsgEthereumTx)
					if !ok {
						continue
					}
					anteEvents = append(anteEvents, abci.Event{
						Type: EventTypeEthereumTx,
						Attributes: []abci.EventAttribute{
							{Key: AttributeKeyEthereumTxHash, Value: ethMsg.Hash().Hex()},
							{Key: AttributeKeyTxIndex, Value: strconv.FormatUint(txIndex, 10)},
						},
					})
					txIndex++
				}
			}
		} else {
			if err := proto.Unmarshal(res.Data, &txMsgData); err != nil {
				panic(err)
			}

			for i, rsp := range txMsgData.MsgResponses {
				var response MsgEthereumTxResponse
				if rsp.TypeUrl != "/"+proto.MessageName(&response) {
					continue
				}

				if err := proto.Unmarshal(rsp.Value, &response); err != nil {
					panic(err)
				}

				anteEvents = append(anteEvents, abci.Event{
					Type: EventTypeEthereumTx,
					Attributes: []abci.EventAttribute{
						{Key: AttributeKeyEthereumTxHash, Value: response.Hash},
						{Key: AttributeKeyTxIndex, Value: strconv.FormatUint(txIndex, 10)},
					},
				})

				if len(response.Logs) > 0 {
					for _, log := range response.Logs {
						log.TxIndex = txIndex
						log.Index = logIndex
						logIndex++
					}

					anyRsp, err := codectypes.NewAnyWithValue(&response)
					if err != nil {
						panic(err)
					}
					txMsgData.MsgResponses[i] = anyRsp

					dataDirty = true
				}

				txIndex++
			}
		}

		if len(anteEvents) > 0 {
			// prepend ante events in front to emulate the side effect of `EthEmitEventDecorator`
			events := make([]abci.Event, len(anteEvents)+len(res.Events))
			copy(events, anteEvents)
			copy(events[len(anteEvents):], res.Events)
			res.Events = events

			if dataDirty {
				data, err := proto.Marshal(&txMsgData)
				if err != nil {
					panic(err)
				}

				res.Data = data
			}
		}
	}
	return input
}

// isExpectedFailure returns true if the tx failed with an error that still
// counts as a valid ethereum transaction (fee was deducted, nonce incremented).
func isExpectedFailure(res *abci.ExecTxResult) bool {
	return strings.Contains(res.Log, ExceedBlockGasLimitError) ||
		strings.Contains(res.Log, StateDBCommitError)
}
