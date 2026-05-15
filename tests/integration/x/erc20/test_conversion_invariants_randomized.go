package erc20

import (
	"fmt"
	"math/big"
	"math/rand"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/x/erc20/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestRandomizedConvertRoundTripInvariant runs randomized convert sequences and
// asserts that sender coin/token balances always conserve value.
func (s *KeeperTestSuite) TestRandomizedConvertRoundTripInvariant() {
	const (
		initialMint = int64(5_000)
		steps       = 50
	)

	for _, seed := range []int64{1, 7, 42, 777, 2026} {
		s.Run(fmt.Sprintf("seed-%d", seed), func() {
			s.mintFeeCollector = true
			defer func() {
				s.mintFeeCollector = false
			}()
			s.SetupTest()

			contractAddr, err := s.setupRegisterERC20Pair(contractMinterBurner)
			s.Require().NoError(err)

			senderAcc := s.keyring.GetAccAddr(0)
			senderHex := s.keyring.GetAddr(0)
			denom := types.CreateDenom(contractAddr.String())
			total := math.NewInt(initialMint)

			_, err = s.MintERC20Token(contractAddr, senderHex, big.NewInt(initialMint))
			s.Require().NoError(err)

			rng := rand.New(rand.NewSource(seed))
			erc20Keeper := s.network.App.GetErc20Keeper()
			bankKeeper := s.network.App.GetBankKeeper()
			erc20ABI := contracts.ERC20MinterBurnerDecimalsContract.ABI

			for i := 0; i < steps; i++ {
				ctx := s.network.GetContext()
				erc20Bal := erc20Keeper.BalanceOf(ctx, erc20ABI, contractAddr, senderHex)
				coinBal := bankKeeper.GetBalance(ctx, senderAcc, denom).Amount

				combined := new(big.Int).Add(erc20Bal, coinBal.BigInt())
				s.Require().Equal(total.String(), combined.String(), "value should be conserved before step")

				convertFromERC20 := rng.Intn(2) == 0
				if erc20Bal.Sign() == 0 {
					convertFromERC20 = false
				}
				if coinBal.IsZero() {
					convertFromERC20 = true
				}

				if convertFromERC20 {
					maxAmount := erc20Bal.Int64()
					amount := int64(rng.Intn(int(maxAmount)) + 1)
					_, err = erc20Keeper.ConvertERC20(
						ctx,
						types.NewMsgConvertERC20(math.NewInt(amount), senderAcc, contractAddr, senderHex),
					)
					s.Require().NoError(err)
				} else {
					maxAmount := coinBal.Int64()
					amount := int64(rng.Intn(int(maxAmount)) + 1)
					_, err = erc20Keeper.ConvertCoin(
						ctx,
						types.NewMsgConvertCoin(sdk.NewCoin(denom, math.NewInt(amount)), senderHex, senderAcc),
					)
					s.Require().NoError(err)
				}
			}

			ctx := s.network.GetContext()
			finalERC20Bal := erc20Keeper.BalanceOf(ctx, erc20ABI, contractAddr, senderHex)
			finalCoinBal := bankKeeper.GetBalance(ctx, senderAcc, denom).Amount
			finalCombined := new(big.Int).Add(finalERC20Bal, finalCoinBal.BigInt())
			s.Require().Equal(total.String(), finalCombined.String(), "value should be conserved after sequence")
		})
	}
}
