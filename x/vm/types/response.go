package types

import (
	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/gogoproto/proto"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// PatchTxResponses rewrites log.TxIndex (eth-only tx counter) and log.Index
// (cumulative across the block) inside MsgEthereumTxResponse payloads. These
// values cannot be computed per-tx because they depend on the ordering and
// log counts of every prior successful eth tx in the block; under BlockSTM
// the invariant is invisible at execution time. Must be invoked once per
// block on the full ExecTxResult slice produced by the TxRunner.
func PatchTxResponses(input []*abci.ExecTxResult) ([]*abci.ExecTxResult, error) {
	var (
		ethTxIndex uint64
		logIndex   uint64
	)
	for _, res := range input {
		if res.Code != 0 {
			continue
		}

		var txMsgData sdk.TxMsgData
		if err := proto.Unmarshal(res.Data, &txMsgData); err != nil {
			return nil, err
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
