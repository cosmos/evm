package staking

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	evmaddress "github.com/cosmos/evm/encoding/address"
	cmn "github.com/cosmos/evm/precompiles/common"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type stakingKeeperStub struct{ bondDenom string }

func (k stakingKeeperStub) BondDenom(context.Context) (string, error)   { return k.bondDenom, nil }
func (stakingKeeperStub) MaxValidators(context.Context) (uint32, error) { return 0, nil }
func (stakingKeeperStub) GetDelegatorValidators(context.Context, sdk.AccAddress, uint32) (stakingtypes.Validators, error) {
	return stakingtypes.Validators{}, nil
}

func (stakingKeeperStub) GetRedelegation(context.Context, sdk.AccAddress, sdk.ValAddress, sdk.ValAddress) (stakingtypes.Redelegation, error) {
	return stakingtypes.Redelegation{}, nil
}

func (stakingKeeperStub) GetValidator(context.Context, sdk.ValAddress) (stakingtypes.Validator, error) {
	return stakingtypes.Validator{}, nil
}

type queryServerStub struct {
	stakingtypes.UnimplementedQueryServer
	err error
}

func (q queryServerStub) Delegation(context.Context, *stakingtypes.QueryDelegationRequest) (*stakingtypes.QueryDelegationResponse, error) {
	return nil, q.err
}

func (q queryServerStub) UnbondingDelegation(context.Context, *stakingtypes.QueryUnbondingDelegationRequest) (*stakingtypes.QueryUnbondingDelegationResponse, error) {
	return nil, q.err
}

func (q queryServerStub) Validator(context.Context, *stakingtypes.QueryValidatorRequest) (*stakingtypes.QueryValidatorResponse, error) {
	return nil, q.err
}

type msgServerStub struct {
	stakingtypes.UnimplementedMsgServer
	err error
}

func (m msgServerStub) Delegate(context.Context, *stakingtypes.MsgDelegate) (*stakingtypes.MsgDelegateResponse, error) {
	return nil, m.err
}

func (m msgServerStub) CancelUnbondingDelegation(context.Context, *stakingtypes.MsgCancelUnbondingDelegation) (*stakingtypes.MsgCancelUnbondingDelegationResponse, error) {
	return nil, m.err
}

func TestReviewedQueryNotFoundOutcomesIgnoreMessageText(t *testing.T) {
	ctx := testContext()
	caller := common.HexToAddress("0x100")
	validator := sdk.ValAddress(caller.Bytes()).String()
	contract := vm.NewContract(caller, common.HexToAddress(vmtypes.StakingPrecompileAddress), uint256.NewInt(0), 100_000, nil)

	tests := []struct {
		name   string
		method string
		args   []interface{}
		call   func(Precompile, *abiMethodAndContract) ([]byte, error)
	}{
		{
			name: "delegation", method: DelegationMethod, args: []interface{}{caller, validator},
			call: func(p Precompile, input *abiMethodAndContract) ([]byte, error) {
				return p.Delegation(ctx, contract, input.method, input.args)
			},
		},
		{
			name: "unbonding", method: UnbondingDelegationMethod, args: []interface{}{caller, validator},
			call: func(p Precompile, input *abiMethodAndContract) ([]byte, error) {
				return p.UnbondingDelegation(ctx, contract, input.method, input.args)
			},
		},
		{
			name: "validator", method: ValidatorMethod, args: []interface{}{caller},
			call: func(p Precompile, input *abiMethodAndContract) ([]byte, error) {
				return p.Validator(ctx, input.method, contract, input.args)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var previous []byte
			for _, message := range []string{"not found", "upstream wording completely changed"} {
				p := testStakingPrecompile(&queryServerStub{err: status.Error(codes.NotFound, message)}, nil)
				input := &abiMethodAndContract{method: methodPointer(tc.method), args: tc.args}
				got, err := tc.call(p, input)
				require.NoError(t, err)
				require.NotEmpty(t, got)
				if previous != nil {
					require.Equal(t, previous, got)
				}
				previous = got
			}
		})
	}
}

