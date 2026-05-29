package erc20

import (
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/testutil"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	"github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

// Define useful variables for tests here.
var (
	// tooShort is a denomination with a name that will raise the "denom too short" error
	tooShort = types.NewDenom("ab", types.NewHop(types.PortID, "channel-0"))
	// validDenom is a denomination with a valid IBC voucher name
	validDenom = types.NewDenom("uosmo", types.NewHop(types.PortID, "channel-0"))
	// validAttoDenom is a denomination with a valid IBC voucher name and 18 decimals
	validAttoDenom = types.NewDenom("aatom", types.NewHop(types.PortID, "channel-0"))
	// validDenomNoMicroAtto is a denomination with a valid IBC voucher name but no micro or atto prefix
	validDenomNoMicroAtto = types.NewDenom("matom", types.NewHop(types.PortID, "channel-0"))

	// --------------------
	// Variables for coin with valid metadata
	//

	// validMetadataDenom is the base denomination of the coin with valid metadata
	validMetadataDenom = "uatom"
	// validMetadataDisplay is the denomination displayed of the coin with valid metadata
	validMetadataDisplay = "atom"
	// validMetadataName is the name of the coin with valid metadata
	validMetadataName = "Atom"
	// validMetadataSymbol is the symbol of the coin with valid metadata
	validMetadataSymbol = "ATOM"

	// validMetadata is the metadata of the coin with valid metadata
	validMetadata = banktypes.Metadata{
		Description: "description",
		Base:        validMetadataDenom,
		// NOTE: Denom units MUST be increasing
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    validMetadataDenom,
				Exponent: 0,
			},
			{
				Denom:    validMetadataDisplay,
				Exponent: uint32(6),
			},
		},
		Name:    validMetadataName,
		Symbol:  validMetadataSymbol,
		Display: validMetadataDisplay,
	}

	// overflowMetadata contains a metadata with an exponent that overflows uint8
	overflowMetadata = banktypes.Metadata{
		Description: "description",
		Base:        validMetadataDenom,
		// NOTE: Denom units MUST be increasing
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    validMetadataDenom,
				Exponent: 0,
			},
			{
				Denom:    validMetadataDisplay,
				Exponent: uint32(math.MaxUint8 + 1),
			},
		},
		Name:    validMetadataName,
		Symbol:  validMetadataSymbol,
		Display: validMetadataDisplay,
	}

	// noDisplayMetadata contains a metadata where the denom units do not contain with no display denom
	noDisplayMetadata = banktypes.Metadata{
		Description: "description",
		Base:        validMetadataDenom,
		// NOTE: Denom units MUST be increasing
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    validMetadataDenom,
				Exponent: 0,
			},
		},
		Name:    validMetadataName,
		Symbol:  validMetadataSymbol,
		Display: "",
	}
)

