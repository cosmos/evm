package erc20

import (
	"fmt"
	"math/big"
	"math/rand"
	"strings"

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

			assertInvariants := func(phase string) {
				ctx := s.network.GetContext()
				erc20Bal := erc20Keeper.BalanceOf(ctx, erc20ABI, contractAddr, senderHex)
				coinBal := bankKeeper.GetBalance(ctx, senderAcc, denom).Amount

				combined := new(big.Int).Add(erc20Bal, coinBal.BigInt())
				s.Require().Equal(total.String(), combined.String(), "value should be conserved at %s", phase)

				for _, bal := range bankKeeper.GetAllBalances(ctx, senderAcc) {
					if strings.HasPrefix(bal.Denom, types.Erc20NativeCoinDenomPrefix) {
						s.Require().Equal(denom, bal.Denom, "unexpected erc20 native denom at %s", phase)
					}
				}
			}

			// Deterministic edge sequence before random steps:
			// smallest amount in/out, then full-balance in/out.
			assertInvariants("initial")

			_, err = erc20Keeper.ConvertERC20(
				s.network.GetContext(),
				types.NewMsgConvertERC20(math.NewInt(1), senderAcc, contractAddr, senderHex),
			)
			s.Require().NoError(err)
			assertInvariants("after convertERC20(1)")

			_, err = erc20Keeper.ConvertCoin(
				s.network.GetContext(),
				types.NewMsgConvertCoin(sdk.NewCoin(denom, math.NewInt(1)), senderHex, senderAcc),
			)
			s.Require().NoError(err)
			assertInvariants("after convertCoin(1)")

			ctx := s.network.GetContext()
			allERC20 := erc20Keeper.BalanceOf(ctx, erc20ABI, contractAddr, senderHex)
			if allERC20.Sign() > 0 {
				_, err = erc20Keeper.ConvertERC20(
					ctx,
					types.NewMsgConvertERC20(math.NewIntFromBigInt(allERC20), senderAcc, contractAddr, senderHex),
				)
				s.Require().NoError(err)
				assertInvariants("after full convertERC20")
			}

			allCoin := bankKeeper.GetBalance(s.network.GetContext(), senderAcc, denom).Amount
			if !allCoin.IsZero() {
				_, err = erc20Keeper.ConvertCoin(
					s.network.GetContext(),
					types.NewMsgConvertCoin(sdk.NewCoin(denom, allCoin), senderHex, senderAcc),
				)
				s.Require().NoError(err)
				assertInvariants("after full convertCoin")
			}

			for i := 0; i < steps; i++ {
				ctx := s.network.GetContext()
				erc20Bal := erc20Keeper.BalanceOf(ctx, erc20ABI, contractAddr, senderHex)
				coinBal := bankKeeper.GetBalance(ctx, senderAcc, denom).Amount

				assertInvariants(fmt.Sprintf("before randomized step %d", i))

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
				assertInvariants(fmt.Sprintf("after randomized step %d", i))
			}

			assertInvariants("final")
		})
	}
}
