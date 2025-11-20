package ante

import (
	"github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/mempool"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
)

func (s *EvmUnitAnteTestSuite) TestIncrementSequence() {
	keyring := testkeyring.New(1)
	unitNetwork := network.NewUnitTestNetwork(
		s.create,
		network.WithChainID(testconstants.ChainID{
			ChainID:    s.ChainID,
			EVMChainID: s.EvmChainID,
		}),
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
			name:          "fail: nonce gap",
			expectedError: mempool.ErrNonceGap,
			malleate: func(acct sdktypes.AccountI) uint64 {
				return acct.GetSequence() + 1
			},
		},
		{
			name:          "fail: nonce is low",
			expectedError: mempool.ErrNonceLow,
			malleate: func(acct sdktypes.AccountI) uint64 {
				err := acct.SetSequence(acct.GetSequence() + 1)
				s.Require().NoError(err)
				unitNetwork.App.GetAccountKeeper().SetAccount(unitNetwork.GetContext(), acct)
				return acct.GetSequence() - 1
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
		s.Run(tc.name, func() {
			account, err := grpcHandler.GetAccount(accAddr.String())
			s.Require().NoError(err)
			preSequence := account.GetSequence()

			nonce := tc.malleate(account)

			// Function under test
			err = evm.IncrementNonce(
				unitNetwork.GetContext(),
				unitNetwork.App.GetAccountKeeper(),
				account,
				nonce,
			)

			if tc.expectedError != nil {
				s.Require().Error(err)
				s.Contains(err.Error(), tc.expectedError.Error())
			} else {
				s.Require().NoError(err)

				s.Require().Equal(preSequence+1, account.GetSequence())
				updatedAccount, err := grpcHandler.GetAccount(accAddr.String())
				s.Require().NoError(err)
				s.Require().Equal(preSequence+1, updatedAccount.GetSequence())
			}
		})
	}
}
