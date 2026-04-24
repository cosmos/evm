package types

import (
	"strconv"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/gogoproto/proto"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PatchTxResponses rewrites block-cumulative / eth-only indices that cannot
// be computed per-tx under BlockSTM:
//
//   - log.Index (cumulative across the block) and log.TxIndex (eth-only tx
//     counter) inside MsgEthereumTxResponse payloads.
//   - AttributeKeyTxIndex on ante-emitted ethereum_tx events, rewritten from
//     ctx.TxIndex() (cosmos-level position) to the eth-only counter so the
//     indexer's stored EthTxIndex — which becomes receipt.TransactionIndex
//     in RPC receipts — stays aligned with log.TxIndex in mixed-tx blocks.
//
// Must be invoked once per block on the full ExecTxResult slice produced by
// the TxRunner.
func PatchTxResponses(input []*abci.ExecTxResult) ([]*abci.ExecTxResult, error) {
	var (
		ethTxIndex uint64
		logIndex   uint64
	)
	for _, res := range input {
		rewritten := rewriteEthTxEventIndex(res.Events, ethTxIndex)

		if res.Code != 0 {
			ethTxIndex += uint64(rewritten) //#nosec G115 -- int overflow is not a concern here
			continue
		}

		var txMsgData sdk.TxMsgData
		if err := proto.Unmarshal(res.Data, &txMsgData); err != nil {
			ethTxIndex += uint64(rewritten) //#nosec G115 -- int overflow is not a concern here
			continue
		}

		dataDirty := false
		for i, rsp := range txMsgData.MsgResponses {
			var response MsgEthereumTxResponse
			if rsp.TypeUrl != "/"+proto.MessageName(&response) {
				continue
			}
			if err := proto.Unmarshal(rsp.Value, &response); err != nil {
				return nil, err
			}

			if len(response.Logs) > 0 {
				for _, log := range response.Logs {
					log.TxIndex = ethTxIndex
					log.Index = logIndex
					logIndex++
				}

				anyRsp, err := codectypes.NewAnyWithValue(&response)
				if err != nil {
					return nil, err
				}
				txMsgData.MsgResponses[i] = anyRsp
				dataDirty = true
			}

			ethTxIndex++
		}

		if dataDirty {
			data, err := proto.Marshal(&txMsgData)
			if err != nil {
				return nil, err
			}
			res.Data = data
		}
	}
	return input, nil
}

// rewriteEthTxEventIndex rewrites AttributeKeyTxIndex on every ethereum_tx
// event in events to start, start+1, ... and returns the number of events
// rewritten.
func rewriteEthTxEventIndex(events []abci.Event, start uint64) int {
	n := 0
	for eIdx := range events {
		if events[eIdx].Type != EventTypeEthereumTx {
			continue
		}
		for aIdx := range events[eIdx].Attributes {
			if events[eIdx].Attributes[aIdx].Key == AttributeKeyTxIndex {
				events[eIdx].Attributes[aIdx].Value = strconv.FormatUint(start+uint64(n), 10) //#nosec G115 -- int overflow is not a concern here
			}
		}
		n++
	}
	return n
}
