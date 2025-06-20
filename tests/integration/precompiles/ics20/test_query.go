package ics20

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm"
	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/ics20"
	precompiletestutil "github.com/cosmos/evm/precompiles/testutil"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

func (s *PrecompileTestSuite) TestDenoms() {
	method := s.chainAPrecompile.Methods[ics20.DenomsMethod]

	denom := precompiletestutil.UosmoDenom

	for _, tc := range []struct {
		name        string
		args        []interface{}
		malleate    func(ctx sdk.Context)
		expErr      bool
		errContains string
		expDenom    transfertypes.Denom
	}{
		{
			name:        "fail - invalid number of arguments",
			args:        []interface{}{},
			malleate:    func(ctx sdk.Context) {},
			expErr:      true,
			errContains: fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 0),
		},
		{
			name:        "fail - invalid arg type",
			args:        []interface{}{true},
			malleate:    func(ctx sdk.Context) {},
			expErr:      true,
			errContains: "NumField on bool Value",
		},
		{
			name: "success",
			args: []interface{}{query.PageRequest{Limit: 10, CountTotal: true}},
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
			bz, err := s.chainAPrecompile.Denoms(ctx, nil, &method, tc.args)

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				var out ics20.DenomsResponse
				err = s.chainAPrecompile.UnpackIntoInterface(&out, ics20.DenomsMethod, bz)
				s.Require().NoError(err)
				s.Require().NotEmpty(out.Denoms)
				s.Require().Equal(tc.expDenom, out.Denoms[0])
			}
		})
	}
}

func (s *PrecompileTestSuite) TestDenom() {
	method := s.chainAPrecompile.Methods[ics20.DenomMethod]
	gas := uint64(100000)

	denom := precompiletestutil.UosmoDenom

	for _, tc := range []struct {
		name        string
		arg         interface{}
		malleate    func(ctx sdk.Context)
		expErr      bool
		errContains string
		expDenom    transfertypes.Denom
	}{
		{
			name:        "fail - invalid number of arguments",
			arg:         nil,
			malleate:    func(ctx sdk.Context) {},
			expErr:      true,
			errContains: "invalid input arguments",
		},
		{
			name:        "fail - invalid type",
			arg:         1,
			malleate:    func(ctx sdk.Context) {},
			expErr:      true,
			errContains: "invalid hash",
		},
		{
			name: "success - denom found",
			arg:  denom.Hash().String(),
			malleate: func(ctx sdk.Context) {
				evmApp := s.chainA.App.(evm.EvmApp)
				evmApp.GetTransferKeeper().SetDenom(ctx, denom)
			},
			expDenom: denom,
		},
		{
			name:     "success - denom not found",
			arg:      "0000000000000000000000000000000000000000000000000000000000000000",
			malleate: func(ctx sdk.Context) {},
			expDenom: transfertypes.Denom{Base: "", Trace: []transfertypes.Hop{}},
		},
		{
			name:        "fail - invalid hash",
			arg:         "INVALID-DENOM-HASH",
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
			caller := common.BytesToAddress(s.chainA.SenderAccount.GetAddress().Bytes())
			contract, ctx := precompiletestutil.NewPrecompileContract(s.T(), ctx, caller, s.chainAPrecompile.Address(), gas)

			args := []interface{}{}
			if tc.arg != nil {
				args = append(args, tc.arg)
			}

			bz, err := s.chainAPrecompile.Denom(ctx, contract, &method, args)

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				var out ics20.DenomResponse
				err = s.chainAPrecompile.UnpackIntoInterface(&out, ics20.DenomMethod, bz)
				s.Require().NoError(err)
				s.Require().Equal(tc.expDenom, out.Denom)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestDenomHash() {
	method := s.chainAPrecompile.Methods[ics20.DenomHashMethod]
	gas := uint64(100000)

	denom := precompiletestutil.UosmoDenom

	for _, tc := range []struct {
		name        string
		arg         interface{}
		malleate    func(ctx sdk.Context)
		expErr      bool
		errContains string
		expHash     string
	}{
		{
			name:        "fail - invalid number of arguments",
			arg:         nil,
			malleate:    func(ctx sdk.Context) {},
			expErr:      true,
			errContains: "invalid input arguments",
		},
		{
			name:        "fail - invalid type",
			arg:         1,
			malleate:    func(ctx sdk.Context) {},
			expErr:      true,
			errContains: "invalid trace",
		},
		{
			name: "success",
			arg:  denom.Path(),
			malleate: func(ctx sdk.Context) {
				evmApp := s.chainA.App.(evm.EvmApp)
				evmApp.GetTransferKeeper().SetDenom(ctx, denom)
			},
			expHash: denom.Hash().String(),
		},
		{
			name:     "success - not found",
			arg:      "transfer/channel-0/erc20:not-exists-case",
			malleate: func(ctx sdk.Context) {},
			expHash:  "",
		},
		{
			name:        "fail - invalid denom",
			arg:         "",
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
			caller := common.BytesToAddress(s.chainA.SenderAccount.GetAddress().Bytes())
			contract, ctx := precompiletestutil.NewPrecompileContract(s.T(), ctx, caller, s.chainAPrecompile.Address(), gas)

			args := []interface{}{}
			if tc.arg != nil {
				args = append(args, tc.arg)
			}

			bz, err := s.chainAPrecompile.DenomHash(ctx, contract, &method, args)

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				var out transfertypes.QueryDenomHashResponse
				err = s.chainAPrecompile.UnpackIntoInterface(&out, ics20.DenomHashMethod, bz)
				s.Require().NoError(err)
				s.Require().Equal(tc.expHash, out.Hash)
			}
		})
	}
}