// TestNameSymbolDecimals tests the Name and Symbol methods of the ERC20 precompile.
//
// NOTE: we test both methods in the same test because they need the same testcases and
// the same setup.
func (s *PrecompileTestSuite) TestNameSymbol() {
	nameMethod := s.precompile.Methods[erc20.NameMethod]
	symbolMethod := s.precompile.Methods[erc20.SymbolMethod]

	testcases := []struct {
		name        string
		denom       string
		malleate    func(sdk.Context, bankkeeper.Keeper, transferkeeper.Keeper)
		expPass     bool
		errContains string
		expName     string
		expSymbol   string
	}{
		{
			name:        "fail - invalid denom trace",
			denom:       tooShort.IBCDenom()[:len(tooShort.IBCDenom())-1],
			errContains: "odd length hex string",
		},
		{
			name:        "fail - denom not found",
			denom:       types.NewDenom("notfound", types.NewHop(types.PortID, "channel-0")).IBCDenom(),
			errContains: vm.ErrExecutionReverted.Error(),
		},
		{
			name:  "fail - invalid denom (too short < 3 chars)",
			denom: tooShort.IBCDenom(),
			malleate: func(ctx sdk.Context, _ bankkeeper.Keeper, keeper transferkeeper.Keeper) {
				keeper.SetDenom(ctx, tooShort)
			},
			errContains: vm.ErrExecutionReverted.Error(),
		},
		{
			name:        "fail - denom without metadata and not an IBC voucher",
			denom:       "noIBCvoucher",
			errContains: vm.ErrExecutionReverted.Error(),
		},
		{
			name:  "pass - valid ibc denom without metadata and neither atto nor micro prefix",
			denom: validDenomNoMicroAtto.IBCDenom(),
			malleate: func(ctx sdk.Context, _ bankkeeper.Keeper, keeper transferkeeper.Keeper) {
				keeper.SetDenom(ctx, validDenomNoMicroAtto)
			},
			expPass:   true,
			expName:   "Atom",
			expSymbol: "ATOM",
		},
		{
			name:  "pass - valid denom with metadata",
			denom: validMetadataDenom,
			malleate: func(ctx sdk.Context, keeper bankkeeper.Keeper, _ transferkeeper.Keeper) {
				// NOTE: we mint some coins to the inflation module address to be able to set denom metadata
				err := keeper.MintCoins(ctx, minttypes.ModuleName, sdk.Coins{sdk.NewInt64Coin(validMetadata.Base, 1)})
				s.Require().NoError(err)

				// NOTE: we set the denom metadata for the coin
				keeper.SetDenomMetaData(ctx, validMetadata)
			},
			expPass:   true,
			expName:   "Atom",
			expSymbol: "ATOM",
		},
		{
			name:  "pass - valid ibc denom without metadata",
			denom: validDenom.IBCDenom(),
			malleate: func(ctx sdk.Context, _ bankkeeper.Keeper, keeper transferkeeper.Keeper) {
				keeper.SetDenom(ctx, validDenom)
			},
			expPass:   true,
			expName:   "Osmo",
			expSymbol: "OSMO",
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()

			if tc.malleate != nil {
				tc.malleate(s.network.GetContext(), s.network.App.GetBankKeeper(), s.network.App.GetTransferKeeper())
			}

			precompile, err := s.setupERC20Precompile(tc.denom)
			s.Require().NoError(err)

			s.Run("name", func() {
				bz, err := precompile.Name(
					s.network.GetContext(),
					nil,
					nil,
					&nameMethod,
					[]interface{}{},
				)

				// NOTE: all output and error checking happens in here
				s.requireOut(bz, err, nameMethod, tc.expPass, tc.errContains, tc.expName)
			})

			s.Run("symbol", func() {
				bz, err := precompile.Symbol(
					s.network.GetContext(),
					nil,
					nil,
					&symbolMethod,
					[]interface{}{},
				)

				// NOTE: all output and error checking happens in here
				s.requireOut(bz, err, symbolMethod, tc.expPass, tc.errContains, tc.expSymbol)
			})
		})
	}
}

