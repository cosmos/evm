package precisebank_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/os/network"
	"github.com/cosmos/evm/x/precisebank"
	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type GenesisTestSuite struct {
	suite.Suite

	network *network.UnitTestNetwork
}

func TestGenesisTestSuite(t *testing.T) {
	suite.Run(t, new(GenesisTestSuite))
}

func (suite *GenesisTestSuite) SetupTest() {
	suite.network = network.NewUnitTestNetwork(
		network.WithChainID(testconstants.SixDecimalsChainID),
	)
}

func (suite *GenesisTestSuite) TestInitGenesis() {
	tests := []struct {
		name         string
		setupFn      func()
		genesisState *types.GenesisState
		panicMsg     string
	}{
		{
			"valid - default genesisState",
			func() {},
			types.DefaultGenesisState(),
			"",
		},
		{
			"valid - empty genesisState",
			func() {},
			&types.GenesisState{},
			"failed to validate precisebank genesis state: nil remainder amount",
		},
		{
			"valid - module balance matches non-zero amount",
			func() {
				// Set module account balance to expected amount
				err := suite.network.App.BankKeeper.MintCoins(
					suite.network.GetContext(),
					types.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), sdkmath.NewInt(2))),
				)
				suite.Require().NoError(err)
			},
			types.NewGenesisState(
				types.FractionalBalances{
					types.NewFractionalBalance(sdk.AccAddress{1}.String(), types.ConversionFactor().SubRaw(1)),
					types.NewFractionalBalance(sdk.AccAddress{2}.String(), types.ConversionFactor().SubRaw(1)),
				},
				// 2 leftover from 0.999... + 0.999...
				sdkmath.NewInt(2),
			),
			"",
		},
		{
			// Other GenesisState.Validate() tests are in types/genesis_test.go
			"invalid genesisState - GenesisState.Validate() is called",
			func() {},
			types.NewGenesisState(
				types.FractionalBalances{
					types.NewFractionalBalance(sdk.AccAddress{1}.String(), sdkmath.NewInt(1)),
					types.NewFractionalBalance(sdk.AccAddress{1}.String(), sdkmath.NewInt(1)),
				},
				sdkmath.ZeroInt(),
			),
			"failed to validate precisebank genesis state: invalid balances: duplicate address cosmos1qyfkm2y3",
		},
		{
			"invalid - module balance insufficient",
			func() {},
			types.NewGenesisState(
				types.FractionalBalances{
					types.NewFractionalBalance(sdk.AccAddress{1}.String(), types.ConversionFactor().SubRaw(1)),
					types.NewFractionalBalance(sdk.AccAddress{2}.String(), types.ConversionFactor().SubRaw(1)),
				},
				// 2 leftover from 0.999... + 0.999...
				sdkmath.NewInt(2),
			),
			fmt.Sprintf("module account balance does not match sum of fractional balances and remainder, balance is 0%s but expected 2000000000000%s (2%s)",
				types.IntegerCoinDenom(), types.ExtendedCoinDenom(), types.IntegerCoinDenom()),
		},
		{
			"invalid - module balance excessive",
			func() {
				// Set module account balance to greater than expected amount
				err := suite.network.App.BankKeeper.MintCoins(
					suite.network.GetContext(),
					types.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), sdkmath.NewInt(100))),
				)
				suite.Require().NoError(err)
			},
			types.NewGenesisState(
				types.FractionalBalances{
					types.NewFractionalBalance(sdk.AccAddress{1}.String(), types.ConversionFactor().SubRaw(1)),
					types.NewFractionalBalance(sdk.AccAddress{2}.String(), types.ConversionFactor().SubRaw(1)),
				},
				sdkmath.NewInt(2),
			),
			fmt.Sprintf("module account balance does not match sum of fractional balances and remainder, balance is 100%s but expected 2000000000000%s (2%s)",
				types.IntegerCoinDenom(), types.ExtendedCoinDenom(), types.IntegerCoinDenom()),
		},
		{
			"sets module account",
			func() {
				// Delete the module account first to ensure it's created here
				moduleAcc := suite.network.App.AccountKeeper.GetModuleAccount(suite.network.GetContext(), types.ModuleName)
				suite.network.App.AccountKeeper.RemoveAccount(suite.network.GetContext(), moduleAcc)

				// Ensure module account is deleted in state.
				// GetModuleAccount() will always return non-nil and does not
				// necessarily equate to the account being stored in the account store.
				suite.Require().Nil(suite.network.App.AccountKeeper.GetAccount(suite.network.GetContext(), moduleAcc.GetAddress()))
			},
			types.DefaultGenesisState(),
			"",
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setupFn()

			if tc.panicMsg != "" {
				suite.Require().PanicsWithValue(
					tc.panicMsg,
					func() {
						precisebank.InitGenesis(
							suite.network.GetContext(),
							suite.network.App.PreciseBankKeeper,
							suite.network.App.AccountKeeper,
							suite.network.App.BankKeeper,
							tc.genesisState,
						)
					},
				)

				return
			}

			suite.Require().NotPanics(func() {
				precisebank.InitGenesis(
					suite.network.GetContext(),
					suite.network.App.PreciseBankKeeper,
					suite.network.App.AccountKeeper,
					suite.network.App.BankKeeper,
					tc.genesisState,
				)
			})

			// Ensure module account is created
			moduleAcc := suite.network.App.AccountKeeper.GetModuleAccount(suite.network.GetContext(), types.ModuleName)
			suite.NotNil(moduleAcc)
			suite.NotNil(
				suite.network.App.AccountKeeper.GetAccount(suite.network.GetContext(), moduleAcc.GetAddress()),
				"module account should be created & stored in account store",
			)

			// Verify balances are set in state, get full list of balances in
			// state to ensure they are set AND no extra balances are set
			var bals []types.FractionalBalance
			suite.network.App.PreciseBankKeeper.IterateFractionalBalances(suite.network.GetContext(), func(addr sdk.AccAddress, bal sdkmath.Int) bool {
				bals = append(bals, types.NewFractionalBalance(addr.String(), bal))

				return false
			})

			suite.Require().ElementsMatch(tc.genesisState.Balances, bals, "balances should be set in state")

			remainder := suite.network.App.PreciseBankKeeper.GetRemainderAmount(suite.network.GetContext())
			suite.Require().Equal(tc.genesisState.Remainder, remainder, "remainder should be set in state")
		})
	}
}

