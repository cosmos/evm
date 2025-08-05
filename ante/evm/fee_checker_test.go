package evm_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/ante/evm"
	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/testutil/config"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	"cosmossdk.io/math"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
)

var _ anteinterfaces.FeeMarketKeeper = MockFeemarketKeeper{}

type MockFeemarketKeeper struct {
	BaseFee math.LegacyDec
}

func (m MockFeemarketKeeper) GetBaseFee(_ sdk.Context) math.LegacyDec {
	return m.BaseFee
}

func (m MockFeemarketKeeper) GetBaseFeeEnabled(_ sdk.Context) bool {
	return true
}

func (m MockFeemarketKeeper) AddTransientGasWanted(_ sdk.Context, _ uint64) (uint64, error) {
	return 0, nil
}

func (m MockFeemarketKeeper) GetParams(_ sdk.Context) (params feemarkettypes.Params) {
	return feemarkettypes.DefaultParams()
}

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
	chainID := uint64(config.EighteenDecimalsChainID)
	encodingConfig := encoding.MakeConfig(chainID)
	err := config.EvmAppOptions(chainID)
	require.NoError(t, err)

	evmDenom := evmtypes.GetEVMCoinDenom()
	minGasPrices := sdk.NewDecCoins(sdk.NewDecCoin(evmDenom, math.NewInt(10)))

	genesisCtx := sdk.NewContext(nil, tmproto.Header{}, false, log.NewNopLogger())
	checkTxCtx := sdk.NewContext(nil, tmproto.Header{Height: 1}, true, log.NewNopLogger()).WithMinGasPrices(minGasPrices)
	deliverTxCtx := sdk.NewContext(nil, tmproto.Header{Height: 1}, false, log.NewNopLogger())

	testCases := []struct {
		name          string
		ctx           sdk.Context
		keeper        anteinterfaces.FeeMarketKeeper
		buildTx       func() sdk.FeeTx
		londonEnabled bool
		expFees       string
		expPriority   int64
		expSuccess    bool
	}{
		{
			"success, genesis tx",
			genesisCtx,
			MockFeemarketKeeper{},
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
			MockFeemarketKeeper{},
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
			MockFeemarketKeeper{},
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
			MockFeemarketKeeper{},
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
			MockFeemarketKeeper{
				BaseFee: math.LegacyNewDec(1),
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
			MockFeemarketKeeper{
				BaseFee: math.LegacyNewDec(10),
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
			MockFeemarketKeeper{
				BaseFee: math.LegacyNewDec(10),
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
			MockFeemarketKeeper{
				BaseFee: math.LegacyNewDec(10),
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
			MockFeemarketKeeper{
				BaseFee: math.LegacyNewDec(10),
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
			MockFeemarketKeeper{
				BaseFee: math.LegacyNewDec(10),
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
			fees, priority, err := evm.NewDynamicFeeChecker(tc.keeper)(tc.ctx, tc.buildTx())
			if tc.expSuccess {
				require.Equal(t, tc.expFees, fees.String())
				require.Equal(t, tc.expPriority, priority)
			} else {
				require.Error(t, err)
			}
		})
	}
}
