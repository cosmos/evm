package indexer

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/indexer"
	"github.com/cosmos/evm/testutil/integration/evm/network"
)

func TestStakingDelegateTransformer(t *testing.T, _ network.CreateEvmApp, _ ...network.ConfigOption) {
	stakingPrecompileAddr := common.HexToAddress("0x0000000000000000000000000000000000000800")
	transformer := NewStakingDelegateTransformer(stakingPrecompileAddr)

	testCases := []struct {
		name         string
		eventType    string
		expCanHandle bool
	}{
		{
			name:         "can handle delegate",
			eventType:    EventTypeDelegate,
			expCanHandle: true,
		},
		{
			name:         "cannot handle unbond",
			eventType:    "unbond",
			expCanHandle: false,
		},
		{
			name:         "cannot handle coin_spent",
			eventType:    "coin_spent",
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

func TestStakingDelegateTransformerTransform(t *testing.T, _ network.CreateEvmApp, _ ...network.ConfigOption) {
	stakingPrecompileAddr := common.HexToAddress("0x0000000000000000000000000000000000000800")
	transformer := NewStakingDelegateTransformer(stakingPrecompileAddr)

	validDelegatorAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())
	validValidatorAddr := sdk.ValAddress(common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").Bytes())

	testCases := []struct {
		name      string
		event     abci.Event
		ethTxHash common.Hash
		height    int64
		expErr    bool
		validate  func(*testing.T, *indexer.EthReceiptData)
	}{
		{
			name: "success - valid delegate event",
			event: abci.Event{
				Type: EventTypeDelegate,
				Attributes: []abci.EventAttribute{
					{Key: AttributeKeyDelegator, Value: validDelegatorAddr.String()},
					{Key: AttributeKeyValidator, Value: validValidatorAddr.String()},
					{Key: sdk.AttributeKeyAmount, Value: "5000stake"},
				},
			},
			ethTxHash: indexer.GenerateSyntheticEthTxHash([]byte("delegate_txhash")),
			height:       150,
						expErr:       false,
			validate: func(t *testing.T, data *indexer.EthReceiptData) {
				require.NotNil(t, data)
				require.Equal(t, uint64(1), data.Status)
				require.Equal(t, uint64(50000), data.GasUsed)
				require.Len(t, data.Logs, 1)
				require.Equal(t, stakingPrecompileAddr, data.Logs[0].Address)
				require.Len(t, data.Logs[0].Topics, 3)
				require.Equal(t, DelegateEventSignature, data.Logs[0].Topics[0])
				require.Equal(t, big.NewInt(5000), data.Value)
				// Data should contain amount + newShares (64 bytes)
				require.Len(t, data.Logs[0].Data, 64)
			},
		},
		{
			name: "fail - missing delegator",
			event: abci.Event{
				Type: EventTypeDelegate,
				Attributes: []abci.EventAttribute{
					{Key: AttributeKeyValidator, Value: validValidatorAddr.String()},
					{Key: sdk.AttributeKeyAmount, Value: "5000stake"},
				},
			},
			ethTxHash: indexer.GenerateSyntheticEthTxHash([]byte("txhash")),
			height:       100,
						expErr:       true,
		},
		{
			name: "fail - missing validator",
			event: abci.Event{
				Type: EventTypeDelegate,
				Attributes: []abci.EventAttribute{
					{Key: AttributeKeyDelegator, Value: validDelegatorAddr.String()},
					{Key: sdk.AttributeKeyAmount, Value: "5000stake"},
				},
			},
			ethTxHash: indexer.GenerateSyntheticEthTxHash([]byte("txhash")),
			height:       100,
						expErr:       true,
		},
		{
			name: "fail - missing amount",
			event: abci.Event{
				Type: EventTypeDelegate,
				Attributes: []abci.EventAttribute{
					{Key: AttributeKeyDelegator, Value: validDelegatorAddr.String()},
					{Key: AttributeKeyValidator, Value: validValidatorAddr.String()},
				},
			},
			ethTxHash: indexer.GenerateSyntheticEthTxHash([]byte("txhash")),
			height:       100,
						expErr:       true,
		},
		{
			name: "fail - invalid delegator address",
			event: abci.Event{
				Type: EventTypeDelegate,
				Attributes: []abci.EventAttribute{
					{Key: AttributeKeyDelegator, Value: "invalid_delegator"},
					{Key: AttributeKeyValidator, Value: validValidatorAddr.String()},
					{Key: sdk.AttributeKeyAmount, Value: "5000stake"},
				},
			},
			ethTxHash: indexer.GenerateSyntheticEthTxHash([]byte("txhash")),
			height:       100,
						expErr:       true,
		},
		{
			name: "fail - invalid validator address",
			event: abci.Event{
				Type: EventTypeDelegate,
				Attributes: []abci.EventAttribute{
					{Key: AttributeKeyDelegator, Value: validDelegatorAddr.String()},
					{Key: AttributeKeyValidator, Value: "invalid_validator"},
					{Key: sdk.AttributeKeyAmount, Value: "5000stake"},
				},
			},
			ethTxHash: indexer.GenerateSyntheticEthTxHash([]byte("txhash")),
			height:       100,
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

func TestStakingDelegateTransformerDeterministicHash(t *testing.T, _ network.CreateEvmApp, _ ...network.ConfigOption) {
	stakingPrecompileAddr := common.HexToAddress("0x0000000000000000000000000000000000000800")
	transformer := NewStakingDelegateTransformer(stakingPrecompileAddr)

	validDelegatorAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())
	validValidatorAddr := sdk.ValAddress(common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").Bytes())

	event := abci.Event{
		Type: EventTypeDelegate,
		Attributes: []abci.EventAttribute{
			{Key: AttributeKeyDelegator, Value: validDelegatorAddr.String()},
			{Key: AttributeKeyValidator, Value: validValidatorAddr.String()},
			{Key: sdk.AttributeKeyAmount, Value: "5000stake"},
		},
	}

	ethTxHash := indexer.GenerateSyntheticEthTxHash([]byte("deterministic_delegate_test"))
	height := int64(200)

	// Same eth tx hash should produce same result
	result1, err := transformer.Transform(event, height, ethTxHash)
	require.NoError(t, err)

	result2, err := transformer.Transform(event, height, ethTxHash)
	require.NoError(t, err)

	require.Equal(t, result1.EthTxHash, result2.EthTxHash)

	// Different eth tx hash should produce different result
	differentEthTxHash := indexer.GenerateSyntheticEthTxHash([]byte("different_delegate_test"))
	result3, err := transformer.Transform(event, height, differentEthTxHash)
	require.NoError(t, err)

	require.NotEqual(t, result1.EthTxHash, result3.EthTxHash)
}