func (s *PrecompileTestSuite) TestDecimals() {
	DecimalsMethod := s.precompile.Methods[erc20.DecimalsMethod]

	testcases := []struct {
		name        string
		denom       string
		malleate    func(sdk.Context, bankkeeper.Keeper, transferkeeper.Keeper)
		expPass     bool
		errContains string
		expDecimals uint8
	}{
		{
			name:        "fail - invalid denom trace",
			denom:       tooShort.IBCDenom()[:len(tooShort.IBCDenom())-1],
			errContains: "odd length hex string",
		},
		{
			name:        "fail - denom not found",
			denom:       types.NewDenom("notfound", types.NewHop(types.PortID, "channel-0")).IBCDenom(),
			errContains: vm.ErrExecutionReverted.Error(),
		},
		{
			name:        "fail - denom without metadata and not an IBC voucher",
			denom:       "noIBCvoucher",
			errContains: vm.ErrExecutionReverted.Error(),
		},
		{
			name:  "fail - valid ibc denom without metadata and neither atto nor micro prefix",
			denom: validDenomNoMicroAtto.IBCDenom(),
			malleate: func(ctx sdk.Context, _ bankkeeper.Keeper, keeper transferkeeper.Keeper) {
				keeper.SetDenom(ctx, validDenomNoMicroAtto)
			},
			errContains: vm.ErrExecutionReverted.Error(),
		},
		{
			name:  "pass - invalid denom (too short < 3 chars)",
			denom: tooShort.IBCDenom(),
			malleate: func(ctx sdk.Context, _ bankkeeper.Keeper, keeper transferkeeper.Keeper) {
				keeper.SetDenom(ctx, tooShort)
			},
			expPass:     true, // TODO: do we want to check in decimals query for the above error?
			expDecimals: 18,   // expect 18 decimals here because of "a" prefix
		},
		{
			name:  "pass - valid denom with metadata",
			denom: validMetadataDenom,
			malleate: func(ctx sdk.Context, keeper bankkeeper.Keeper, _ transferkeeper.Keeper) {
				// NOTE: we mint some coins to the inflation module address to be able to set denom metadata
				err := keeper.MintCoins(ctx, minttypes.ModuleName, sdk.Coins{sdk.NewInt64Coin(validMetadata.Base, 1)})
				s.Require().NoError(err)

				// NOTE: we set the denom metadata for the coin
				keeper.SetDenomMetaData(ctx, validMetadata)
			},
			expPass:     true,
			expDecimals: 6,
		},
		{
			name:  "pass - valid ibc denom without metadata",
			denom: validDenom.IBCDenom(),
			malleate: func(ctx sdk.Context, _ bankkeeper.Keeper, keeper transferkeeper.Keeper) {
				keeper.SetDenom(ctx, validDenom)
			},
			expPass:     true,
			expDecimals: 6,
		},
		{
			name:  "pass - valid ibc denom without metadata and 18 decimals",
			denom: validAttoDenom.IBCDenom(),
			malleate: func(ctx sdk.Context, _ bankkeeper.Keeper, keeper transferkeeper.Keeper) {
				keeper.SetDenom(ctx, validAttoDenom)
			},
			expPass:     true,
			expDecimals: 18,
		},
		{
			name:  "pass - valid denom with metadata but decimals overflow",
			denom: validMetadataDenom,
			malleate: func(ctx sdk.Context, keeper bankkeeper.Keeper, _ transferkeeper.Keeper) {
				// NOTE: we mint some coins to the inflation module address to be able to set denom metadata
				err := keeper.MintCoins(ctx, minttypes.ModuleName, sdk.Coins{sdk.NewInt64Coin(validMetadata.Base, 1)})
				s.Require().NoError(err)

				// NOTE: we set the denom metadata for the coin
				keeper.SetDenomMetaData(s.network.GetContext(), overflowMetadata)
			},
			errContains: vm.ErrExecutionReverted.Error(),
		},
		{
			name:  "pass - valid ibc denom with metadata but no display denom",
			denom: validMetadataDenom,
			malleate: func(ctx sdk.Context, keeper bankkeeper.Keeper, _ transferkeeper.Keeper) {
				// NOTE: we mint some coins to the inflation module address to be able to set denom metadata
				err := keeper.MintCoins(ctx, minttypes.ModuleName, sdk.Coins{sdk.NewInt64Coin(validMetadata.Base, 1)})
				s.Require().NoError(err)

				// NOTE: we set the denom metadata for the coin
				keeper.SetDenomMetaData(ctx, noDisplayMetadata)
			},
			errContains: vm.ErrExecutionReverted.Error(),
		},
		{
			name:  "pass - valid IBC denom with metadata using display path",
			denom: "ibc/B89BE1E96B3DBC0ABB05F858F08561BA12B9C5E420CA2F5E83C475CCB47A834E",
			malleate: func(ctx sdk.Context, keeper bankkeeper.Keeper, _ transferkeeper.Keeper) {
				keeper.SetDenomMetaData(ctx, banktypes.Metadata{
					Base: "ibc/B89BE1E96B3DBC0ABB05F858F08561BA12B9C5E420CA2F5E83C475CCB47A834E",
					DenomUnits: []*banktypes.DenomUnit{
						{
							Denom:    "ibc/B89BE1E96B3DBC0ABB05F858F08561BA12B9C5E420CA2F5E83C475CCB47A834E",
							Exponent: 0,
						},
						{
							Denom:    "uom",
							Exponent: 6,
						},
					},
					Display: "transfer/channel-0/uom",
					Name:    "transfer/channel-0/uom IBC token",
					Symbol:  "UOM",
				})
			},
			expPass:     true,
			expDecimals: 6,
		},
		{
			name:  "fail - IBC denom with metadata but no matching display unit",
			denom: "ibc/C1D2E3F4567890123456789012345678901234567890123456789012345678901234",
			malleate: func(ctx sdk.Context, keeper bankkeeper.Keeper, _ transferkeeper.Keeper) {
				keeper.SetDenomMetaData(ctx, banktypes.Metadata{
					Base: "ibc/C1D2E3F4567890123456789012345678901234567890123456789012345678901234",
					DenomUnits: []*banktypes.DenomUnit{
						{
							Denom:    "ibc/C1D2E3F4567890123456789012345678901234567890123456789012345678901234",
							Exponent: 0,
						},
						{
							Denom:    "nomatch",
							Exponent: 6,
						},
					},
					Display: "transfer/channel-0/lastpart",
					Name:    "Mismatched Token",
					Symbol:  "MISMATCH",
				})
			},
			expPass:     false,
			errContains: "execution reverted",
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()

			if tc.malleate != nil {
				tc.malleate(s.network.GetContext(), s.network.App.GetBankKeeper(), s.network.App.GetTransferKeeper())
			}

			precompile, err := s.setupERC20Precompile(tc.denom)
			s.Require().NoError(err)

			bz, err := precompile.Decimals(
				s.network.GetContext(),
				nil,
				nil,
				&DecimalsMethod,
				[]interface{}{},
			)

			// NOTE: all output and error checking happens in here
			s.requireOut(bz, err, DecimalsMethod, tc.expPass, tc.errContains, tc.expDecimals)
		})
	}
}

