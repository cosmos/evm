package keeper_test

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/mock"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (suite *KeeperTestSuite) TestGetCoinbaseAddress() {
	proposerConsAddr := sdk.ConsAddress([]byte("proposer"))
	valAddr := sdk.ValAddress([]byte("test_validator_addr"))
	validatorOperator := valAddr.String()

	testCases := []struct {
		name         string
		proposerAddr sdk.ConsAddress
		malleate     func()
		expectedAddr common.Address
		expectedErr  bool
	}{
		{
			name:         "success - validator found",
			proposerAddr: proposerConsAddr,
			malleate: func() {
				validator := stakingtypes.Validator{
					OperatorAddress: validatorOperator,
				}
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(validator, nil).Once()
			},
			expectedAddr: common.BytesToAddress(valAddr.Bytes()),
			expectedErr:  false,
		},
		{
			name:         "validator not found returns error",
			proposerAddr: proposerConsAddr,
			malleate: func() {
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound).Once()
			},
			expectedAddr: common.Address{},
			expectedErr:  true,
		},
		{
			name:         "empty proposer address returns empty address",
			proposerAddr: sdk.ConsAddress{},
			malleate:     func() {},
			expectedAddr: common.Address{},
			expectedErr:  false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.malleate()
			addr, err := suite.vmKeeper.GetCoinbaseAddress(suite.ctx, tc.proposerAddr)
			if tc.expectedErr {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(tc.expectedAddr, addr)
			}
		})
	}
}
