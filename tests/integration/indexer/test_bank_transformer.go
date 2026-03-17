package indexer

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/cosmos/evm/indexer"
	"github.com/cosmos/evm/testutil/integration/evm/network"
)

func TestBankTransferTransformer(t *testing.T, _ network.CreateEvmApp, _ ...network.ConfigOption) {
	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	transformer := NewBankTransferTransformer(tokenAddress)

	testCases := []struct {
		name         string
		eventType    string
		expCanHandle bool
	}{
		{
			name:         "can handle coin_spent",
			eventType:    banktypes.EventTypeCoinSpent,
			expCanHandle: true,
		},
		{
			name:         "can handle coin_received",
			eventType:    banktypes.EventTypeCoinReceived,
			expCanHandle: true,
		},
		{
			name:         "cannot handle transfer",
			eventType:    "transfer",
			expCanHandle: false,
		},
		{
			name:         "cannot handle ethereum_tx",
			eventType:    "ethereum_tx",
			expCanHandle: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := transformer.CanHandle(tc.eventType)
			require.Equal(t, tc.expCanHandle, result)
		})
	}
}

func TestBankTransferTransformerTransform(t *testing.T, _ network.CreateEvmApp, _ ...network.ConfigOption) {
	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	transformer := NewBankTransferTransformer(tokenAddress)

	validSpenderAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())
	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").Bytes())

	testCases := []struct {
		name       string
		event      abci.Event
		ethTxHash  common.Hash
		height     int64
		eventIndex int
		expErr     bool
		validate   func(*testing.T, *indexer.TransformedTxData)
	}{
		{
			name: "success - coin_spent event",
			event: abci.Event{
				Type: banktypes.EventTypeCoinSpent,
				Attributes: []abci.EventAttribute{
					{Key: banktypes.AttributeKeySpender, Value: validSpenderAddr.String()},
					{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
				},
			},
			ethTxHash: indexer.GenerateTransformedEthTxHash([]byte("txhash123")),
			height:       100,
			eventIndex:   0,
			expErr:       false,
			validate: func(t *testing.T, data *indexer.TransformedTxData) {
				require.NotNil(t, data)
				require.Equal(t, uint64(1), data.Status)
				require.Equal(t, uint64(21000), data.GasUsed)
				require.Len(t, data.Logs, 1)
				require.Equal(t, tokenAddress, data.Logs[0].Address)
				require.Len(t, data.Logs[0].Topics, 3)
				require.Equal(t, TransferEventSignature, data.Logs[0].Topics[0])
				require.Equal(t, big.NewInt(1000), data.Value)
			},
		},
		{
			name: "success - coin_received event",
			event: abci.Event{
				Type: banktypes.EventTypeCoinReceived,
				Attributes: []abci.EventAttribute{
					{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
					{Key: sdk.AttributeKeyAmount, Value: "2000stake"},
				},
			},
			ethTxHash: indexer.GenerateTransformedEthTxHash([]byte("txhash456")),
			height:       200,
			eventIndex:   1,
			expErr:       false,
			validate: func(t *testing.T, data *indexer.TransformedTxData) {
				require.NotNil(t, data)
				require.Equal(t, uint64(1), data.Status)
				require.Len(t, data.Logs, 1)
				require.Equal(t, big.NewInt(2000), data.Value)
			},
		},
		{
			name: "fail - missing spender in coin_spent",
			event: abci.Event{
				Type: banktypes.EventTypeCoinSpent,
				Attributes: []abci.EventAttribute{
					{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
				},
			},
			ethTxHash: indexer.GenerateTransformedEthTxHash([]byte("txhash789")),
			height:       100,
			eventIndex:   0,
			expErr:       true,
		},
		{
			name: "fail - missing receiver in coin_received",
			event: abci.Event{
				Type: banktypes.EventTypeCoinReceived,
				Attributes: []abci.EventAttribute{
					{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
				},
			},
			ethTxHash: indexer.GenerateTransformedEthTxHash([]byte("txhash789")),
			height:       100,
			eventIndex:   0,
			expErr:       true,
		},
		{
			name: "fail - missing amount in coin_spent",
			event: abci.Event{
				Type: banktypes.EventTypeCoinSpent,
				Attributes: []abci.EventAttribute{
					{Key: banktypes.AttributeKeySpender, Value: validSpenderAddr.String()},
				},
			},
			ethTxHash: indexer.GenerateTransformedEthTxHash([]byte("txhash789")),
			height:       100,
			eventIndex:   0,
			expErr:       true,
		},
		{
			name: "fail - invalid spender address",
			event: abci.Event{
				Type: banktypes.EventTypeCoinSpent,
				Attributes: []abci.EventAttribute{
					{Key: banktypes.AttributeKeySpender, Value: "invalid_address"},
					{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
				},
			},
			ethTxHash: indexer.GenerateTransformedEthTxHash([]byte("txhash789")),
			height:       100,
			eventIndex:   0,
			expErr:       true,
		},
		{
			name: "fail - unsupported event type",
			event: abci.Event{
				Type: "unsupported",
				Attributes: []abci.EventAttribute{
					{Key: "key", Value: "value"},
				},
			},
			ethTxHash: indexer.GenerateTransformedEthTxHash([]byte("txhash789")),
			height:       100,
			eventIndex:   0,
			expErr:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := transformer.Transform(tc.event, tc.height, tc.ethTxHash)

			if tc.expErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tc.validate != nil {
				tc.validate(t, result)
			}
		})
	}
}

func TestBankTransferTransformerDeterministicHash(t *testing.T, _ network.CreateEvmApp, _ ...network.ConfigOption) {
	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	transformer := NewBankTransferTransformer(tokenAddress)

	validSpenderAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())

	event := abci.Event{
		Type: banktypes.EventTypeCoinSpent,
		Attributes: []abci.EventAttribute{
			{Key: banktypes.AttributeKeySpender, Value: validSpenderAddr.String()},
			{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
		},
	}

	ethTxHash := indexer.GenerateTransformedEthTxHash([]byte("deterministic_hash_test"))
	height := int64(100)

	result1, err := transformer.Transform(event, height, ethTxHash)
	require.NoError(t, err)

	result2, err := transformer.Transform(event, height, ethTxHash)
	require.NoError(t, err)

	// Same ethTxHash produces same result
	require.Equal(t, result1.EthTxHash, result2.EthTxHash)

	// Different ethTxHash produces different result
	differentEthTxHash := indexer.GenerateTransformedEthTxHash([]byte("different_hash_test"))
	result3, err := transformer.Transform(event, height, differentEthTxHash)
	require.NoError(t, err)

	require.NotEqual(t, result1.EthTxHash, result3.EthTxHash)
}
