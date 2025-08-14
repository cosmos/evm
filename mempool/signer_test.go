package mempool_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	mempool2 "github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/x/vm/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	mempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

type mockFallback struct {
	called bool
}

func (m *mockFallback) GetSigners(tx sdk.Tx) ([]mempool.SignerData, error) {
	m.called = true
	return []mempool.SignerData{mempool.NewSignerData(sdk.AccAddress("fallback"), 1)}, nil
}

type mockHasExtOptions struct {
	msg sdk.Msg
}

func (m *mockHasExtOptions) GetMsgs() []sdk.Msg { return []sdk.Msg{m.msg} }
func (m *mockHasExtOptions) GetMsgsV2() ([]protov2.Message, error) {
	return []protov2.Message{}, nil
}

func (m *mockHasExtOptions) GetExtensionOptions() []*codectypes.Any {
	return []*codectypes.Any{
		{
			TypeUrl: "/cosmos.evm.vm.v1.ExtensionOptionsEthereumTx",
			Value:   []byte{},
		},
	}
}
func (m *mockHasExtOptions) GetNonCriticalExtensionOptions() []*codectypes.Any { return nil }

func TestGetSigners(t *testing.T) {
	ethAddr := sdk.AccAddress("ethsigner")
	evmTx := &types.EvmTxArgs{
		ChainID:   big.NewInt(100),
		Nonce:     1,
		Amount:    big.NewInt(10),
		GasLimit:  100000,
		GasPrice:  big.NewInt(150),
		GasFeeCap: big.NewInt(200),
	}
	ethMsg := types.NewTx(evmTx)
	ethMsg.From = ethAddr.Bytes()
	txWithEth := &mockHasExtOptions{
		msg: ethMsg,
	}
	fallback := &mockFallback{}
	adapter := mempool2.NewEthSignerExtractionAdapter(fallback)
	signers, err := adapter.GetSigners(txWithEth)
	require.NoError(t, err)
	require.Equal(t, []mempool.SignerData{
		mempool.NewSignerData(
			ethMsg.GetFrom(),
			ethMsg.AsTransaction().Nonce(),
		),
	}, signers)
	require.False(t, fallback.called)

	fallback = &mockFallback{}
	txWithEth = &mockHasExtOptions{}
	adapter = mempool2.NewEthSignerExtractionAdapter(fallback)
	signers, err = adapter.GetSigners(txWithEth)
	require.NoError(t, err)
	fallbackSigners, err := new(mockFallback).GetSigners(txWithEth)
	require.NoError(t, err)
	require.Equal(t, fallbackSigners, signers)
	require.True(t, fallback.called)
}
