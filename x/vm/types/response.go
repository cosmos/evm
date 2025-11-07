package types

import (
	"strconv"

	abci "github.com/cometbft/cometbft/abci/types"

	proto "github.com/cosmos/gogoproto/proto"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PatchTxResponses fills the evm tx index and log indexes in the tx result
func PatchTxResponses(input []*abci.ExecTxResult) []*abci.ExecTxResult {
	var (
		txIndex  uint64
		logIndex uint64
	)
	for _, res := range input {
		// assume no error result in msg handler
		if res.Code != 0 {
			continue
		}

		var txMsgData sdk.TxMsgData
		if err := proto.Unmarshal(res.Data, &txMsgData); err != nil {
			panic(err)
		}

		var (
			anteEvents []abci.Event
			// if the response data is modified and need to be marshaled back
			dataDirty bool
		)
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
