package distribution

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/distribution"
	"github.com/cosmos/evm/precompiles/testutil"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// TestTxAcceptHexValidatorAddress checks that the distribution tx methods that take a
// validator address accept a 0x-hex address, as the distribution msg server does. The
// chain's validator address codec accepts hex, so the precompile must not require bech32.
func (s *PrecompileTestSuite) TestTxAcceptHexValidatorAddress() {
	hexOf := func(bech32 string) (sdk.ValAddress, string) {
		valAddr, err := sdk.ValAddressFromBech32(bech32)
		s.Require().NoError(err)
		return valAddr, common.BytesToAddress(valAddr).Hex()
	}

	s.Run("withdrawDelegatorRewards", func() {
		s.SetupTest()
		ctx := s.network.GetContext()
		val := s.network.GetValidators()[0]
		_, valHex := hexOf(val.OperatorAddress)

		ctx, err := s.prepareStakingRewards(ctx, stakingRewards{
			Validator: val,
			Delegator: s.keyring.GetAccAddr(0),
			RewardAmt: testRewardsAmt,
		})
		s.Require().NoError(err)

		contract, ctx := testutil.NewPrecompileContract(s.T(), ctx, s.keyring.GetAddr(0), s.precompile.Address(), 200000)
		method := s.precompile.Methods[distribution.WithdrawDelegatorRewardMethod]
		bz, err := s.precompile.WithdrawDelegatorReward(ctx, contract, s.network.GetStateDB(), &method,
			[]interface{}{s.keyring.GetAddr(0), valHex})
		s.Require().NoError(err)

		var coins []cmn.Coin
		s.Require().NoError(s.precompile.UnpackIntoInterface(&coins, distribution.WithdrawDelegatorRewardMethod, bz))
		s.Require().Equal(expRewardsAmt.BigInt(), coins[0].Amount)
	})

	s.Run("depositValidatorRewardsPool", func() {
		s.SetupTest()
		ctx := s.network.GetContext()
		_, valHex := hexOf(s.network.GetValidators()[0].OperatorAddress)

		contract, ctx := testutil.NewPrecompileContract(s.T(), ctx, s.keyring.GetAddr(0), s.precompile.Address(), 200000)
		method := s.precompile.Methods[distribution.DepositValidatorRewardsPoolMethod]
		_, err := s.precompile.DepositValidatorRewardsPool(ctx, contract, s.network.GetStateDB(), &method,
			[]interface{}{s.keyring.GetAddr(0), valHex, []cmn.Coin{{Denom: s.bondDenom, Amount: big.NewInt(1e17)}}})
		s.Require().NoError(err)
	})

	s.Run("withdrawValidatorCommission", func() {
		s.SetupTest()
		ctx := s.network.GetContext()
		valAddr, valHex := hexOf(s.network.GetValidators()[0].OperatorAddress)

		commission := sdk.DecCoins{sdk.NewDecCoin(s.bondDenom, math.NewInt(1e17))}
		distrKeeper := s.network.App.GetDistrKeeper()
		s.Require().NoError(distrKeeper.SetValidatorOutstandingRewards(ctx, valAddr, types.ValidatorOutstandingRewards{Rewards: commission}))
		s.Require().NoError(distrKeeper.SetValidatorAccumulatedCommission(ctx, valAddr, types.ValidatorAccumulatedCommission{Commission: commission}))
		s.Require().NoError(s.mintCoinsForDistrMod(ctx, sdk.NewCoins(sdk.NewCoin(s.bondDenom, math.NewInt(1e17)))))

		contract, ctx := testutil.NewPrecompileContract(s.T(), ctx, common.BytesToAddress(valAddr), s.precompile.Address(), 200000)
		method := s.precompile.Methods[distribution.WithdrawValidatorCommissionMethod]
		bz, err := s.precompile.WithdrawValidatorCommission(ctx, contract, s.network.GetStateDB(), &method, []interface{}{valHex})
		s.Require().NoError(err)

		var coins []cmn.Coin
		s.Require().NoError(s.precompile.UnpackIntoInterface(&coins, distribution.WithdrawValidatorCommissionMethod, bz))
		s.Require().Equal(big.NewInt(1e17), coins[0].Amount)
	})
}