func TestAmbiguousQueryStatusRemainsQueryFailed(t *testing.T) {
	caller := common.HexToAddress("0x100")
	validator := sdk.ValAddress(caller.Bytes()).String()
	p := testStakingPrecompile(&queryServerStub{err: status.Error(codes.InvalidArgument, "delegation with delegator looks not found")}, nil)
	method := methodPointer(DelegationMethod)
	_, err := p.Delegation(testContext(), nil, method, []interface{}{caller, validator})
	require.Error(t, err)
	require.Equal(t, stakingErrorSelector(cmn.SolidityErrQueryFailed), err.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestCancelUnbondingNotFoundExactSelectorIndependentOfMessage(t *testing.T) {
	caller := common.HexToAddress("0x100")
	validator := sdk.ValAddress(caller.Bytes()).String()
	contract := vm.NewContract(caller, common.HexToAddress(vmtypes.StakingPrecompileAddress), uint256.NewInt(0), 100_000, nil)
	args := []interface{}{caller, validator, big.NewInt(1), big.NewInt(1)}

	for _, message := range []string{"not found", "transport text changed"} {
		p := testStakingPrecompile(nil, &msgServerStub{err: status.Error(codes.NotFound, message)})
		_, err := p.CancelUnbondingDelegation(testContext(), contract, nil, methodPointer(CancelUnbondingDelegationMethod), args)
		require.Error(t, err)
		require.Equal(t, []byte{0x46, 0x41, 0xdb, 0x46}, err.(cmn.RevertDataCarrier).RevertData())
	}

	p := testStakingPrecompile(nil, &msgServerStub{err: sdkerrors.ErrNotFound.Wrap("registered text changed")})
	_, err := p.CancelUnbondingDelegation(testContext(), contract, nil, methodPointer(CancelUnbondingDelegationMethod), args)
	require.Error(t, err)
	require.Equal(t, []byte{0x76, 0x58, 0x16, 0xdd}, err.(cmn.RevertDataCarrier).RevertData())
}

func TestCancelUnbondingRegisteredSentinelIsNotGRPCDisposition(t *testing.T) {
	caller := common.HexToAddress("0x100")
	validator := sdk.ValAddress(caller.Bytes()).String()
	contract := vm.NewContract(caller, common.HexToAddress(vmtypes.StakingPrecompileAddress), uint256.NewInt(0), 100_000, nil)
	args := []interface{}{caller, validator, big.NewInt(1), big.NewInt(1)}

	for _, returned := range []error{
		stakingtypes.ErrNoUnbondingDelegation,
		fmt.Errorf("outer: %w", stakingtypes.ErrNoUnbondingDelegation),
	} {
		p := testStakingPrecompile(nil, &msgServerStub{err: returned})
		_, err := p.CancelUnbondingDelegation(testContext(), contract, nil, methodPointer(CancelUnbondingDelegationMethod), args)
		require.Error(t, err)
		expected := cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrUnmappedCosmosError, stakingtypes.ModuleName, uint32(26))
		require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), err.(cmn.RevertDataCarrier).RevertData())
		require.NotEqual(t, []byte{0x46, 0x41, 0xdb, 0x46}, err.(cmn.RevertDataCarrier).RevertData()[:4])
	}
}

func TestDelegateRegisteredErrorUsesConcreteSelector(t *testing.T) {
	caller := common.HexToAddress("0x100")
	validator := sdk.ValAddress(caller.Bytes()).String()
	contract := vm.NewContract(caller, common.HexToAddress(vmtypes.StakingPrecompileAddress), uint256.NewInt(0), 100_000, nil)
	for _, returned := range []error{stakingtypes.ErrNoValidatorFound, fmt.Errorf("outer: %w", stakingtypes.ErrNoValidatorFound)} {
		p := testStakingPrecompile(nil, &msgServerStub{err: returned})
		_, err := p.Delegate(testContext(), contract, nil, methodPointer(DelegateMethod), []interface{}{caller, validator, big.NewInt(1)})
		require.Error(t, err)
		require.Equal(t, stakingErrorSelector(SolidityErrStakingValidatorNotFound), err.(cmn.RevertDataCarrier).RevertData())
	}
}

type abiMethodAndContract struct {
	method *abi.Method
	args   []interface{}
}

func methodPointer(name string) *abi.Method {
	method := ABI.Methods[name]
	return &method
}

func testStakingPrecompile(query stakingtypes.QueryServer, msg stakingtypes.MsgServer) Precompile {
	return Precompile{
		ABI:              ABI,
		stakingKeeper:    stakingKeeperStub{bondDenom: "stake"},
		stakingQuerier:   query,
		stakingMsgServer: msg,
		addrCdc:          evmaddress.NewEvmCodec("cosmos"),
	}
}

func testContext() sdk.Context {
	return sdk.Context{}.WithLogger(log.NewNopLogger())
}
