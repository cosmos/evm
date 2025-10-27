package evm_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/ante/types"
	"github.com/cosmos/evm/encoding"
	testconstants "github.com/cosmos/evm/testutil/constants"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	"cosmossdk.io/math"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
)

func TestSDKTxFeeChecker(t *testing.T) {
	// testCases:
	//   fallback
	//      genesis tx
	//      checkTx, validate with min-gas-prices
	//      deliverTx, no validation
	//   dynamic fee
	//      with extension option
	//      without extension option
	//      london hardfork enableness
	chainID := uint64(testconstants.EighteenDecimalsChainID)
	encodingConfig := encoding.MakeConfig(chainID) //nolint:staticcheck // this is used

	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	// set global chain config
	ethCfg := evmtypes.DefaultChainConfig(chainID)
	if err := evmtypes.SetChainConfig(ethCfg); err != nil {
		panic(err)
	}
	err := configurator.
		WithExtendedEips(evmtypes.DefaultCosmosEVMActivators).
		// NOTE: we're using the 18 decimals default for the example chain
		WithEVMCoinInfo(testconstants.ChainsCoinInfo[chainID]).
		Configure()
	require.NoError(t, err)
	if err != nil {
		panic(err)
	}

	evmDenom := evmtypes.GetEVMCoinDenom()
	minGasPrices := sdk.NewDecCoins(sdk.NewDecCoin(evmDenom, math.NewInt(10)))

	genesisCtx := sdk.NewContext(nil, tmproto.Header{}, false, log.NewNopLogger())
	checkTxCtx := sdk.NewContext(nil, tmproto.Header{Height: 1}, true, log.NewNopLogger()).WithMinGasPrices(minGasPrices)
	deliverTxCtx := sdk.NewContext(nil, tmproto.Header{Height: 1}, false, log.NewNopLogger())

	feemarketParams := feemarkettypes.Params{}

	testCases := []struct {
		name              string
		ctx               sdk.Context
		feemarketParamsFn func() feemarkettypes.Params
		buildTx           func() sdk.FeeTx
		londonEnabled     bool
		expFees           string
		expPriority       int64
		expSuccess        bool
	}{
		{
			"success, genesis tx",
			genesisCtx,
			func() feemarkettypes.Params { return feemarketParams },
			func() sdk.FeeTx {
				return encodingConfig.TxConfig.NewTxBuilder().GetTx()
			},
			false,
			"",
			0,
			true,
		},
		{
			"fail, min-gas-prices",
			checkTxCtx,
			func() feemarkettypes.Params { return feemarketParams },
			func() sdk.FeeTx {
				return encodingConfig.TxConfig.NewTxBuilder().GetTx()
			},
			false,
			"",
			0,
			false,
		},
		{
			"success, min-gas-prices",
			checkTxCtx,
			func() feemarkettypes.Params { return feemarketParams },
			func() sdk.FeeTx {
				txBuilder := encodingConfig.TxConfig.NewTxBuilder()
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(10))))
				return txBuilder.GetTx()
			},
			false,
			"10aatom",
			0,
			true,
		},
		{
			"success, min-gas-prices deliverTx",
			deliverTxCtx,
			func() feemarkettypes.Params { return feemarketParams },
			func() sdk.FeeTx {
				return encodingConfig.TxConfig.NewTxBuilder().GetTx()
			},
			false,
			"",
			0,
			true,
		},
		{
			"fail, dynamic fee",
			deliverTxCtx,
			func() feemarkettypes.Params {
				feemarketParams.BaseFee = math.LegacyNewDec(1)
				return feemarketParams
			},
			func() sdk.FeeTx {
				txBuilder := encodingConfig.TxConfig.NewTxBuilder()
				txBuilder.SetGasLimit(1)
				return txBuilder.GetTx()
			},
			true,
			"",
			0,
			false,
		},
		{
			"success, dynamic fee",
			deliverTxCtx,
			func() feemarkettypes.Params {
				feemarketParams.BaseFee = math.LegacyNewDec(10)
				return feemarketParams
			},
			func() sdk.FeeTx {
				txBuilder := encodingConfig.TxConfig.NewTxBuilder()
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(10))))
				return txBuilder.GetTx()
			},
			true,
			"10aatom",
			0,
			true,
		},
		{
			"success, dynamic fee priority",
			deliverTxCtx,
			func() feemarkettypes.Params {
				feemarketParams.BaseFee = math.LegacyNewDec(10)
				return feemarketParams
			},
			func() sdk.FeeTx {
				txBuilder := encodingConfig.TxConfig.NewTxBuilder()
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(10).Mul(evmtypes.DefaultPriorityReduction).Add(math.NewInt(10)))))
				return txBuilder.GetTx()
			},
			true,
			"10000010aatom",
			10,
			true,
		},
		{
			"success, dynamic fee empty tipFeeCap",
			deliverTxCtx,
			func() feemarkettypes.Params {
				feemarketParams.BaseFee = math.LegacyNewDec(10)
				return feemarketParams
			},
			func() sdk.FeeTx {
				txBuilder := encodingConfig.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(10).Mul(evmtypes.DefaultPriorityReduction))))

				option, err := codectypes.NewAnyWithValue(&types.ExtensionOptionDynamicFeeTx{})
				require.NoError(t, err)
				txBuilder.SetExtensionOptions(option)
				return txBuilder.GetTx()
			},
			true,
			"10aatom",
			0,
			true,
		},
		{
			"success, dynamic fee tipFeeCap",
			deliverTxCtx,
			func() feemarkettypes.Params {
				feemarketParams.BaseFee = math.LegacyNewDec(10)
				return feemarketParams
			},
			func() sdk.FeeTx {
				txBuilder := encodingConfig.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(10).Mul(evmtypes.DefaultPriorityReduction).Add(math.NewInt(10)))))

				option, err := codectypes.NewAnyWithValue(&types.ExtensionOptionDynamicFeeTx{
					MaxPriorityPrice: math.LegacyNewDec(5).MulInt(evmtypes.DefaultPriorityReduction),
				})
				require.NoError(t, err)
				txBuilder.SetExtensionOptions(option)
				return txBuilder.GetTx()
			},
			true,
			"5000010aatom",
			5,
			true,
		},
		{
			"fail, negative dynamic fee tipFeeCap",
			deliverTxCtx,
			func() feemarkettypes.Params {
				feemarketParams.BaseFee = math.LegacyNewDec(10)
				return feemarketParams
			},
			func() sdk.FeeTx {
				txBuilder := encodingConfig.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
				txBuilder.SetGasLimit(1)
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(10).Mul(evmtypes.DefaultPriorityReduction).Add(math.NewInt(10)))))

				// set negative priority fee
				option, err := codectypes.NewAnyWithValue(&types.ExtensionOptionDynamicFeeTx{
					MaxPriorityPrice: math.LegacyNewDec(-5).MulInt(evmtypes.DefaultPriorityReduction),
				})
				require.NoError(t, err)
				txBuilder.SetExtensionOptions(option)
				return txBuilder.GetTx()
			},
			true,
			"",
			0,
			false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := evmtypes.GetEthChainConfig()
			if !tc.londonEnabled {
				cfg.LondonBlock = big.NewInt(10000)
			} else {
				cfg.LondonBlock = big.NewInt(0)
			}
			feemarketParams := tc.feemarketParamsFn()
			fees, priority, err := evm.NewDynamicFeeChecker(&feemarketParams)(tc.ctx, tc.buildTx())
			if tc.expSuccess {
				require.Equal(t, tc.expFees, fees.String())
				require.Equal(t, tc.expPriority, priority)
			} else {
				require.Error(t, err)
			}
		})
	}
}
