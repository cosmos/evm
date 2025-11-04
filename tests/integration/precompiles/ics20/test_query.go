package ics20

import (
	"github.com/cosmos/evm"
	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/ics20"
	precompiletestutil "github.com/cosmos/evm/precompiles/testutil"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestDenoms() {
	denom := precompiletestutil.UosmoDenom

	for _, tc := range []struct {
		name        string
		args        *ics20.DenomsCall
		malleate    func(ctx sdk.Context)
		expErr      bool
		errContains string
		expDenom    transfertypes.Denom
	}{
		{
			name: "success",
			args: ics20.NewDenomsCall(cmn.PageRequest{Limit: 10, CountTotal: true}),
			malleate: func(ctx sdk.Context) {
				evmApp := s.chainA.App.(evm.EvmApp)
				evmApp.GetTransferKeeper().SetDenom(ctx, denom)
			},
			expDenom: denom,
		},
	} {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx := s.chainA.GetContext()
			if tc.malleate != nil {
				tc.malleate(ctx)
			}
			out, err := s.chainAPrecompile.Denoms(ctx, *tc.args)

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(out.Denoms)
				s.Require().Equal(tc.expDenom, out.Denoms[0])
			}
		})
	}
}

func (s *PrecompileTestSuite) TestDenom() {
	denom := precompiletestutil.UosmoDenom

	for _, tc := range []struct {
		name        string
		arg         *ics20.DenomCall
		malleate    func(ctx sdk.Context)
		expErr      bool
		errContains string
		expDenom    transfertypes.Denom
	}{
		{
			name: "success - denom found",
			arg:  ics20.NewDenomCall(denom.Hash().String()),
			malleate: func(ctx sdk.Context) {
				evmApp := s.chainA.App.(evm.EvmApp)
				evmApp.GetTransferKeeper().SetDenom(ctx, denom)
			},
			expDenom: denom,
		},
		{
			name:     "success - denom not found",
			arg:      ics20.NewDenomCall("0000000000000000000000000000000000000000000000000000000000000000"),
			malleate: func(ctx sdk.Context) {},
			expDenom: transfertypes.Denom{Base: "", Trace: []transfertypes.Hop{}},
		},
		{
			name:        "fail - invalid hash",
			arg:         ics20.NewDenomCall("INVALID-DENOM-HASH"),
			malleate:    func(ctx sdk.Context) {},
			expErr:      true,
			errContains: "invalid denom trace hash",
		},
	} {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx := s.chainA.GetContext()
			if tc.malleate != nil {
				tc.malleate(ctx)
			}
			out, err := s.chainAPrecompile.Denom(ctx, *tc.arg)

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().Equal(tc.expDenom, out.Denom)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestDenomHash() {
	denom := precompiletestutil.UosmoDenom

	for _, tc := range []struct {
		name        string
		arg         *ics20.DenomHashCall
		malleate    func(ctx sdk.Context)
		expErr      bool
		errContains string
		expHash     string
	}{
		{
			name: "success",
			arg:  ics20.NewDenomHashCall(denom.Path()),
			malleate: func(ctx sdk.Context) {
				evmApp := s.chainA.App.(evm.EvmApp)
				evmApp.GetTransferKeeper().SetDenom(ctx, denom)
			},
			expHash: denom.Hash().String(),
		},
		{
			name:     "success - not found",
			arg:      ics20.NewDenomHashCall("transfer/channel-0/erc20:not-exists-case"),
			malleate: func(ctx sdk.Context) {},
			expHash:  "",
		},
		{
			name:        "fail - invalid denom",
			arg:         ics20.NewDenomHashCall(""),
			malleate:    func(ctx sdk.Context) {},
			expErr:      true,
			errContains: "invalid denomination for cross-chain transfer",
		},
	} {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx := s.chainA.GetContext()
			if tc.malleate != nil {
				tc.malleate(ctx)
			}
			out, err := s.chainAPrecompile.DenomHash(ctx, *tc.arg)

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().Equal(tc.expHash, out.Hash)
			}
		})
	}
}
