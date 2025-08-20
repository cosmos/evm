package vm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func (s *KeeperTestSuite) TestBaseFee() {
	testCases := []struct {
		name            string
		EnableLondonHF  bool
		EnableFeemarket bool
		expectBaseFee   *big.Int
	}{
		{"not enable london HF, not enable feemarket", false, false, nil},
		{"enable london HF, not enable feemarket", true, false, big.NewInt(0)},
		{"enable london HF, enable feemarket", true, true, big.NewInt(1000000000)},
		{"not enable london HF, enable feemarket", false, true, nil},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.EnableFeemarket = tc.EnableFeemarket
			s.EnableLondonHF = tc.EnableLondonHF
			s.SetupTest()

			baseFee := s.Network.App.GetEVMKeeper().GetBaseFee(s.Network.GetContext())
			s.Require().Equal(tc.expectBaseFee, baseFee)
		})
	}
	s.EnableFeemarket = false
	s.EnableLondonHF = true
}

func (s *KeeperTestSuite) TestGetAccountStorage() {
	var ctx sdk.Context
	testCases := []struct {
		name     string
		malleate func() common.Address
	}{
		{
			name:     "Only accounts that are not a contract (no storage)",
			malleate: nil,
		},
		{
			name: "One contract (with storage) and other EOAs",
			malleate: func() common.Address {
				supply := big.NewInt(100)
				contractAddr := s.DeployTestContract(s.T(), ctx, s.Keyring.GetAddr(0), supply)
				return contractAddr
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.Network.GetContext()

			var contractAddr common.Address
			if tc.malleate != nil {
				contractAddr = tc.malleate()
			}

			i := 0
			s.Network.App.GetAccountKeeper().IterateAccounts(ctx, func(account sdk.AccountI) bool {
				acc, ok := account.(*authtypes.BaseAccount)
				if !ok {
					// Ignore e.g. module accounts
					return false
				}

				address, err := utils.HexAddressFromBech32String(acc.Address)
				if err != nil {
					// NOTE: we panic in the test to see any potential problems
					// instead of skipping to the next account
					panic(fmt.Sprintf("failed to convert %s to hex address", err))
				}

				storage := s.Network.App.GetEVMKeeper().GetAccountStorage(ctx, address)

				if address == contractAddr || address == ethparams.HistoryStorageAddress {
					s.Require().NotEqual(0, len(storage),
						"expected account %d to have non-zero amount of storage slots, got %d",
						i, len(storage),
					)
				} else {
					s.Require().Len(storage, 0,
						"expected account %d to have %d storage slots, got %d",
						i, 0, len(storage),
					)
				}

				i++
				return false
			})
		})
	}
}

func (s *KeeperTestSuite) TestGetAccountOrEmpty() {
	ctx := s.Network.GetContext()
	empty := statedb.Account{
		Balance:  new(uint256.Int),
		CodeHash: evmtypes.EmptyCodeHash,
	}

	supply := big.NewInt(100)
	contractAddr := s.DeployTestContract(s.T(), ctx, s.Keyring.GetAddr(0), supply)

	testCases := []struct {
		name     string
		addr     common.Address
		expEmpty bool
	}{
		{
			"unexisting account - get empty",
			common.Address{},
			true,
		},
		{
			"existing contract account",
			contractAddr,
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			res := s.Network.App.GetEVMKeeper().GetAccountOrEmpty(ctx, tc.addr)
			if tc.expEmpty {
				s.Require().Equal(empty, res)
			} else {
				s.Require().NotEqual(empty, res)
			}
		})
	}
}
