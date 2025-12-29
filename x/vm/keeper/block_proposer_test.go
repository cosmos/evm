package keeper_test

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/mock"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (suite *KeeperTestSuite) TestGetCoinbaseAddress() {
	proposerConsAddr := sdk.ConsAddress([]byte("proposer"))
	validatorOperator := "cosmosvaloper1abcdefghijklmnopqrstuvwxyz"

	testCases := []struct {
		name         string
		proposerAddr sdk.ConsAddress
		malleate     func()
		expectedAddr common.Address
		expectedErr  bool
		errContains  string
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
			expectedAddr: common.BytesToAddress([]byte(validatorOperator)),
			expectedErr:  false,
		},
		{
			name:         "success - consumer chain (zero bonded tokens)",
			proposerAddr: proposerConsAddr,
			malleate: func() {
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound).Once()
				suite.stakingKeeper.On("TotalValidatorPower", mock.Anything).
					Return(math.ZeroInt(), nil).Once()
			},
			expectedAddr: common.BytesToAddress(proposerConsAddr.Bytes()),
			expectedErr:  false,
		},
		{
			name:         "empty proposer address returns empty address",
			proposerAddr: sdk.ConsAddress{},
			malleate:     func() {},
			expectedAddr: common.Address{},
			expectedErr:  false,
		},
		{
			name:         "error - standalone chain with validator not found",
			proposerAddr: proposerConsAddr,
			malleate: func() {
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound).Once()
				suite.stakingKeeper.On("TotalValidatorPower", mock.Anything).
					Return(math.NewInt(1000000), nil).Once()
			},
			expectedAddr: common.Address{},
			expectedErr:  true,
			errContains:  "failed to retrieve validator from block proposer address",
		},
		{
			name:         "error - failed to retrieve total validator power",
			proposerAddr: proposerConsAddr,
			malleate: func() {
				suite.stakingKeeper.On("GetValidatorByConsAddr", mock.Anything, proposerConsAddr).
					Return(stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound).Once()
				suite.stakingKeeper.On("TotalValidatorPower", mock.Anything).
					Return(math.Int{}, errors.New("failed to get total power")).Once()
			},
			expectedAddr: common.Address{},
			expectedErr:  true,
			errContains:  "failed to retrieve bonded pool balance",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.malleate()
			addr, err := suite.vmKeeper.GetCoinbaseAddress(suite.ctx, tc.proposerAddr)
			if tc.expectedErr {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.errContains)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(tc.expectedAddr, addr)
			}
		})
	}
}