func (s *PrecompileTestSuite) TestTotalSupply() {
	method := s.precompile.Methods[erc20.TotalSupplyMethod]

	testcases := []struct {
		name        string
		malleate    func(sdk.Context, bankkeeper.Keeper, *big.Int)
		expPass     bool
		errContains string
		expTotal    *big.Int
	}{
		{
			name:     "pass - no coins",
			expPass:  true,
			expTotal: common.Big0,
		},
		{
			name: "pass - some coins",
			malleate: func(ctx sdk.Context, keeper bankkeeper.Keeper, amount *big.Int) {
				// NOTE: we mint some coins to the inflation module address to be able to set denom metadata
				err := keeper.MintCoins(ctx, minttypes.ModuleName, sdk.Coins{sdk.NewCoin(validMetadata.Base, sdkmath.NewIntFromBigInt(amount))})
				s.Require().NoError(err)
			},
			expPass:  true,
			expTotal: big.NewInt(100),
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()

			if tc.malleate != nil {
				tc.malleate(s.network.GetContext(), s.network.App.GetBankKeeper(), tc.expTotal)
			}

			precompile, err := s.setupERC20Precompile(validMetadataDenom)
			s.Require().NoError(err)

			bz, err := precompile.TotalSupply(
				s.network.GetContext(),
				nil,
				nil,
				&method,
				[]interface{}{},
			)

			// NOTE: all output and error checking happens in here
			s.requireOut(bz, err, method, tc.expPass, tc.errContains, tc.expTotal)
		})
	}
}

