package staking

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/staking"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	"github.com/cosmos/evm/x/vm/statedb"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *PrecompileTestSuite) TestCreateValidatorEvent() {
	var (
		stDB            *statedb.StateDB
		ctx             sdk.Context
		delegationValue = big.NewInt(1205000000000000000)
		method          = s.precompile.Methods[staking.CreateValidatorMethod]
		pubkey          = "nfJ0axJC9dhta1MAE1EBFaVdxxkYzxYrBaHuJVjG//M="
	)

	testCases := []struct {
		name        string
		malleate    func(delegator common.Address) []interface{}
		expErr      bool
		errContains string
		postCheck   func(delegator common.Address)
	}{
		{
			name: "success - the correct event is emitted",
			malleate: func(delegator common.Address) []interface{} {
				return []interface{}{
					staking.Description{
						Moniker:         "node0",
						Identity:        "",
						Website:         "",
						SecurityContact: "",
						Details:         "",
					},
					staking.Commission{
						Rate:          math.LegacyOneDec().BigInt(),
						MaxRate:       math.LegacyOneDec().BigInt(),
						MaxChangeRate: math.LegacyOneDec().BigInt(),
					},
					big.NewInt(1),
					delegator,
					pubkey,
					delegationValue,
				}
			},
			postCheck: func(delegator common.Address) {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				event := s.precompile.Events[staking.EventTypeCreateValidator]
				s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var createValidatorEvent staking.EventCreateValidator
				err := cmn.UnpackLog(s.precompile.ABI, &createValidatorEvent, staking.EventTypeCreateValidator, *log)
				s.Require().NoError(err)
				s.Require().Equal(delegator, createValidatorEvent.ValidatorAddress)
				s.Require().Equal(delegationValue, createValidatorEvent.Value)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			delegator := s.keyring.GetKey(0)

			contract := vm.NewContract(delegator.Addr, s.precompile.Address(), common.U2560, 200000, nil)
			_, err := s.precompile.CreateValidator(ctx, contract, stDB, &method, tc.malleate(delegator.Addr))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				tc.postCheck(delegator.Addr)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestEditValidatorEvent() {
	var (
		stDB        *statedb.StateDB
		ctx         sdk.Context
		valOperAddr common.Address
		method      = s.precompile.Methods[staking.EditValidatorMethod]
		minSelfDel  = big.NewInt(11)
		commRate    = math.LegacyNewDecWithPrec(5, 2).BigInt()
	)
	testCases := []struct {
		name        string
		malleate    func() []interface{}
		expErr      bool
		errContains string
		postCheck   func()
	}{
		{
			name: "success - the correct event is emitted",
			malleate: func() []interface{} {
				return []interface{}{
					staking.Description{
						Moniker:         "node0-edited",
						Identity:        "",
						Website:         "",
						SecurityContact: "",
						Details:         "",
					},
					valOperAddr,
					commRate,
					minSelfDel,
				}
			},
			postCheck: func() {
				s.Require().Equal(len(stDB.Logs()), 1)
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				event := s.precompile.Events[staking.EventTypeEditValidator]
				s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				// Check the fully unpacked event matches the one emitted
				var editValidatorEvent staking.EventEditValidator
				err := cmn.UnpackLog(s.precompile.ABI, &editValidatorEvent, staking.EventTypeEditValidator, *log)
				s.Require().NoError(err)
				s.Require().Equal(valOperAddr, editValidatorEvent.ValidatorAddress)
				s.Require().Equal(minSelfDel, editValidatorEvent.MinSelfDelegation)
				s.Require().Equal(commRate, editValidatorEvent.CommissionRate)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			acc, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
			s.Require().NoError(err)
			valOperAddr = common.BytesToAddress(acc.Bytes())

			contract := vm.NewContract(valOperAddr, s.precompile.Address(), common.U2560, 200000, nil)
			_, err = s.precompile.EditValidator(ctx, contract, stDB, &method, tc.malleate())

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				tc.postCheck()
			}
		})
	}
}

func (s *PrecompileTestSuite) TestDelegateEvent() {
	var (
		stDB          *statedb.StateDB
		ctx           sdk.Context
		delegationAmt = big.NewInt(1500000000000000000)
		newSharesExp  = delegationAmt
		method        = s.precompile.Methods[staking.DelegateMethod]
	)
	testCases := []struct {
		name        string
		malleate    func(delegator common.Address) []interface{}
		expErr      bool
		errContains string
		postCheck   func(delegator common.Address)
	}{
		{
			"success - the correct event is emitted",
			func(delegator common.Address) []interface{} {
				return []interface{}{
					delegator,
					s.network.GetValidators()[0].OperatorAddress,
					delegationAmt,
				}
			},
			false,
			"",
			func(delegator common.Address) {
				log := stDB.Logs()[0]
				s.Require().Equal(log.Address, s.precompile.Address())

				// Check event signature matches the one emitted
				event := s.precompile.Events[staking.EventTypeDelegate]
				s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				optAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)
				optHexAddr := common.BytesToAddress(optAddr)

				// Check the fully unpacked event matches the one emitted
				var delegationEvent staking.EventDelegate
				err = cmn.UnpackLog(s.precompile.ABI, &delegationEvent, staking.EventTypeDelegate, *log)
				s.Require().NoError(err)
				s.Require().Equal(delegator, delegationEvent.DelegatorAddress)
				s.Require().Equal(optHexAddr, delegationEvent.ValidatorAddress)
				s.Require().Equal(delegationAmt, delegationEvent.Amount)
				s.Require().Equal(newSharesExp, delegationEvent.NewShares)
			},
		},
	}

	for _, tc := range testCases { //nolint:dupl
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			delegator := s.keyring.GetKey(0)

			contract := vm.NewContract(delegator.Addr, s.precompile.Address(), common.U2560, 20000, nil)
			_, err := s.precompile.Delegate(ctx, contract, stDB, &method, tc.malleate(delegator.Addr))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				tc.postCheck(delegator.Addr)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestUnbondEvent() {
	var (
		stDB *statedb.StateDB
		ctx  sdk.Context
	)
	method := s.precompile.Methods[staking.UndelegateMethod]

	testCases := []struct {
		name        string
		malleate    func(delegator common.Address) []interface{}
		expErr      bool
		errContains string
		postCheck   func(delegator common.Address)
	}{
		{
			"success - the correct event is emitted",
			func(delegator common.Address) []interface{} {
				return []interface{}{
					delegator,
					s.network.GetValidators()[0].OperatorAddress,
					big.NewInt(1000000000000000000),
				}
			},
			false,
			"",
			func(delegator common.Address) {
				log := stDB.Logs()[0]
				// Check event signature matches the one emitted
				event := s.precompile.Events[staking.EventTypeUnbond]
				s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				optAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)
				optHexAddr := common.BytesToAddress(optAddr)

				// Check the fully unpacked event matches the one emitted
				var unbondEvent staking.EventUnbond
				err = cmn.UnpackLog(s.precompile.ABI, &unbondEvent, staking.EventTypeUnbond, *log)
				s.Require().NoError(err)
				s.Require().Equal(delegator, unbondEvent.DelegatorAddress)
				s.Require().Equal(optHexAddr, unbondEvent.ValidatorAddress)
				s.Require().Equal(big.NewInt(1000000000000000000), unbondEvent.Amount)
			},
		},
	}

	for _, tc := range testCases { //nolint:dupl
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			delegator := s.keyring.GetKey(0)

			contract := vm.NewContract(delegator.Addr, s.precompile.Address(), common.U2560, 20000, nil)
			_, err := s.precompile.Undelegate(ctx, contract, stDB, &method, tc.malleate(delegator.Addr))

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				s.Require().NoError(err)
				tc.postCheck(delegator.Addr)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestRedelegateEvent() {
	var (
		stDB *statedb.StateDB
		ctx  sdk.Context
	)
	method := s.precompile.Methods[staking.RedelegateMethod]

	testCases := []struct {
		name        string
		malleate    func(delegator common.Address) []interface{}
		expErr      bool
		errContains string
		postCheck   func(delegator common.Address)
	}{
		{
			"success - the correct event is emitted",
			func(delegator common.Address) []interface{} {
				return []interface{}{
					delegator,
					s.network.GetValidators()[0].OperatorAddress,
					s.network.GetValidators()[1].OperatorAddress,
					big.NewInt(1000000000000000000),
				}
			},
			false,
			"",
			func(delegator common.Address) {
				log := stDB.Logs()[0]
				// Check event signature matches the one emitted
				event := s.precompile.Events[staking.EventTypeRedelegate]
				s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				optSrcAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)
				optSrcHexAddr := common.BytesToAddress(optSrcAddr)

				optDstAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[1].OperatorAddress)
				s.Require().NoError(err)
				optDstHexAddr := common.BytesToAddress(optDstAddr)

				var redelegateEvent staking.EventRedelegate
				err = cmn.UnpackLog(s.precompile.ABI, &redelegateEvent, staking.EventTypeRedelegate, *log)
				s.Require().NoError(err)
				s.Require().Equal(delegator, redelegateEvent.DelegatorAddress)
				s.Require().Equal(optSrcHexAddr, redelegateEvent.ValidatorSrcAddress)
				s.Require().Equal(optDstHexAddr, redelegateEvent.ValidatorDstAddress)
				s.Require().Equal(big.NewInt(1000000000000000000), redelegateEvent.Amount)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			delegator := s.keyring.GetKey(0)

			contract := vm.NewContract(delegator.Addr, s.precompile.Address(), common.U2560, 20000, nil)
			_, err := s.precompile.Redelegate(ctx, contract, stDB, &method, tc.malleate(delegator.Addr))
			s.Require().NoError(err)

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				tc.postCheck(delegator.Addr)
			}
		})
	}
}

func (s *PrecompileTestSuite) TestCancelUnbondingDelegationEvent() {
	var (
		stDB *statedb.StateDB
		ctx  sdk.Context
	)
	methodCancelUnbonding := s.precompile.Methods[staking.CancelUnbondingDelegationMethod]
	methodUndelegate := s.precompile.Methods[staking.UndelegateMethod]

	testCases := []struct {
		name        string
		malleate    func(contract *vm.Contract, delegator testkeyring.Key) []interface{}
		expErr      bool
		errContains string
		postCheck   func(delegator common.Address)
	}{
		{
			"success - the correct event is emitted",
			func(contract *vm.Contract, delegator testkeyring.Key) []interface{} {
				undelegateArgs := []interface{}{
					delegator.Addr,
					s.network.GetValidators()[0].OperatorAddress,
					big.NewInt(1000000000000000000),
				}
				_, err := s.precompile.Undelegate(ctx, contract, stDB, &methodUndelegate, undelegateArgs)
				s.Require().NoError(err)

				return []interface{}{
					delegator.Addr,
					s.network.GetValidators()[0].OperatorAddress,
					big.NewInt(1000000000000000000),
					big.NewInt(1),
				}
			},
			false,
			"",
			func(delegator common.Address) {
				log := stDB.Logs()[1]

				// Check event signature matches the one emitted
				event := s.precompile.Events[staking.EventTypeCancelUnbondingDelegation]
				s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
				s.Require().Equal(log.BlockNumber, uint64(ctx.BlockHeight())) //nolint:gosec // G115

				optAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)
				optHexAddr := common.BytesToAddress(optAddr)

				// Check event fields match the ones emitted
				var cancelUnbondEvent staking.EventCancelUnbonding
				err = cmn.UnpackLog(s.precompile.ABI, &cancelUnbondEvent, staking.EventTypeCancelUnbondingDelegation, *log)
				s.Require().NoError(err)
				s.Require().Equal(delegator, cancelUnbondEvent.DelegatorAddress)
				s.Require().Equal(optHexAddr, cancelUnbondEvent.ValidatorAddress)
				s.Require().Equal(big.NewInt(1000000000000000000), cancelUnbondEvent.Amount)
				s.Require().Equal(big.NewInt(1), cancelUnbondEvent.CreationHeight)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()
			stDB = s.network.GetStateDB()

			delegator := s.keyring.GetKey(0)

			contract := vm.NewContract(delegator.Addr, s.precompile.Address(), uint256.NewInt(0), 20000, nil)
			callArgs := tc.malleate(contract, delegator)
			_, err := s.precompile.CancelUnbondingDelegation(ctx, contract, stDB, &methodCancelUnbonding, callArgs)
			s.Require().NoError(err)

			if tc.expErr {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errContains)
			} else {
				tc.postCheck(delegator.Addr)
			}
		})
	}
}
