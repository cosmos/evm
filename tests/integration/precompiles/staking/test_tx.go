package staking

import (
	"encoding/base64"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/yihuang/go-abi"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/staking"
	"github.com/cosmos/evm/precompiles/testutil"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	cosmosevmutiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/vm/statedb"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestCreateValidator() {
	var (
		stDB        *statedb.StateDB
		description = staking.Description{
			Moniker:         "node0",
			Identity:        "",
			Website:         "",
			SecurityContact: "",
			Details:         "",
		}
		commission = staking.CommissionRates{
			Rate:          big.NewInt(5e16), // 5%
			MaxRate:       big.NewInt(2e17), // 20%
			MaxChangeRate: big.NewInt(5e16), // 5%
		}
		minSelfDelegation = big.NewInt(1)
		pubkey            = "nfJ0axJC9dhta1MAE1EBFaVdxxkYzxYrBaHuJVjG//M="
		validatorAddress  common.Address
		value             = big.NewInt(1205000000000000000)
		diffAddr, _       = cosmosevmutiltx.NewAddrKey()
	)

	testCases := []struct {
		name          string
		malleate      func() *staking.CreateValidatorCall
		gas           uint64
		callerAddress *common.Address
		postCheck     func(data []byte)
		expError      bool
		errContains   string
	}{
		{
			"fail - different origin than delegator",
			func() *staking.CreateValidatorCall {
				differentAddr := cosmosevmutiltx.GenerateAddress()
				return staking.NewCreateValidatorCall(
					description,
					commission,
					minSelfDelegation,
					differentAddr,
					pubkey,
					value,
				)
			},
			200000,
			nil,
			func([]byte) {},
			true,
			"does not match the requester address",
		},
		{
			"fail - pubkey decoding error",
			func() *staking.CreateValidatorCall {
				return staking.NewCreateValidatorCall(
					description,
					commission,
					minSelfDelegation,
					validatorAddress,
					"bHVrZQ=", // base64.StdEncoding.DecodeString error
					value,
				)
			},
			200000,
			nil,
			func([]byte) {},
			true,
			"illegal base64 data",
		},
		{
			"fail - consensus pubkey len is invalid",
			func() *staking.CreateValidatorCall {
				return staking.NewCreateValidatorCall(
					description,
					commission,
					minSelfDelegation,
					validatorAddress,
					"bHVrZQ==",
					value,
				)
			},
			200000,
			nil,
			func([]byte) {},
			true,
			"consensus pubkey len is invalid",
		},
		{
			"fail - cannot be called from address != than validator address",
			func() *staking.CreateValidatorCall {
				return staking.NewCreateValidatorCall(
					description,
					commission,
					minSelfDelegation,
					validatorAddress,
					pubkey,
					value,
				)
			},
			200000,
			&diffAddr,
			func([]byte) {},
			true,
			"does not match the requester address",
		},
		{
			"fail - cannot be called from account with code (if it is not EIP-7702 delegated account)",
			func() *staking.CreateValidatorCall {
				stDB.SetCode(validatorAddress, []byte{0x60, 0x00})
				return staking.NewCreateValidatorCall(
					description,
					commission,
					minSelfDelegation,
					validatorAddress,
					pubkey,
					value,
				)
			},
			200000,
			nil,
			func([]byte) {},
			true,
			staking.ErrCannotCallFromContract,
		},
		{
			"success",
			func() *staking.CreateValidatorCall {
				return staking.NewCreateValidatorCall(
					description,
					commission,
					minSelfDelegation,
					validatorAddress,
					pubkey,
					value,
				)
			},
			200000,
			nil,
			func(data []byte) {
				var out staking.CreateValidatorReturn
				_, err := out.Decode(data)
				s.Require().NoError(err)
				s.Require().Equal(out.Success, true)

				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				s.Require().Equal(log.BlockNumber, uint64(s.network.GetContext().BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var createValidatorEvent staking.CreateValidatorEvent
				err = abi.DecodeEvent(&createValidatorEvent, log.Topics, log.Data)
				s.Require().NoError(err)
				s.Require().Equal(validatorAddress, createValidatorEvent.ValidatorAddress)
				s.Require().Equal(value, createValidatorEvent.Value)

				// check the validator state
				validator, err := s.network.App.GetStakingKeeper().GetValidator(s.network.GetContext(), validatorAddress.Bytes())
				s.Require().NoError(err)
				s.Require().NotNil(validator, "expected validator not to be nil")
				expRate := math.LegacyNewDecFromBigIntWithPrec(commission.Rate, math.LegacyPrecision)
				s.Require().Equal(expRate, validator.Commission.Rate, "expected validator commission rate to be %s; got %s", expRate, validator.Commission.Rate)
				expMaxRate := math.LegacyNewDecFromBigIntWithPrec(commission.MaxRate, math.LegacyPrecision)
				s.Require().Equal(expMaxRate, validator.Commission.MaxRate, "expected validator commission max rate to be %s; got %s", expMaxRate, validator.Commission.MaxRate)
				expMaxChangeRate := math.LegacyNewDecFromBigIntWithPrec(commission.MaxChangeRate, math.LegacyPrecision)
				s.Require().Equal(expMaxChangeRate, validator.Commission.MaxChangeRate, "expected validator commission max change rate to be %s; got %s", expMaxChangeRate, validator.Commission.MaxChangeRate)
				s.Require().Equal(math.NewIntFromBigInt(minSelfDelegation), validator.MinSelfDelegation, "expected validator min self delegation to be %s; got %s", minSelfDelegation, validator.MinSelfDelegation)
			},
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			ctx := s.network.GetContext()
			stDB = s.network.GetStateDB()

			// reset sender
			validator := s.keyring.GetKey(0)
			validatorAddress = validator.Addr
			caller := validatorAddress
			if tc.callerAddress != nil {
				caller = *tc.callerAddress
			}

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(s.T(), ctx, caller, s.precompile.Address(), tc.gas)

			out, err := s.precompile.CreateValidator(ctx, *tc.malleate(), stDB, contract)

			if tc.expError {
				s.Require().ErrorContains(err, tc.errContains)
				s.Require().Nil(out)
			} else {
				s.Require().NoError(err)
				// query the validator in the staking keeper
				validator, err := s.network.App.GetStakingKeeper().Validator(ctx, validator.AccAddr.Bytes())
				s.Require().NoError(err)

				s.Require().NotNil(validator, "expected validator not to be nil")
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)

				isBonded := validator.IsBonded()
				s.Require().Equal(false, isBonded, "expected validator bonded to be %t; got %t", false, isBonded)

				consPubKey, err := validator.ConsPubKey()
				s.Require().NoError(err)
				consPubKeyBase64 := base64.StdEncoding.EncodeToString(consPubKey.Bytes())
				s.Require().Equal(pubkey, consPubKeyBase64, "expected validator pubkey to be %s; got %s", pubkey, consPubKeyBase64)

				operator := validator.GetOperator()
				s.Require().Equal(sdk.ValAddress(validatorAddress.Bytes()).String(), operator, "expected validator operator to be %s; got %s", validatorAddress, operator)

				commissionRate := validator.GetCommission()
				s.Require().Equal(commission.Rate.String(), commissionRate.BigInt().String(), "expected validator commission rate to be %s; got %s", commission.Rate.String(), commissionRate.String())

				valMinSelfDelegation := validator.GetMinSelfDelegation()
				s.Require().Equal(minSelfDelegation.String(), valMinSelfDelegation.String(), "expected validator min self delegation to be %s; got %s", minSelfDelegation.String(), valMinSelfDelegation.String())

				moniker := validator.GetMoniker()
				s.Require().Equal(description.Moniker, moniker, "expected validator moniker to be %s; got %s", description.Moniker, moniker)

				jailed := validator.IsJailed()
				s.Require().Equal(false, jailed, "expected validator jailed to be %t; got %t", false, jailed)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestEditValidator() {
	var (
		stDB              *statedb.StateDB
		ctx               sdk.Context
		validatorAddress  common.Address
		commissionRate    *big.Int
		minSelfDelegation *big.Int
		description       = staking.Description{
			Moniker:         "node0-edited",
			Identity:        "",
			Website:         "",
			SecurityContact: "",
			Details:         "",
		}
	)

	testCases := []struct {
		name          string
		malleate      func() *staking.EditValidatorCall
		gas           uint64
		callerAddress *common.Address
		postCheck     func(data []byte)
		expError      bool
		errContains   string
	}{
		{
			"fail - different origin than delegator",
			func() *staking.EditValidatorCall {
				differentAddr := cosmosevmutiltx.GenerateAddress()
				return staking.NewEditValidatorCall(description,
					differentAddr,
					commissionRate,
					minSelfDelegation,
				)
			},
			200000,
			nil,
			func([]byte) {},
			true,
			"does not match the requester address",
		},
		{
			"fail - commission change rate too high",
			func() *staking.EditValidatorCall {
				return staking.NewEditValidatorCall(description,
					validatorAddress,
					math.LegacyNewDecWithPrec(11, 2).BigInt(),
					minSelfDelegation,
				)
			},
			200000,
			nil,
			func([]byte) {},
			true,
			"commission cannot be changed more than max change rate",
		},
		{
			"fail - negative commission rate",
			func() *staking.EditValidatorCall {
				return staking.NewEditValidatorCall(description,
					validatorAddress,
					math.LegacyNewDecWithPrec(-5, 2).BigInt(),
					minSelfDelegation,
				)
			},
			200000,
			nil,
			func([]byte) {},
			true,
			"commission rate must be between 0 and 1 (inclusive)",
		},
		{
			"fail - negative min self delegation",
			func() *staking.EditValidatorCall {
				return staking.NewEditValidatorCall(description,
					validatorAddress,
					commissionRate,
					math.LegacyNewDecWithPrec(-5, 2).BigInt(),
				)
			},
			200000,
			nil,
			func([]byte) {},
			true,
			"minimum self delegation must be a positive integer",
		},
		{
			"fail - cannot be called from account with code (if it is not EIP-7702 delegated account)",
			func() *staking.EditValidatorCall {
				return staking.NewEditValidatorCall(description,
					validatorAddress,
					commissionRate,
					minSelfDelegation,
				)
			},
			200000,
			func() *common.Address {
				addr := s.keyring.GetAddr(0)
				return &addr
			}(),
			func([]byte) {},
			true,
			"does not match the requester address",
		},
		{
			"fail - cannot be called from smart contract",
			func() *staking.EditValidatorCall {
				stDB.SetCode(validatorAddress, []byte{0x60, 0x00})
				return staking.NewEditValidatorCall(description,
					validatorAddress,
					commissionRate,
					minSelfDelegation,
				)
			},
			200000,
			nil,
			func([]byte) {},
			true,
			staking.ErrCannotCallFromContract,
		},
		{
			"success",
			func() *staking.EditValidatorCall {
				return staking.NewEditValidatorCall(description,
					validatorAddress,
					commissionRate,
					minSelfDelegation,
				)
			},
			200000,
			nil,
			func(data []byte) {
				var out staking.EditValidatorReturn
				_, err := out.Decode(data)
				s.Require().NoError(err)
				s.Require().Equal(out.Success, true)

				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var editValidatorEvent staking.EditValidatorEvent
				err = abi.DecodeEvent(&editValidatorEvent, log.Topics, log.Data)
				s.Require().NoError(err)
				s.Require().Equal(validatorAddress, editValidatorEvent.ValidatorAddress)
				s.Require().Equal(commissionRate, editValidatorEvent.CommissionRate)
				s.Require().Equal(minSelfDelegation, editValidatorEvent.MinSelfDelegation)
			},
			false,
			"",
		},
		{
			"success - should not update commission rate",
			func() *staking.EditValidatorCall {
				// expected commission rate is the previous one (5%)
				commissionRate = math.LegacyNewDecWithPrec(5, 2).BigInt()
				return staking.NewEditValidatorCall(description,
					validatorAddress,
					big.NewInt(-1),
					minSelfDelegation,
				)
			},
			200000,
			nil,
			func(data []byte) { //nolint:dupl
				var out staking.EditValidatorReturn
				_, err := out.Decode(data)
				s.Require().NoError(err)
				s.Require().Equal(out.Success, true)

				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var editValidatorEvent staking.EditValidatorEvent
				err = abi.DecodeEvent(&editValidatorEvent, log.Topics, log.Data)
				s.Require().NoError(err)
				s.Require().Equal(validatorAddress, editValidatorEvent.ValidatorAddress)
			},
			false,
			"",
		},
		{
			"success - should not update minimum self delegation",
			func() *staking.EditValidatorCall {
				// expected min self delegation is the previous one (0)
				minSelfDelegation = math.LegacyZeroDec().BigInt()
				return staking.NewEditValidatorCall(description,
					validatorAddress,
					commissionRate,
					big.NewInt(-1),
				)
			},
			200000,
			nil,
			func(data []byte) { //nolint:dupl
				var out staking.EditValidatorReturn
				_, err := out.Decode(data)
				s.Require().NoError(err)
				s.Require().Equal(out.Success, true)

				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var editValidatorEvent staking.EditValidatorEvent
				err = abi.DecodeEvent(&editValidatorEvent, log.Topics, log.Data)
				s.Require().NoError(err)
				s.Require().Equal(validatorAddress, editValidatorEvent.ValidatorAddress)
			},
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			commissionRate = math.LegacyNewDecWithPrec(1, 1).BigInt()
			minSelfDelegation = big.NewInt(11)

			// reset sender
			valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
			s.Require().NoError(err)

			validatorAddress = common.BytesToAddress(valAddr.Bytes())
			caller := validatorAddress
			if tc.callerAddress != nil {
				caller = *tc.callerAddress
			}

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(s.T(), ctx, caller, s.precompile.Address(), tc.gas)

			out, err := s.precompile.EditValidator(ctx, *tc.malleate(), stDB, contract)

			if tc.expError {
				s.Require().ErrorContains(err, tc.errContains)
				s.Require().Nil(out)
			} else {
				s.Require().NoError(err)

				// query the validator in the staking keeper
				validator, err := s.network.App.GetStakingKeeper().Validator(ctx, valAddr.Bytes())
				s.Require().NoError(err)

				s.Require().NotNil(validator, "expected validator not to be nil")
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)

				isBonded := validator.IsBonded()
				s.Require().Equal(true, isBonded, "expected validator bonded to be %t; got %t", true, isBonded)

				operator := validator.GetOperator()
				s.Require().Equal(sdk.ValAddress(validatorAddress.Bytes()).String(), operator, "expected validator operator to be %s; got %s", validatorAddress, operator)

				updatedCommRate := validator.GetCommission()
				s.Require().Equal(commissionRate.String(), updatedCommRate.BigInt().String(), "expected validator commission rate to be %s; got %s", commissionRate.String(), commissionRate.String())

				valMinSelfDelegation := validator.GetMinSelfDelegation()
				s.Require().Equal(minSelfDelegation.String(), valMinSelfDelegation.String(), "expected validator min self delegation to be %s; got %s", minSelfDelegation.String(), valMinSelfDelegation.String())

				moniker := validator.GetMoniker()
				s.Require().Equal(description.Moniker, moniker, "expected validator moniker to be %s; got %s", description.Moniker, moniker)

				jailed := validator.IsJailed()
				s.Require().Equal(false, jailed, "expected validator jailed to be %t; got %t", false, jailed)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestDelegate() {
	var (
		ctx  sdk.Context
		stDB *statedb.StateDB
	)
	testCases := []struct {
		name                string
		malleate            func(delegator testkeyring.Key, operatorAddress string) *staking.DelegateCall
		gas                 uint64
		expDelegationShares *big.Int
		postCheck           func(data []byte)
		expError            bool
		errContains         string
	}{
		{
			name: "fail - different origin than delegator",
			malleate: func(_ testkeyring.Key, operatorAddress string) *staking.DelegateCall {
				differentAddr := cosmosevmutiltx.GenerateAddress()
				return staking.NewDelegateCall(differentAddr,
					operatorAddress,
					big.NewInt(1e18),
				)
			},
			gas:         200000,
			expError:    true,
			errContains: "does not match the requester address",
		},
		{
			"fail - invalid amount",
			func(delegator testkeyring.Key, operatorAddress string) *staking.DelegateCall {
				return staking.NewDelegateCall(delegator.Addr,
					operatorAddress,
					nil,
				)
			},
			200000,
			big.NewInt(1),
			func([]byte) {},
			true,
			fmt.Sprintf(cmn.ErrInvalidAmount, nil),
		},
		{
			"fail - delegation failed because of insufficient funds",
			func(delegator testkeyring.Key, operatorAddress string) *staking.DelegateCall {
				amt, ok := math.NewIntFromString("1000000000000000000000000000")
				s.Require().True(ok)
				return staking.NewDelegateCall(delegator.Addr,
					operatorAddress,
					amt.BigInt(),
				)
			},
			200000,
			big.NewInt(15),
			func([]byte) {},
			true,
			"insufficient funds",
		},
		{
			"success",
			func(delegator testkeyring.Key, operatorAddress string) *staking.DelegateCall {
				return staking.NewDelegateCall(delegator.Addr,
					operatorAddress,
					big.NewInt(1e18),
				)
			},
			20000,
			big.NewInt(2),
			func(data []byte) {
				var out staking.DelegateReturn
				_, err := out.Decode(data)
				s.Require().NoError(err)
				s.Require().Equal(out.Success, true)

				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())
				// Check event signature matches the one emitted
				s.Require().Equal(log.BlockNumber, uint64(s.network.GetContext().BlockHeight())) //nolint:gosec // G115
			},
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			delegator := s.keyring.GetKey(0)

			contract, ctx := testutil.NewPrecompileContract(s.T(), ctx, delegator.Addr, s.precompile.Address(), tc.gas)

			delegateArgs := tc.malleate(
				delegator,
				s.network.GetValidators()[0].OperatorAddress,
			)
			out, err := s.precompile.Delegate(ctx, *delegateArgs, stDB, contract)

			// query the delegation in the staking keeper
			valAddr, valErr := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
			s.Require().NoError(valErr)
			delegation, delErr := s.network.App.GetStakingKeeper().Delegation(ctx, delegator.AccAddr, valAddr)
			s.Require().NoError(delErr)
			if tc.expError {
				s.Require().ErrorContains(err, tc.errContains)
				s.Require().Empty(out)
				s.Require().Equal(s.network.GetValidators()[0].DelegatorShares, delegation.GetShares())
			} else {
				s.Require().NoError(err)
				s.Require().NotNil(delegation, "expected delegation not to be nil")
				out, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(out)

				expDelegationAmt := math.NewIntFromBigInt(tc.expDelegationShares)
				delegationAmt := delegation.GetShares().TruncateInt()

				s.Require().Equal(expDelegationAmt, delegationAmt, "expected delegation amount to be %d; got %d", expDelegationAmt, delegationAmt)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestUndelegate() {
	var (
		ctx  sdk.Context
		stDB *statedb.StateDB
	)

	testCases := []struct {
		name                  string
		malleate              func(delegator testkeyring.Key, operatorAddress string) *staking.UndelegateCall
		postCheck             func(data []byte)
		gas                   uint64
		expUndelegationShares *big.Int
		expError              bool
		errContains           string
	}{
		{
			name: "fail - different origin than delegator",
			malleate: func(_ testkeyring.Key, operatorAddress string) *staking.UndelegateCall {
				differentAddr := cosmosevmutiltx.GenerateAddress()
				return staking.NewUndelegateCall(differentAddr,
					operatorAddress,
					big.NewInt(1000000000000000000),
				)
			},
			gas:         200000,
			expError:    true,
			errContains: "does not match the requester address",
		},
		{
			"fail - invalid amount",
			func(delegator testkeyring.Key, operatorAddress string) *staking.UndelegateCall {
				return staking.NewUndelegateCall(delegator.Addr,
					operatorAddress,
					nil,
				)
			},
			func([]byte) {},
			200000,
			big.NewInt(1),
			true,
			fmt.Sprintf(cmn.ErrInvalidAmount, nil),
		},
		{
			"success",
			func(delegator testkeyring.Key, operatorAddress string) *staking.UndelegateCall {
				return staking.NewUndelegateCall(delegator.Addr,
					operatorAddress,
					big.NewInt(1000000000000000000),
				)
			},
			func(data []byte) {
				var out staking.UndelegateReturn
				_, err := out.Decode(data)
				s.Require().NoError(err, "failed to unpack output")
				params, err := s.network.App.GetStakingKeeper().GetParams(ctx)
				s.Require().NoError(err)
				expCompletionTime := ctx.BlockTime().Add(params.UnbondingTime).UTC().Unix()
				s.Require().Equal(expCompletionTime, out.CompletionTime)
				// Check the event emitted
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())
			},
			20000,
			big.NewInt(1000000000000000000),
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			delegator := s.keyring.GetKey(0)

			var contract *vm.Contract
			contract, ctx = testutil.NewPrecompileContract(s.T(), ctx, delegator.Addr, s.precompile.Address(), tc.gas)

			undelegateArgs := tc.malleate(delegator, s.network.GetValidators()[0].OperatorAddress)
			out, err := s.precompile.Undelegate(ctx, *undelegateArgs, stDB, contract)

			// query the unbonding delegations in the staking keeper
			undelegations, _ := s.network.App.GetStakingKeeper().GetAllUnbondingDelegations(ctx, delegator.AccAddr)

			if tc.expError {
				s.Require().ErrorContains(err, tc.errContains)
				s.Require().Empty(out)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(out)
				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)

				s.Require().Equal(undelegations[0].DelegatorAddress, delegator.AccAddr.String())
				s.Require().Equal(undelegations[0].ValidatorAddress, s.network.GetValidators()[0].OperatorAddress)
				s.Require().Equal(undelegations[0].Entries[0].Balance, math.NewIntFromBigInt(tc.expUndelegationShares))
			}
		})
	}
}

func (s *PrecompileTestSuite) TestRedelegate() {
	var ctx sdk.Context

	testCases := []struct {
		name                  string
		malleate              func(delegator testkeyring.Key, srcOperatorAddr, dstOperatorAddr string) *staking.RedelegateCall
		postCheck             func(data []byte)
		gas                   uint64
		expRedelegationShares *big.Int
		expError              bool
		errContains           string
	}{
		{
			name: "fail - different origin than delegator",
			malleate: func(_ testkeyring.Key, srcOperatorAddr, dstOperatorAddr string) *staking.RedelegateCall {
				differentAddr := cosmosevmutiltx.GenerateAddress()
				return staking.NewRedelegateCall(differentAddr,
					srcOperatorAddr,
					dstOperatorAddr,
					big.NewInt(1000000000000000000),
				)
			},
			gas:         200000,
			expError:    true,
			errContains: "does not match the requester address",
		},
		{
			"fail - invalid amount",
			func(delegator testkeyring.Key, srcOperatorAddr, dstOperatorAddr string) *staking.RedelegateCall {
				return staking.NewRedelegateCall(delegator.Addr,
					srcOperatorAddr,
					dstOperatorAddr,
					nil,
				)
			},
			func([]byte) {},
			200000,
			big.NewInt(1),
			true,
			fmt.Sprintf(cmn.ErrInvalidAmount, nil),
		},
		{
			"fail - invalid shares amount",
			func(delegator testkeyring.Key, srcOperatorAddr, dstOperatorAddr string) *staking.RedelegateCall {
				return staking.NewRedelegateCall(delegator.Addr,
					srcOperatorAddr,
					dstOperatorAddr,
					big.NewInt(-1),
				)
			},
			func([]byte) {},
			200000,
			big.NewInt(1),
			true,
			"invalid shares amount",
		},
		{
			"success",
			func(delegator testkeyring.Key, srcOperatorAddr, dstOperatorAddr string) *staking.RedelegateCall {
				return staking.NewRedelegateCall(delegator.Addr,
					srcOperatorAddr,
					dstOperatorAddr,
					big.NewInt(1000000000000000000),
				)
			},
			func(data []byte) {
				var out staking.RedelegateReturn
				_, err := out.Decode(data)
				s.Require().NoError(err, "failed to unpack output")
				params, err := s.network.App.GetStakingKeeper().GetParams(ctx)
				s.Require().NoError(err)
				expCompletionTime := ctx.BlockTime().Add(params.UnbondingTime).UTC().Unix()
				s.Require().Equal(expCompletionTime, out.CompletionTime)
			},
			200000,
			big.NewInt(1),
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			delegator := s.keyring.GetKey(0)

			contract, ctx := testutil.NewPrecompileContract(s.T(), ctx, delegator.Addr, s.precompile.Address(), tc.gas)

			redelegateArgs := tc.malleate(
				delegator,
				s.network.GetValidators()[0].OperatorAddress,
				s.network.GetValidators()[1].OperatorAddress,
			)
			bz, err := s.precompile.Redelegate(ctx, *redelegateArgs, s.network.GetStateDB(), contract)

			// query the redelegations in the staking keeper
			redelegations, redelErr := s.network.App.GetStakingKeeper().GetRedelegations(ctx, delegator.AccAddr, 5)
			s.Require().NoError(redelErr)

			if tc.expError {
				s.Require().ErrorContains(err, tc.errContains)
				s.Require().Empty(bz)
			} else {
				s.Require().NoError(err)
				s.Require().NotEmpty(bz)

				s.Require().Equal(redelegations[0].DelegatorAddress, delegator.AccAddr.String())
				s.Require().Equal(redelegations[0].ValidatorSrcAddress, s.network.GetValidators()[0].OperatorAddress)
				s.Require().Equal(redelegations[0].ValidatorDstAddress, s.network.GetValidators()[1].OperatorAddress)
				s.Require().Equal(redelegations[0].Entries[0].SharesDst, math.LegacyNewDecFromBigInt(tc.expRedelegationShares))
			}
		})
	}
}

func (s *PrecompileTestSuite) TestCancelUnbondingDelegation() {
	var ctx sdk.Context

	testCases := []struct {
		name               string
		malleate           func(delegator testkeyring.Key, operatorAddress string) *staking.CancelUnbondingDelegationCall
		postCheck          func(data []byte)
		gas                uint64
		expDelegatedShares *big.Int
		expError           bool
		errContains        string
	}{
		{
			"fail - creation height",
			func(delegator testkeyring.Key, operatorAddress string) *staking.CancelUnbondingDelegationCall {
				return staking.NewCancelUnbondingDelegationCall(delegator.Addr,
					operatorAddress,
					big.NewInt(1),
					nil,
				)
			},
			func([]byte) {},
			200000,
			big.NewInt(1),
			true,
			"invalid creation height",
		},
		{
			"fail - invalid amount",
			func(delegator testkeyring.Key, operatorAddress string) *staking.CancelUnbondingDelegationCall {
				return staking.NewCancelUnbondingDelegationCall(delegator.Addr,
					operatorAddress,
					nil,
					big.NewInt(1),
				)
			},
			func([]byte) {},
			200000,
			big.NewInt(1),
			true,
			fmt.Sprintf(cmn.ErrInvalidAmount, nil),
		},
		{
			"fail - invalid amount",
			func(delegator testkeyring.Key, operatorAddress string) *staking.CancelUnbondingDelegationCall {
				return staking.NewCancelUnbondingDelegationCall(delegator.Addr,
					operatorAddress,
					nil,
					big.NewInt(1),
				)
			},
			func([]byte) {},
			200000,
			big.NewInt(1),
			true,
			fmt.Sprintf(cmn.ErrInvalidAmount, nil),
		},
		{
			"fail - invalid shares amount",
			func(delegator testkeyring.Key, operatorAddress string) *staking.CancelUnbondingDelegationCall {
				return staking.NewCancelUnbondingDelegationCall(delegator.Addr,
					operatorAddress,
					big.NewInt(-1),
					big.NewInt(1),
				)
			},
			func([]byte) {},
			200000,
			big.NewInt(1),
			true,
			"invalid amount: invalid request",
		},
		{
			"success",
			func(delegator testkeyring.Key, operatorAddress string) *staking.CancelUnbondingDelegationCall {
				return staking.NewCancelUnbondingDelegationCall(delegator.Addr,
					operatorAddress,
					big.NewInt(1),
					big.NewInt(1),
				)
			},
			func(data []byte) {
				var out staking.CancelUnbondingDelegationReturn
				_, err := out.Decode(data)
				s.Require().NoError(err)
				s.Require().Equal(out.Success, true)
			},
			200000,
			big.NewInt(1),
			false,
			"",
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.network.GetContext()
			stDB := s.network.GetStateDB()

			delegator := s.keyring.GetKey(0)

			contract, ctx := testutil.NewPrecompileContract(s.T(), ctx, delegator.Addr, s.precompile.Address(), tc.gas)
			cancelArgs := tc.malleate(delegator, s.network.GetValidators()[0].OperatorAddress)

			if tc.expError {
				bz, err := s.precompile.CancelUnbondingDelegation(ctx, *cancelArgs, stDB, contract)
				s.Require().ErrorContains(err, tc.errContains)
				s.Require().Empty(bz)
			} else {
				undelegateArgs := staking.NewUndelegateCall(
					delegator.Addr,
					s.network.GetValidators()[0].OperatorAddress,
					big.NewInt(1000000000000000000),
				)

				_, err := s.precompile.Undelegate(ctx, *undelegateArgs, stDB, contract)
				s.Require().NoError(err)

				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)

				_, err = s.network.App.GetStakingKeeper().GetDelegation(ctx, delegator.AccAddr, valAddr)
				s.Require().Error(err)
				s.Require().Contains("no delegation for (address, validator) tuple", err.Error())

				out, err := s.precompile.CancelUnbondingDelegation(ctx, *cancelArgs, stDB, contract)
				s.Require().NoError(err)

				bz, err := out.Encode()
				s.Require().NoError(err)
				tc.postCheck(bz)

				delegation, err := s.network.App.GetStakingKeeper().GetDelegation(ctx, delegator.AccAddr, valAddr)
				s.Require().NoError(err)

				s.Require().Equal(delegation.DelegatorAddress, delegator.AccAddr.String())
				s.Require().Equal(delegation.ValidatorAddress, s.network.GetValidators()[0].OperatorAddress)
				s.Require().Equal(delegation.Shares, math.LegacyNewDecFromBigInt(tc.expDelegatedShares))

			}
		})
	}
}
