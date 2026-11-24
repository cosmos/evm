package common_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/precompiles/bank"
	"github.com/cosmos/evm/precompiles/bech32"
	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/distribution"
	"github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/precompiles/gov"
	"github.com/cosmos/evm/precompiles/ics02"
	"github.com/cosmos/evm/precompiles/ics20"
	"github.com/cosmos/evm/precompiles/slashing"
	"github.com/cosmos/evm/precompiles/staking"
	"github.com/cosmos/evm/precompiles/werc20"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type setupTarget struct {
	name              string
	api               abi.ABI
	isTx              func(*abi.Method) bool
	execute           func(sdk.Context, vm.StateDB, *vm.Contract, bool) ([]byte, error)
	query, tx         string
	queryArgs, txArgs []interface{}
}

func setupTargets() []setupTarget {
	b := bank.Precompile{ABI: bank.ABI}
	e := erc20.Precompile{ABI: erc20.ABI}
	i := ics02.Precompile{ABI: ics02.ABI}
	d := distribution.Precompile{ABI: distribution.ABI}
	g := gov.Precompile{ABI: gov.ABI}
	s := slashing.Precompile{ABI: slashing.ABI}
	st := staking.Precompile{ABI: staking.ABI}
	ibc := ics20.Precompile{ABI: ics20.ABI}
	w := werc20.Precompile{Precompile: &erc20.Precompile{ABI: werc20.ABI}}
	return []setupTarget{
		{"bank", bank.ABI, b.IsTransaction, func(ctx sdk.Context, _ vm.StateDB, c *vm.Contract, ro bool) ([]byte, error) {
			return b.Execute(ctx, c, ro)
		}, "balances", "", []interface{}{common.Address{}}, nil},
		{"erc20", erc20.ABI, e.IsTransaction, e.Execute, "balanceOf", "approve", []interface{}{common.Address{}}, []interface{}{common.Address{}, big.NewInt(1)}},
		{"ics02", ics02.ABI, i.IsTransaction, i.Execute, "getClientState", "updateClient", []interface{}{"client"}, []interface{}{"client", []byte{1}}},
		{"distribution", distribution.ABI, d.IsTransaction, d.Execute, "validatorCommission", "setWithdrawAddress", []interface{}{"validator"}, []interface{}{common.Address{}, "withdrawer"}},
		{"gov", gov.ABI, g.IsTransaction, g.Execute, "getProposal", "cancelProposal", []interface{}{uint64(1)}, []interface{}{common.Address{}, uint64(1)}},
		{"slashing", slashing.ABI, s.IsTransaction, s.Execute, "getSigningInfo", "unjail", []interface{}{common.Address{}}, []interface{}{common.Address{}}},
		{"staking", staking.ABI, st.IsTransaction, st.Execute, "validator", "delegate", []interface{}{common.Address{}}, []interface{}{common.Address{}, "validator", big.NewInt(1)}},
		{"ics20", ics20.ABI, ibc.IsTransaction, ibc.Execute, "denom", "transfer", []interface{}{"00"}, []interface{}{"transfer", "channel-0", "denom", big.NewInt(1), common.Address{}, "receiver", struct {
			RevisionNumber uint64
			RevisionHeight uint64
		}{}, uint64(1), ""}},
		{"werc20", werc20.ABI, w.IsTransaction, w.Execute, "balanceOf", "withdraw", []interface{}{common.Address{}}, []interface{}{big.NewInt(1)}},
	}
}

