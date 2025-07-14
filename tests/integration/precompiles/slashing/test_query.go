package slashing

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/slashing"
	"github.com/cosmos/evm/precompiles/testutil"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

func (s *PrecompileTestSuite) TestGetSigningInfo() {
	method := s.precompile.Methods[slashing.GetSigningInfoMethod]

	valSigners := s.network.GetValidators()
	val0ConsAddr, _ := valSigners[0].GetConsAddr()

	consAddr := types.ConsAddress(val0ConsAddr)
	testCases := []struct {
		name        string
		malleate    func() []interface{}
		postCheck   func(signingInfo *slashing.SigningInfo)
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"fail - empty input args",
			func() []interface{} {
				return []interface{}{}
			},
			func(_ *slashing.SigningInfo) {},
			200000,
			true,
			fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 0),
		},
		{
			"fail - invalid consensus address",
			func() []interface{} {
				return []interface{}{
					common.Address{},
				}
			},
			func(_ *slashing.SigningInfo) {},
			200000,
			true,
			"invalid consensus address",
		},
		{
			"success - get signing info for validator",
			func() []interface{} {
				err := s.network.App.GetSlashingKeeper().SetValidatorSigningInfo(
					s.network.GetContext(),
					consAddr,
					slashingtypes.ValidatorSigningInfo{
						Address:             consAddr.String(),
						StartHeight:         1,
						IndexOffset:         2,
						MissedBlocksCounter: 1,
						Tombstoned:          false,
					},
				)
				s.Require().NoError(err)
				return []interface{}{
					common.BytesToAddress(consAddr.Bytes()),
				}
			},
			func(signingInfo *slashing.SigningInfo) {
				s.Require().Equal(consAddr.Bytes(), signingInfo.ValidatorAddress.Bytes())
				s.Require().Equal(int64(1), signingInfo.StartHeight)
				s.Require().Equal(int64(2), signingInfo.IndexOffset)
				s.Require().Equal(int64(1), signingInfo.MissedBlocksCounter)
				s.Require().False(signingInfo.Tombstoned)
			},
			200000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			contract, ctx := testutil.NewPrecompileContract(s.T(), s.network.GetContext(), s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

			bz, err := s.precompile.GetSigningInfo(ctx, &method, contract, tc.malleate())

			if tc.expError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				var out slashing.SigningInfoOutput
				err = s.precompile.UnpackIntoInterface(&out, slashing.GetSigningInfoMethod, bz)
				s.Require().NoError(err)
				tc.postCheck(&out.SigningInfo)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestGetSigningInfos() {
	method := s.precompile.Methods[slashing.GetSigningInfosMethod]

	testCases := []struct {
		name        string
		malleate    func() []interface{}
		postCheck   func(signingInfos []slashing.SigningInfo, pageResponse *query.PageResponse)
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"fail - empty input args",
			func() []interface{} {
				return []interface{}{}
			},
			func(_ []slashing.SigningInfo, _ *query.PageResponse) {},
			200000,
			true,
			fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 0),
		},
		{
			"success - get all signing infos",
			func() []interface{} {
				return []interface{}{
					query.PageRequest{
						Limit:      10,
						CountTotal: true,
					},
				}
			},
			func(signingInfos []slashing.SigningInfo, pageResponse *query.PageResponse) {
				s.Require().Len(signingInfos, 3)
				s.Require().Equal(uint64(3), pageResponse.Total)

				valSigners := s.network.GetValidators()
				val0ConsAddr, _ := valSigners[0].GetConsAddr()
				val1ConsAddr, _ := valSigners[1].GetConsAddr()
				val2ConsAddr, _ := valSigners[2].GetConsAddr()
				// Check first validator's signing info
				s.Require().Equal(val0ConsAddr, signingInfos[0].ValidatorAddress.Bytes())
				s.Require().Equal(int64(0), signingInfos[0].StartHeight)
				s.Require().Equal(int64(1), signingInfos[0].IndexOffset)
				s.Require().Equal(int64(0), signingInfos[0].JailedUntil)
				s.Require().False(signingInfos[0].Tombstoned)

				// Check second validator's signing info
				s.Require().Equal(val1ConsAddr, signingInfos[1].ValidatorAddress.Bytes())
				s.Require().Equal(int64(0), signingInfos[1].StartHeight)
				s.Require().Equal(int64(1), signingInfos[1].IndexOffset)
				s.Require().Equal(int64(0), signingInfos[1].JailedUntil)
				s.Require().False(signingInfos[1].Tombstoned)

				// Check third validator's signing info
				s.Require().Equal(val2ConsAddr, signingInfos[2].ValidatorAddress.Bytes())
				s.Require().Equal(int64(0), signingInfos[2].StartHeight)
				s.Require().Equal(int64(1), signingInfos[2].IndexOffset)
				s.Require().Equal(int64(0), signingInfos[2].JailedUntil)
				s.Require().False(signingInfos[2].Tombstoned)
			},
			200000,
			false,
			"",
		},
		{
			"success - get signing infos with pagination",
			func() []interface{} {
				return []interface{}{
					query.PageRequest{
						Limit:      1,
						CountTotal: true,
					},
				}
			},
			func(signingInfos []slashing.SigningInfo, pageResponse *query.PageResponse) {
				s.Require().Len(signingInfos, 1)
				s.Require().Equal(uint64(3), pageResponse.Total)
				s.Require().NotNil(pageResponse.NextKey)

				// Check first validator's signing info
				valSigners := s.network.GetValidators()
				val0ConsAddr, _ := valSigners[0].GetConsAddr()
				s.Require().Equal(val0ConsAddr, signingInfos[0].ValidatorAddress.Bytes())
				s.Require().Equal(int64(0), signingInfos[0].StartHeight)
				s.Require().Equal(int64(1), signingInfos[0].IndexOffset)
				s.Require().Equal(int64(0), signingInfos[0].JailedUntil)
				s.Require().False(signingInfos[0].Tombstoned)
			},
			200000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			contract, ctx := testutil.NewPrecompileContract(s.T(), s.network.GetContext(), s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

			bz, err := s.precompile.GetSigningInfos(ctx, &method, contract, tc.malleate())

			if tc.expError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				var out slashing.SigningInfosOutput
				err = s.precompile.UnpackIntoInterface(&out, slashing.GetSigningInfosMethod, bz)
				s.Require().NoError(err)
				tc.postCheck(out.SigningInfos, &out.PageResponse)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestGetParams() {
	method := s.precompile.Methods[slashing.GetParamsMethod]

	testCases := []struct {
		name        string
		malleate    func() []interface{}
		postCheck   func(params *slashing.Params)
		gas         uint64
		expError    bool
		errContains string
	}{
		{
			"success - get params",
			func() []interface{} {
				return []interface{}{}
			},
			func(params *slashing.Params) {
				// Get the default params from the network
				defaultParams, err := s.network.App.GetSlashingKeeper().GetParams(s.network.GetContext())
				s.Require().NoError(err)
				s.Require().Equal(defaultParams.SignedBlocksWindow, params.SignedBlocksWindow)
				s.Require().Equal(defaultParams.MinSignedPerWindow.BigInt(), params.MinSignedPerWindow.Value)
				s.Require().Equal(int64(defaultParams.DowntimeJailDuration.Seconds()), params.DowntimeJailDuration)
				s.Require().Equal(defaultParams.SlashFractionDoubleSign.BigInt(), params.SlashFractionDoubleSign.Value)
				s.Require().Equal(defaultParams.SlashFractionDowntime.BigInt(), params.SlashFractionDowntime.Value)
			},
			200000,
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			contract, ctx := testutil.NewPrecompileContract(s.T(), s.network.GetContext(), s.keyring.GetAddr(0), s.precompile.Address(), tc.gas)

			bz, err := s.precompile.GetParams(ctx, &method, contract, tc.malleate())

			if tc.expError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				var out slashing.ParamsOutput
				err = s.precompile.UnpackIntoInterface(&out, slashing.GetParamsMethod, bz)
				s.Require().NoError(err)
				tc.postCheck(&out.Params)
			}
		})
	}
}
