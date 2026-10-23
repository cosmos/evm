package staking

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/staking"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// TestEventsAcceptHexValidatorAddress checks that every staking tx method that the
// SDK msg server accepts with a 0x-hex validator address also succeeds in the
// precompile and logs the matching validator topic. The chain's validator address
// codec accepts hex, so the precompile must not re-parse the address as bech32.
func (s *PrecompileTestSuite) TestEventsAcceptHexValidatorAddress() {
	s.SetupTest()
	ctx := s.network.GetContext()
	stDB := s.network.GetStateDB()
	delegator := s.keyring.GetKey(0)
	contract := vm.NewContract(delegator.Addr, s.precompile.Address(), uint256.NewInt(0), 20000, nil)

	hexOf := func(bech32 string) (string, common.Address) {
		valAddr, err := sdk.ValAddressFromBech32(bech32)
		s.Require().NoError(err)
		addr := common.BytesToAddress(valAddr)
		return addr.Hex(), addr
	}
	val0Hex, val0Addr := hexOf(s.network.GetValidators()[0].OperatorAddress)
	val1Hex, val1Addr := hexOf(s.network.GetValidators()[1].OperatorAddress)
	amount := big.NewInt(1e17)

	method := s.precompile.Methods[staking.DelegateMethod]
	_, err := s.precompile.Delegate(ctx, contract, stDB, &method, []interface{}{delegator.Addr, val0Hex, amount})
	s.Require().NoError(err, "delegate with a hex validator address")
	var delegateEvent staking.EventDelegate
	s.Require().NoError(cmn.UnpackLog(s.precompile.ABI, &delegateEvent, staking.EventTypeDelegate, *stDB.Logs()[0]))
	s.Require().Equal(val0Addr, delegateEvent.ValidatorAddress)

	method = s.precompile.Methods[staking.UndelegateMethod]
	_, err = s.precompile.Undelegate(ctx, contract, stDB, &method, []interface{}{delegator.Addr, val0Hex, amount})
	s.Require().NoError(err, "undelegate with a hex validator address")
	var unbondEvent staking.EventUnbond
	s.Require().NoError(cmn.UnpackLog(s.precompile.ABI, &unbondEvent, staking.EventTypeUnbond, *stDB.Logs()[1]))
	s.Require().Equal(val0Addr, unbondEvent.ValidatorAddress)

	method = s.precompile.Methods[staking.CancelUnbondingDelegationMethod]
	_, err = s.precompile.CancelUnbondingDelegation(ctx, contract, stDB, &method,
		[]interface{}{delegator.Addr, val0Hex, amount, big.NewInt(ctx.BlockHeight())})
	s.Require().NoError(err, "cancelUnbondingDelegation with a hex validator address")
	var cancelEvent staking.EventCancelUnbonding
	s.Require().NoError(cmn.UnpackLog(s.precompile.ABI, &cancelEvent, staking.EventTypeCancelUnbondingDelegation, *stDB.Logs()[2]))
	s.Require().Equal(val0Addr, cancelEvent.ValidatorAddress)

	method = s.precompile.Methods[staking.RedelegateMethod]
	_, err = s.precompile.Redelegate(ctx, contract, stDB, &method, []interface{}{delegator.Addr, val0Hex, val1Hex, amount})
	s.Require().NoError(err, "redelegate with hex validator addresses")
	var redelegateEvent staking.EventRedelegate
	s.Require().NoError(cmn.UnpackLog(s.precompile.ABI, &redelegateEvent, staking.EventTypeRedelegate, *stDB.Logs()[3]))
	s.Require().Equal(val0Addr, redelegateEvent.ValidatorSrcAddress)
	s.Require().Equal(val1Addr, redelegateEvent.ValidatorDstAddress)
}

// slashValidator slashes the validator at index idx by fraction, so its exchange rate
// is no longer 1 and token/share conversions truncate.
func (s *PrecompileTestSuite) slashValidator(ctx sdk.Context, idx int, fraction math.LegacyDec) sdk.ValAddress {
	sk := s.network.App.GetStakingKeeper()
	valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[idx].OperatorAddress)
	s.Require().NoError(err)
	validator, err := sk.GetValidator(ctx, valAddr)
	s.Require().NoError(err)
	consAddr, err := validator.GetConsAddr()
	s.Require().NoError(err)
	power := validator.GetConsensusPower(sk.PowerReduction(ctx))
	_, err = sk.Slash(ctx, consAddr, ctx.BlockHeight(), power, fraction)
	s.Require().NoError(err)
	return valAddr
}

// TestUnbondEventReportsUndelegatedAmount checks that the Unbond log reports the amount
// the SDK actually moved into the unbonding entry, not the requested amount. The two
// differ when the validator's exchange rate makes share conversion truncate.
func (s *PrecompileTestSuite) TestUnbondEventReportsUndelegatedAmount() {
	s.SetupTest()
	ctx := s.network.GetContext()
	stDB := s.network.GetStateDB()
	delegator := s.keyring.GetKey(0)
	valAddr := s.slashValidator(ctx, 0, math.LegacyNewDecWithPrec(1, 1))

	requested := big.NewInt(1e17)
	contract := vm.NewContract(delegator.Addr, s.precompile.Address(), uint256.NewInt(0), 20000, nil)
	method := s.precompile.Methods[staking.UndelegateMethod]
	_, err := s.precompile.Undelegate(ctx, contract, stDB, &method,
		[]interface{}{delegator.Addr, s.network.GetValidators()[0].OperatorAddress, requested})
	s.Require().NoError(err)

	ubd, err := s.network.App.GetStakingKeeper().GetUnbondingDelegation(ctx, delegator.AccAddr, valAddr)
	s.Require().NoError(err)
	s.Require().Len(ubd.Entries, 1)
	undelegated := ubd.Entries[0].Balance.BigInt()
	s.Require().NotEqual(requested, undelegated, "test setup must make the requested and undelegated amounts differ")

	var unbondEvent staking.EventUnbond
	s.Require().NoError(cmn.UnpackLog(s.precompile.ABI, &unbondEvent, staking.EventTypeUnbond, *stDB.Logs()[0]))
	s.Require().Equal(undelegated, unbondEvent.Amount)
}

