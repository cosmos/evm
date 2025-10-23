package utils

import (
	abcitypes "github.com/cometbft/cometbft/abci/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func DecodeExecTxResult(res abcitypes.ExecTxResult) (*evmtypes.MsgEthereumTxResponse, error) {
	msgEthResponse, err := evmtypes.DecodeTxResponse(res.Data)
	if err != nil {
		return nil, err
	}
	return msgEthResponse, nil
}