func (suite *GenesisTestSuite) TestExportGenesis() {
	// ExportGenesis(InitGenesis(genesisState)) == genesisState
	// Must also be valid.

	tests := []struct {
		name             string
		initGenesisState func() *types.GenesisState
	}{
		{
			"InitGenesis(DefaultGenesisState)",
			types.DefaultGenesisState,
		},
		{
			"balances, no remainder",
			func() *types.GenesisState {
				err := suite.network.App.BankKeeper.MintCoins(
					suite.network.GetContext(),
					types.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), sdkmath.NewInt(1))),
				)
				suite.Require().NoError(err)

				return types.NewGenesisState(
					types.FractionalBalances{
						types.NewFractionalBalance(sdk.AccAddress{1}.String(), types.ConversionFactor().QuoRaw(2)),
						types.NewFractionalBalance(sdk.AccAddress{2}.String(), types.ConversionFactor().QuoRaw(2)),
					},
					sdkmath.ZeroInt(),
				)
			},
		},
		{
			"balances, remainder",
			func() *types.GenesisState {
				err := suite.network.App.BankKeeper.MintCoins(
					suite.network.GetContext(),
					types.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), sdkmath.NewInt(1))),
				)
				suite.Require().NoError(err)

				return types.NewGenesisState(
					types.FractionalBalances{
						types.NewFractionalBalance(sdk.AccAddress{1}.String(), types.ConversionFactor().QuoRaw(2)),
						types.NewFractionalBalance(sdk.AccAddress{2}.String(), types.ConversionFactor().QuoRaw(2).SubRaw(1)),
					},
					sdkmath.OneInt(),
				)
			},
		},
	}

	for _, tc := range tests {
		suite.Run(tc.name, func() {
			// Reset state
			suite.SetupTest()

			initGs := tc.initGenesisState()

			suite.Require().NotPanics(func() {
				precisebank.InitGenesis(
					suite.network.GetContext(),
					suite.network.App.PreciseBankKeeper,
					suite.network.App.AccountKeeper,
					suite.network.App.BankKeeper,
					initGs,
				)
			})

			genesisState := precisebank.ExportGenesis(suite.network.GetContext(), suite.network.App.PreciseBankKeeper)
			suite.Require().NoError(genesisState.Validate(), "exported genesis state should be valid")

			suite.Require().Equal(
				initGs,
				genesisState,
				"exported genesis state should equal initial genesis state",
			)
		})
	}
}