// TestDelegateEventReportsIssuedShares checks that the Delegate log's newShares equals
// the shares the SDK actually added to the delegation when the validator's exchange
// rate is not 1.
func (s *PrecompileTestSuite) TestDelegateEventReportsIssuedShares() {
	cases := []struct {
		fraction math.LegacyDec
		amount   *big.Int
	}{
		{math.LegacyNewDecWithPrec(1, 1), big.NewInt(1e17)},
		{math.LegacyNewDecWithPrec(333333333333333333, 18), big.NewInt(123456789012345679)},
		{math.LegacyNewDecWithPrec(7, 2), big.NewInt(3)},
		{math.LegacyNewDecWithPrec(999, 3), big.NewInt(1)},
		{math.LegacyNewDecWithPrec(5, 1), big.NewInt(999999999999999999)},
	}
	for _, tc := range cases {
		s.Run(tc.fraction.String()+"/"+tc.amount.String(), func() {
			s.SetupTest()
			ctx := s.network.GetContext()
			stDB := s.network.GetStateDB()
			delegator := s.keyring.GetKey(0)
			sk := s.network.App.GetStakingKeeper()
			valAddr := s.slashValidator(ctx, 0, tc.fraction)

			before, err := sk.GetDelegation(ctx, delegator.AccAddr, valAddr)
			s.Require().NoError(err)

			contract := vm.NewContract(delegator.Addr, s.precompile.Address(), uint256.NewInt(0), 20000, nil)
			method := s.precompile.Methods[staking.DelegateMethod]
			_, err = s.precompile.Delegate(ctx, contract, stDB, &method,
				[]interface{}{delegator.Addr, s.network.GetValidators()[0].OperatorAddress, tc.amount})
			s.Require().NoError(err)

			after, err := sk.GetDelegation(ctx, delegator.AccAddr, valAddr)
			s.Require().NoError(err)
			issued := after.Shares.Sub(before.Shares)

			var delegateEvent staking.EventDelegate
			s.Require().NoError(cmn.UnpackLog(s.precompile.ABI, &delegateEvent, staking.EventTypeDelegate, *stDB.Logs()[0]))
			s.Require().Equal(issued.BigInt(), delegateEvent.NewShares, "issued shares %s", issued)
		})
	}
}

// TestRedelegateFromUnbondedValidator checks the completion time reported when the
// source validator is Unbonded. The SDK completes such a redelegation immediately and
// returns the zero time.Time; the precompile reports that as 0 in both the return
// value and the Redelegate log, instead of a negative Unix time that the log's uint256
// field would encode as 2^256 - 62135596800.
func (s *PrecompileTestSuite) TestRedelegateFromUnbondedValidator() {
	s.SetupTest()
	ctx := s.network.GetContext()
	stDB := s.network.GetStateDB()
	delegator := s.keyring.GetKey(0)
	operator := s.keyring.GetKey(1)
	sk := s.network.App.GetStakingKeeper()
	msgServer := stakingkeeper.NewMsgServerImpl(sk)

	// A newly created validator stays Unbonded until the end of the block.
	srcValAddr := sdk.ValAddress(operator.AccAddr)
	createMsg, err := stakingtypes.NewMsgCreateValidator(
		srcValAddr.String(),
		ed25519.GenPrivKey().PubKey(),
		sdk.NewCoin(s.bondDenom, math.NewInt(1e18)),
		stakingtypes.NewDescription("unbonded", "", "", "", ""),
		stakingtypes.NewCommissionRates(math.LegacyNewDecWithPrec(1, 1), math.LegacyNewDecWithPrec(2, 1), math.LegacyNewDecWithPrec(1, 2)),
		math.OneInt(),
	)
	s.Require().NoError(err)
	_, err = msgServer.CreateValidator(ctx, createMsg)
	s.Require().NoError(err)

	amount := big.NewInt(1e17)
	_, err = msgServer.Delegate(ctx, stakingtypes.NewMsgDelegate(
		delegator.AccAddr.String(), srcValAddr.String(), sdk.NewCoin(s.bondDenom, math.NewIntFromBigInt(amount))))
	s.Require().NoError(err)

	srcVal, err := sk.GetValidator(ctx, srcValAddr)
	s.Require().NoError(err)
	s.Require().True(srcVal.IsUnbonded(), "source validator must be Unbonded")

	contract := vm.NewContract(delegator.Addr, s.precompile.Address(), uint256.NewInt(0), 20000, nil)
	method := s.precompile.Methods[staking.RedelegateMethod]
	bz, err := s.precompile.Redelegate(ctx, contract, stDB, &method,
		[]interface{}{delegator.Addr, srcValAddr.String(), s.network.GetValidators()[0].OperatorAddress, amount})
	s.Require().NoError(err)

	out, err := method.Outputs.Unpack(bz)
	s.Require().NoError(err)
	s.Require().Equal(int64(0), out[0].(int64), "return value completionTime")

	var redelegateEvent staking.EventRedelegate
	s.Require().NoError(cmn.UnpackLog(s.precompile.ABI, &redelegateEvent, staking.EventTypeRedelegate, *stDB.Logs()[0]))
	s.Require().Zero(redelegateEvent.CompletionTime.Sign(), "log completionTime %s", redelegateEvent.CompletionTime)
}
