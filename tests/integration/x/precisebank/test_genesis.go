package precisebank

import (
	"fmt"

	"github.com/stretchr/testify/suite"

	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/x/precisebank"
	"github.com/cosmos/evm/x/precisebank/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type GenesisTestSuite struct {
	suite.Suite

	network *network.UnitTestNetwork
	create  network.CreateEvmApp
	options []network.ConfigOption
}

func NewGenesisTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *GenesisTestSuite {
	return &GenesisTestSuite{
		create:  create,
		options: options,
	}
}

func (s *GenesisTestSuite) SetupTest() {
	s.SetupTestWithChainID(testconstants.SixDecimalsChainID)
}

func (s *GenesisTestSuite) SetupTestWithChainID(chainID testconstants.ChainID) {
	options := []network.ConfigOption{
		network.WithChainID(chainID),
	}
	options = append(options, s.options...)
	s.network = network.NewUnitTestNetwork(s.create, options...)

	// Clear all fractional balances to ensure no leftover balances persist between tests
	s.network.App.GetPreciseBankKeeper().IterateFractionalBalances(s.network.GetContext(), func(addr sdk.AccAddress, bal sdkmath.Int) bool {
		s.network.App.GetPreciseBankKeeper().DeleteFractionalBalance(s.network.GetContext(), addr)
		return false
	})
}

func (s *GenesisTestSuite) TestInitGenesis() {
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
				// The network setup creates an initial balance of 1, so we need to mint 1 more
				// to get to the expected amount of 2 for this test case
				err := s.network.App.GetBankKeeper().MintCoins(
					s.network.GetContext(),
					types.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), sdkmath.NewInt(1))),
				)
				s.Require().NoError(err)
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
			func() {
				// The network setup creates an initial balance of 1, so we need to burn that
				// to get to 0 balance for this test case
				err := s.network.App.GetBankKeeper().BurnCoins(
					s.network.GetContext(),
					types.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), sdkmath.NewInt(1))),
				)
				s.Require().NoError(err)
			},
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
				// The network setup creates an initial balance of 1, so we need to mint 99 more
				// to get to 100 total balance for this test case
				err := s.network.App.GetBankKeeper().MintCoins(
					s.network.GetContext(),
					types.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), sdkmath.NewInt(99))),
				)
				s.Require().NoError(err)
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
				moduleAcc := s.network.App.GetAccountKeeper().GetModuleAccount(s.network.GetContext(), types.ModuleName)
				s.network.App.GetAccountKeeper().RemoveAccount(s.network.GetContext(), moduleAcc)

				// Ensure module account is deleted in state.
				// GetModuleAccount() will always return non-nil and does not
				// necessarily equate to the account being stored in the account store.
				s.Require().Nil(s.network.App.GetAccountKeeper().GetAccount(s.network.GetContext(), moduleAcc.GetAddress()))
			},
			types.DefaultGenesisState(),
			"",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.setupFn()

			if tc.panicMsg != "" {
				s.Require().PanicsWithValue(
					tc.panicMsg,
					func() {
						precisebank.InitGenesis(
							s.network.GetContext(),
							*s.network.App.GetPreciseBankKeeper(),
							s.network.App.GetAccountKeeper(),
							s.network.App.GetBankKeeper(),
							tc.genesisState,
						)
					},
				)

				return
			}

			s.Require().NotPanics(func() {
				precisebank.InitGenesis(
					s.network.GetContext(),
					*s.network.App.GetPreciseBankKeeper(),
					s.network.App.GetAccountKeeper(),
					s.network.App.GetBankKeeper(),
					tc.genesisState,
				)
			})

			// Ensure module account is created
			moduleAcc := s.network.App.GetAccountKeeper().GetModuleAccount(s.network.GetContext(), types.ModuleName)
			s.NotNil(moduleAcc)
			s.NotNil(
				s.network.App.GetAccountKeeper().GetAccount(s.network.GetContext(), moduleAcc.GetAddress()),
				"module account should be created & stored in account store",
			)

			// Verify balances are set in state, get full list of balances in
			// state to ensure they are set AND no extra balances are set
			var bals []types.FractionalBalance
			s.network.App.GetPreciseBankKeeper().IterateFractionalBalances(s.network.GetContext(), func(addr sdk.AccAddress, bal sdkmath.Int) bool {
				bals = append(bals, types.NewFractionalBalance(addr.String(), bal))
				return false
			})
			s.Require().ElementsMatch(tc.genesisState.Balances, bals, "balances should be set in state")

			remainder := s.network.App.GetPreciseBankKeeper().GetRemainderAmount(s.network.GetContext())
			s.Require().Equal(tc.genesisState.Remainder, remainder, "remainder should be set in state")
		})
	}
}

func (s *GenesisTestSuite) TestExportGenesis() {
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
				// Burn the initial balance created by network setup, then mint the expected amount
				err := s.network.App.GetBankKeeper().BurnCoins(
					s.network.GetContext(),
					types.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), sdkmath.NewInt(1))),
				)
				s.Require().NoError(err)

				err = s.network.App.GetBankKeeper().MintCoins(
					s.network.GetContext(),
					types.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), sdkmath.NewInt(1))),
				)
				s.Require().NoError(err)

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
				// Burn the initial balance created by network setup, then mint the expected amount
				err := s.network.App.GetBankKeeper().BurnCoins(
					s.network.GetContext(),
					types.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), sdkmath.NewInt(1))),
				)
				s.Require().NoError(err)

				err = s.network.App.GetBankKeeper().MintCoins(
					s.network.GetContext(),
					types.ModuleName,
					sdk.NewCoins(sdk.NewCoin(types.IntegerCoinDenom(), sdkmath.NewInt(1))),
				)
				s.Require().NoError(err)

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
		s.Run(tc.name, func() {
			// Reset state
			s.SetupTest()

			initGs := tc.initGenesisState()

			s.Require().NotPanics(func() {
				precisebank.InitGenesis(
					s.network.GetContext(),
					*s.network.App.GetPreciseBankKeeper(),
					s.network.App.GetAccountKeeper(),
					s.network.App.GetBankKeeper(),
					initGs,
				)
			})

			genesisState := precisebank.ExportGenesis(s.network.GetContext(), *s.network.App.GetPreciseBankKeeper())
			s.Require().NoError(genesisState.Validate(), "exported genesis state should be valid")

			s.Require().Equal(
				initGs,
				genesisState,
				"exported genesis state should equal initial genesis state",
			)
		})
	}
}
