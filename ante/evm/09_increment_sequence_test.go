package evm_test

import (
	"github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	testkeyring "github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

func (suite *EvmAnteTestSuite) TestIncrementSequence() {
	keyring := testkeyring.New(1)
	unitNetwork := network.NewUnitTestNetwork(
		network.WithChainID(suite.chainID),
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	)
	grpcHandler := grpc.NewIntegrationHandler(unitNetwork)
	accAddr := keyring.GetAccAddr(0)

	testCases := []struct {
		name          string
		expectedError error
		malleate      func(acct sdktypes.AccountI) uint64
	}{
		{
			name:          "fail: invalid sequence",
			expectedError: errortypes.ErrInvalidSequence,
			malleate: func(acct sdktypes.AccountI) uint64 {
				return acct.GetSequence() + 1
			},
		},
		{
			name:          "success: increments sequence",
			expectedError: nil,
			malleate: func(acct sdktypes.AccountI) uint64 {
				return acct.GetSequence()
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			account, err := grpcHandler.GetAccount(accAddr.String())
			suite.Require().NoError(err)
			preSequence := account.GetSequence()

			nonce := tc.malleate(account)

			// Function under test
			err = evm.IncrementNonce(
				unitNetwork.GetContext(),
				unitNetwork.App.AccountKeeper,
				account,
				nonce,
			)

			if tc.expectedError != nil {
				suite.Require().Error(err)
				suite.Contains(err.Error(), tc.expectedError.Error())
			} else {
				suite.Require().NoError(err)

				suite.Require().Equal(preSequence+1, account.GetSequence())
				updatedAccount, err := grpcHandler.GetAccount(accAddr.String())
				suite.Require().NoError(err)
				suite.Require().Equal(preSequence+1, updatedAccount.GetSequence())
			}
		})
	}
}
