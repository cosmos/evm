package types

import (
	"testing"

	"github.com/stretchr/testify/require"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type rpcMsgEthereumTxWrapper struct {
	*evmtypes.MsgEthereumTx
}

func (m *rpcMsgEthereumTxWrapper) GetMsgs() []sdk.Msg {
	return []sdk.Msg{m}
}

type txConfigDecoder struct {
	client.TxConfig
	decoder sdk.TxDecoder
}

func (t txConfigDecoder) TxDecoder() sdk.TxDecoder {
	return t.decoder
}

func TestRawTxToEthTxRPCMsg(t *testing.T) {
	wrappedMsg := &rpcMsgEthereumTxWrapper{
		MsgEthereumTx: &evmtypes.MsgEthereumTx{},
	}
	clientCtx := client.Context{}.WithTxConfig(txConfigDecoder{
		decoder: func([]byte) (sdk.Tx, error) {
			return wrappedMsg, nil
		},
	})

	txs, err := RawTxToEthTx(clientCtx, []byte{0x01})

	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Same(t, wrappedMsg, txs[0])
}
