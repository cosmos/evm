package ante

import (
	"fmt"

	cosmosante "github.com/cosmos/evm/ante/cosmos"
	"github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/testutil/constants"
	testutiltx "github.com/cosmos/evm/testutil/tx"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

var execTypes = []struct {
	name      string
	isCheckTx bool
	simulate  bool
}{
	{"deliverTx", false, false},
	{"deliverTxSimulate", false, true},
}

func (s *AnteTestSuite) TestMinGasPriceDecorator() {
	denom := constants.ExampleAttoDenom
	testMsg := banktypes.MsgSend{
		FromAddress: "cosmos1x8fhpj9nmhqk8z9kpgjt95ck2xwyue0ptzkucp",
		ToAddress:   "cosmos1dx67l23hz9l0k9hcher8xz04uj7wf3yu26l2yn",
		Amount:      sdk.Coins{sdk.Coin{Amount: math.NewInt(10), Denom: denom}},
	}
	nw := s.GetNetwork()
	ctx := nw.GetContext()

	testCases := []struct {
		name                string
		malleate            func() sdk.Tx
		expPass             bool
		errMsg              string
		allowPassOnSimulate bool
	}{
		{
			"invalid cosmos tx type",
			func() sdk.Tx {
				return &testutiltx.InvalidTx{}
			},
			false,
			"invalid transaction type",
			false,
		},
		{
			"valid cosmos tx with MinGasPrices = 0, gasPrice = 0",
			func() sdk.Tx {
				params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
				params.MinGasPrice = math.LegacyZeroDec()
				err := nw.App.GetFeeMarketKeeper().SetParams(ctx, params)
				s.Require().NoError(err)

				txBuilder := s.CreateTestCosmosTxBuilder(math.NewInt(0), denom, &testMsg)
				return txBuilder.GetTx()
			},
			true,
			"",
			true,
		},
		{
			"valid cosmos tx with MinGasPrices = 0, gasPrice > 0",
			func() sdk.Tx {
				params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
				params.MinGasPrice = math.LegacyZeroDec()
				err := nw.App.GetFeeMarketKeeper().SetParams(ctx, params)
				s.Require().NoError(err)

				txBuilder := s.CreateTestCosmosTxBuilder(math.NewInt(10), denom, &testMsg)
				return txBuilder.GetTx()
			},
			true,
			"",
			true,
		},
		{
			"valid cosmos tx with MinGasPrices = 10, gasPrice = 10",
			func() sdk.Tx {
				params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
				params.MinGasPrice = math.LegacyNewDec(10)
				err := nw.App.GetFeeMarketKeeper().SetParams(ctx, params)
				s.Require().NoError(err)

				txBuilder := s.CreateTestCosmosTxBuilder(math.NewInt(10), denom, &testMsg)
				return txBuilder.GetTx()
			},
			true,
			"",
			true,
		},
		{
			"invalid cosmos tx with MinGasPrices = 10, gasPrice = 0",
			func() sdk.Tx {
				params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
				params.MinGasPrice = math.LegacyNewDec(10)
				err := nw.App.GetFeeMarketKeeper().SetParams(ctx, params)
				s.Require().NoError(err)

				txBuilder := s.CreateTestCosmosTxBuilder(math.NewInt(0), denom, &testMsg)
				return txBuilder.GetTx()
			},
			false,
			"provided fee < minimum global fee",
			true,
		},
		{
			"invalid cosmos tx with stake denom",
			func() sdk.Tx {
				params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
				params.MinGasPrice = math.LegacyNewDec(10)
				err := nw.App.GetFeeMarketKeeper().SetParams(ctx, params)
				s.Require().NoError(err)

				txBuilder := s.CreateTestCosmosTxBuilder(math.NewInt(10), sdk.DefaultBondDenom, &testMsg)
				return txBuilder.GetTx()
			},
			false,
			"provided fee < minimum global fee",
			true,
		},
		{
			"valid cosmos tx with MinGasPrices = 0, gasPrice = 0, valid fee",
			func() sdk.Tx {
				params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
				params.MinGasPrice = math.LegacyZeroDec()
				err := nw.App.GetFeeMarketKeeper().SetParams(ctx, params)
				s.Require().NoError(err)

				txBuilder := s.CreateTestCosmosTxBuilderWithFees(sdk.Coins{sdk.Coin{Amount: math.NewInt(0), Denom: denom}}, &testMsg)
				return txBuilder.GetTx()
			},
			true,
			"",
			true,
		},
		{
			"valid cosmos tx with MinGasPrices = 0, gasPrice = 0, nil fees, means len(fees) == 0",
			func() sdk.Tx {
				params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
				params.MinGasPrice = math.LegacyZeroDec()
				err := nw.App.GetFeeMarketKeeper().SetParams(ctx, params)
				s.Require().NoError(err)

				txBuilder := s.CreateTestCosmosTxBuilderWithFees(nil, &testMsg)
				return txBuilder.GetTx()
			},
			true,
			"",
			true,
		},
		{
			"valid cosmos tx with MinGasPrices = 0, gasPrice = 0, empty fees, means len(fees) == 0",
			func() sdk.Tx {
				params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
				params.MinGasPrice = math.LegacyZeroDec()
				err := nw.App.GetFeeMarketKeeper().SetParams(ctx, params)
				s.Require().NoError(err)

				txBuilder := s.CreateTestCosmosTxBuilderWithFees(sdk.Coins{}, &testMsg)
				return txBuilder.GetTx()
			},
			true,
			"",
			true,
		},
		{
			"valid cosmos tx with MinGasPrices = 0, gasPrice = 0, invalid fees",
			func() sdk.Tx {
				params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
				params.MinGasPrice = math.LegacyZeroDec()
				err := nw.App.GetFeeMarketKeeper().SetParams(ctx, params)
				s.Require().NoError(err)

				fees := sdk.Coins{sdk.Coin{Amount: math.NewInt(0), Denom: denom}, sdk.Coin{Amount: math.NewInt(10), Denom: "stake"}}
				txBuilder := s.CreateTestCosmosTxBuilderWithFees(fees, &testMsg)
				return txBuilder.GetTx()
			},
			false,
			fmt.Sprintf("expected only native token %s for fee", denom),
			true,
		},
	}

	for _, et := range execTypes {
		for _, tc := range testCases {
			s.Run(et.name+"_"+tc.name, func() {
				ctx := ctx.WithIsReCheckTx(et.isCheckTx)
				params := nw.App.GetFeeMarketKeeper().GetParams(ctx)
				dec := cosmosante.NewMinGasPriceDecorator(&params)
				_, err := dec.AnteHandle(ctx, tc.malleate(), et.simulate, testutil.NoOpNextFn)

				if (et.name == "deliverTx" && tc.expPass) || (et.name == "deliverTxSimulate" && et.simulate && tc.allowPassOnSimulate) {
					s.Require().NoError(err, tc.name)
				} else {
					s.Require().Error(err, tc.name)
					s.Require().Contains(err.Error(), tc.errMsg, tc.name)
				}
			})
		}
	}
}