func TestStatefulABISetupEntrypoints(t *testing.T) {
	for _, target := range setupTargets() {
		t.Run(target.name, func(t *testing.T) {
			malformed := target.api.Methods[target.query].ID
			unknown := []byte{0xff, 0xff, 0xff, 0xff}
			_, unknownErr := target.api.MethodById(unknown)
			require.Error(t, unknownErr)
			_, malformedErr := target.api.Methods[target.query].Inputs.Unpack(nil)
			require.Error(t, malformedErr)
			cases := []struct {
				input  []byte
				reason string
			}{
				{nil, vm.ErrExecutionReverted.Error()},
				{[]byte{1}, vm.ErrExecutionReverted.Error()},
				{[]byte{1, 2}, vm.ErrExecutionReverted.Error()},
				{[]byte{1, 2, 3}, vm.ErrExecutionReverted.Error()},
				{unknown, unknownErr.Error()},
				{malformed, malformedErr.Error()},
			}
			for index, tc := range cases {
				if index < len(cases)-1 && target.api.HasFallback() {
					continue
				}
				contract := vm.NewContract(common.Address{}, common.Address{}, uint256.NewInt(0), 100_000, nil)
				contract.Input = tc.input
				expected := cmn.NewRevertWithSolidityError(target.api, cmn.SolidityErrABISetupFailed, tc.reason)
				method, args, setupErr := cmn.SetupABI(target.api, contract, false, target.isTx)
				require.Nil(t, method)
				require.Nil(t, args)
				_, actual := target.execute(sdk.Context{}, nil, contract, false)
				for _, returned := range []error{setupErr, actual} {
					var carrier cmn.RevertDataCarrier
					require.ErrorAs(t, returned, &carrier, "calldata %x", tc.input)
					require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), carrier.RevertData())
				}
			}
			// Valid query/tx setup and STATICCALL keep the existing classification.
			for _, tx := range []bool{false, true} {
				name, args := target.query, target.queryArgs
				if tx {
					name, args = target.tx, target.txArgs
				}
				if name == "" {
					continue
				}
				input, err := target.api.Pack(name, args...)
				require.NoError(t, err)
				contract := vm.NewContract(common.Address{}, common.Address{}, uint256.NewInt(0), 100_000, nil)
				contract.Input = input
				method, _, err := cmn.SetupABI(target.api, contract, false, target.isTx)
				require.NoError(t, err)
				require.Equal(t, tx, target.isTx(method))
				_, _, err = cmn.SetupABI(target.api, contract, true, target.isTx)
				if !tx {
					require.NoError(t, err)
					continue
				}
				expected := cmn.NewRevertWithSolidityError(target.api, cmn.SolidityErrABISetupFailed, vm.ErrWriteProtection.Error())
				require.Equal(t, expected, err)
				_, actual := target.execute(sdk.Context{}, nil, contract, true)
				require.Equal(t, expected, actual)
			}
		})
	}
}

func TestBech32KeepsSeparateUnknownSelector(t *testing.T) {
	p, err := bech32.NewPrecompile(1)
	require.NoError(t, err)
	for _, input := range [][]byte{nil, {1}, {1, 2}, {1, 2, 3}, {0xff, 0xff, 0xff, 0xff}} {
		contract := vm.NewContract(common.Address{}, common.Address{}, uint256.NewInt(0), 100_000, nil)
		contract.Input = input
		data, err := p.Run(&vm.EVM{}, contract, false)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		if len(input) < 4 {
			require.Equal(t, []byte{8, 0xc3, 0x79, 0xa0}, data[:4])
		} else {
			definition := bech32.ABI.Errors[cmn.SolidityErrUnknownMethod]
			require.Equal(t, definition.ID[:4], data[:4])
		}
	}
}

func TestABISetupPreservesSuccessfulSetup(t *testing.T) {
	for _, target := range setupTargets() {
		for _, tx := range []bool{false, true} {
			name, args := target.query, target.queryArgs
			if tx {
				name, args = target.tx, target.txArgs
			}
			if name == "" {
				continue
			}
			input, err := target.api.Pack(name, args...)
			require.NoError(t, err)
			contract := vm.NewContract(common.Address{}, common.Address{}, uint256.NewInt(0), 100_000, nil)
			contract.Input = input
			expectedMethod := target.api.Methods[name]
			expectedArgs, err := expectedMethod.Inputs.Unpack(input[4:])
			require.NoError(t, err)
			method, args, err := cmn.SetupABI(target.api, contract, false, target.isTx)
			require.NoError(t, err)
			require.Equal(t, &expectedMethod, method)
			require.Equal(t, expectedArgs, args)
		}
	}
	// WERC20 owns both successful fallback and receive handling. Keep the method
	// chosen for empty/short/unknown calldata and nonzero value, including STATICCALL.
	p := werc20.Precompile{Precompile: &erc20.Precompile{ABI: werc20.ABI}}
	for _, value := range []uint64{0, 1} {
		for _, input := range [][]byte{nil, {1}, {1, 2}, {1, 2, 3}, {0xff, 0xff, 0xff, 0xff}} {
			contract := vm.NewContract(common.Address{}, common.Address{}, uint256.NewInt(value), 100_000, nil)
			contract.Input = input
			expected := &werc20.ABI.Fallback
			if len(input) == 0 && value > 0 {
				expected = &werc20.ABI.Receive
			}
			actual, got, err := cmn.SetupABI(werc20.ABI, contract, false, p.IsTransaction)
			require.NoError(t, err)
			require.Equal(t, expected, actual)
			require.Nil(t, got)
			require.True(t, actual.Type == abi.Fallback || actual.Type == abi.Receive)
			_, _, err = cmn.SetupABI(werc20.ABI, contract, true, p.IsTransaction)
			require.Equal(t, cmn.NewRevertWithSolidityError(werc20.ABI, cmn.SolidityErrABISetupFailed, vm.ErrWriteProtection.Error()), err)
		}
	}
}
