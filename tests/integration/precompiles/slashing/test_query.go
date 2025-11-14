package slashing

import (
	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/slashing"

	"github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

func (s *PrecompileTestSuite) TestGetSigningInfo() {
	valSigners := s.network.GetValidators()
	val0ConsAddr, _ := valSigners[0].GetConsAddr()

	consAddr := types.ConsAddress(val0ConsAddr)
	testCases := []struct {
		name        string
		malleate    func() slashing.GetSigningInfoCall
		postCheck   func(signingInfo *slashing.SigningInfo)
		expError    bool
		errContains string
	}{
		{
			"fail - invalid consensus address",
			func() slashing.GetSigningInfoCall {
				return slashing.GetSigningInfoCall{
					ConsAddress: common.Address{},
				}
			},
			func(_ *slashing.SigningInfo) {},
			true,
			"invalid consensus address",
		},
		{
			"success - get signing info for validator",
			func() slashing.GetSigningInfoCall {
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
				return slashing.GetSigningInfoCall{
					ConsAddress: common.BytesToAddress(consAddr.Bytes()),
				}
			},
			func(signingInfo *slashing.SigningInfo) {
				s.Require().Equal(consAddr.Bytes(), signingInfo.ValidatorAddress.Bytes())
				s.Require().Equal(int64(1), signingInfo.StartHeight)
				s.Require().Equal(int64(2), signingInfo.IndexOffset)
				s.Require().Equal(int64(1), signingInfo.MissedBlocksCounter)
				s.Require().False(signingInfo.Tombstoned)
			},
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			ctx := s.network.GetContext()
			call := tc.malleate()
			result, err := s.precompile.GetSigningInfo(ctx, call)

			if tc.expError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(result)
				tc.postCheck(&result.SigningInfo)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestGetSigningInfos() {
	testCases := []struct {
		name        string
		malleate    func() slashing.GetSigningInfosCall
		postCheck   func(signingInfos []slashing.SigningInfo, pageResponse cmn.PageResponse)
		expError    bool
		errContains string
	}{
		{
			"success - get all signing infos",
			func() slashing.GetSigningInfosCall {
				return slashing.GetSigningInfosCall{
					Pagination: cmn.PageRequest{
						Limit:      10,
						CountTotal: true,
					},
				}
			},
			func(signingInfos []slashing.SigningInfo, pageResponse cmn.PageResponse) {
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
			false,
			"",
		},
		{
			"success - get signing infos with pagination",
			func() slashing.GetSigningInfosCall {
				return slashing.GetSigningInfosCall{
					Pagination: cmn.PageRequest{
						Limit:      1,
						CountTotal: true,
					},
				}
			},
			func(signingInfos []slashing.SigningInfo, pageResponse cmn.PageResponse) {
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
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			ctx := s.network.GetContext()
			call := tc.malleate()
			result, err := s.precompile.GetSigningInfos(ctx, call)

			if tc.expError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(result)
				tc.postCheck(result.SigningInfos, result.PageResponse)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestGetParams() {
	testCases := []struct {
		name        string
		malleate    func() slashing.GetParamsCall
		postCheck   func(params *slashing.Params)
		expError    bool
		errContains string
	}{
		{
			"success - get params",
			func() slashing.GetParamsCall {
				return slashing.GetParamsCall{}
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
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			ctx := s.network.GetContext()
			call := tc.malleate()
			result, err := s.precompile.GetParams(ctx, call)

			if tc.expError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(result)
				tc.postCheck(&result.Params)
			}
		})
	}
}