func (s *PrecompileTestSuite) TestBalanceOf() {
	method := s.precompile.Methods[erc20.BalanceOfMethod]

	testcases := []struct {
		name        string
		malleate    func(sdk.Context, bankkeeper.Keeper, *big.Int) []interface{}
		expPass     bool
		errContains string
		expBalance  *big.Int
	}{
		{
			name: "fail - invalid number of arguments",
			malleate: func(_ sdk.Context, _ bankkeeper.Keeper, _ *big.Int) []interface{} {
				return []interface{}{}
			},
			errContains: "invalid number of arguments; expected 1; got: 0",
		},
		{
			name: "fail - invalid address",
			malleate: func(_ sdk.Context, _ bankkeeper.Keeper, _ *big.Int) []interface{} {
				return []interface{}{"invalid address"}
			},
			errContains: "invalid account address: invalid address",
		},
		{
			name: "pass - no coins in token denomination of precompile token pair",
			malleate: func(_ sdk.Context, keeper bankkeeper.Keeper, _ *big.Int) []interface{} {
				// NOTE: we fund the account with some coins in a different denomination from what was used in the precompile.
				err := testutil.FundAccount(
					s.network.GetContext(), keeper, s.keyring.GetAccAddr(0), sdk.NewCoins(sdk.NewInt64Coin(s.bondDenom, 100)),
				)
				s.Require().NoError(err, "expected no error funding account")

				return []interface{}{s.keyring.GetAddr(0)}
			},
			expPass:    true,
			expBalance: common.Big0,
		},
		{
			name: "pass - some coins",
			malleate: func(ctx sdk.Context, keeper bankkeeper.Keeper, amount *big.Int) []interface{} {
				// NOTE: we fund the account with some coins of the token denomination that was used for the precompile
				err := testutil.FundAccount(
					ctx, keeper, s.keyring.GetAccAddr(0), sdk.NewCoins(sdk.NewCoin(s.tokenDenom, sdkmath.NewIntFromBigInt(amount))),
				)
				s.Require().NoError(err, "expected no error funding account")

				return []interface{}{s.keyring.GetAddr(0)}
			},
			expPass:    true,
			expBalance: big.NewInt(100),
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()

			var balanceOfArgs []interface{}
			if tc.malleate != nil {
				balanceOfArgs = tc.malleate(s.network.GetContext(), s.network.App.GetBankKeeper(), tc.expBalance)
			}

			bz, err := s.precompile.BalanceOf(
				s.network.GetContext(),
				nil,
				nil,
				&method,
				balanceOfArgs,
			)

			// NOTE: all output and error checking happens in here
			s.requireOut(bz, err, method, tc.expPass, tc.errContains, tc.expBalance)
		})
	}
}

func (s *PrecompileTestSuite) TestAllowance() {
	method := s.precompile.Methods[erc20.AllowanceMethod]

	testcases := []struct {
		name        string
		malleate    func(sdk.Context, *big.Int) []interface{}
		expPass     bool
		errContains string
		expAllow    *big.Int
	}{
		{
			name: "fail - invalid number of arguments",
			malleate: func(_ sdk.Context, _ *big.Int) []interface{} {
				return []interface{}{1}
			},
			errContains: "invalid number of arguments; expected 2; got: 1",
		},
		{
			name: "fail - invalid owner address",
			malleate: func(_ sdk.Context, _ *big.Int) []interface{} {
				return []interface{}{"invalid address", s.keyring.GetAddr(1)}
			},
			errContains: "invalid owner address: invalid address",
		},
		{
			name: "fail - invalid spender address",
			malleate: func(_ sdk.Context, _ *big.Int) []interface{} {
				return []interface{}{s.keyring.GetAddr(0), "invalid address"}
			},
			errContains: "invalid spender address: invalid address",
		},
		{
			name: "pass - no allowance exists should return 0",
			malleate: func(_ sdk.Context, _ *big.Int) []interface{} {
				return []interface{}{s.keyring.GetAddr(0), s.keyring.GetAddr(1)}
			},
			expPass:  true,
			expAllow: common.Big0,
		},
		{
			name: "pass - allowance exists for precompile token pair denom",
			malleate: func(_ sdk.Context, amount *big.Int) []interface{} {
				ownerIdx := 0
				spenderIdx := 1

				s.setAllowance(
					s.precompile.Address(),
					s.keyring.GetPrivKey(ownerIdx),
					s.keyring.GetAddr(spenderIdx),
					amount,
				)

				return []interface{}{s.keyring.GetAddr(ownerIdx), s.keyring.GetAddr(spenderIdx)}
			},
			expPass:  true,
			expAllow: big.NewInt(100),
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()

			var allowanceArgs []interface{}
			if tc.malleate != nil {
				allowanceArgs = tc.malleate(s.network.GetContext(), tc.expAllow)
			}

			bz, err := s.precompile.Allowance(
				s.network.GetContext(),
				nil,
				nil,
				&method,
				allowanceArgs,
			)

			// NOTE: all output and error checking happens in here
			s.requireOut(bz, err, method, tc.expPass, tc.errContains, tc.expAllow)
		})
	}
}
