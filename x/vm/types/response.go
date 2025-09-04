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
			continue
		}

		var dirty bool
		for i, rsp := range txMsgData.MsgResponses {
			var response MsgEthereumTxResponse
			if rsp.TypeUrl != "/"+proto.MessageName(&response) {
				continue
			}
			if err := proto.Unmarshal(rsp.Value, &response); err != nil {
				continue
			}

			res.Events = append(res.Events, abci.Event(sdk.NewEvent(
				EventTypeEthereumTx,
				sdk.NewAttribute(AttributeKeyTxIndex, strconv.FormatUint(txIndex, 10)),
			)))
			for _, log := range response.Logs {
				log.TxIndex = txIndex
				log.Index = logIndex
				logIndex++
			}
			txIndex++

			dirty = true
			anyRsp, err := codectypes.NewAnyWithValue(&response)
			if err != nil {
				panic(err)
			}
			txMsgData.MsgResponses[i] = anyRsp
		}

		if dirty {
			if data, err := proto.Marshal(&txMsgData); err != nil {
				res.Data = data
			}
		}
	}
	return input
}
