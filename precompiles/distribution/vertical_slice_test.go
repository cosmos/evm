package distribution

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	evmaddress "github.com/cosmos/evm/encoding/address"
	cmn "github.com/cosmos/evm/precompiles/common"
	precompiletest "github.com/cosmos/evm/precompiles/testutil"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type distributionQueryServerStub struct {
	distributiontypes.UnimplementedQueryServer
	err error
}

func (q distributionQueryServerStub) ValidatorOutstandingRewards(context.Context, *distributiontypes.QueryValidatorOutstandingRewardsRequest) (*distributiontypes.QueryValidatorOutstandingRewardsResponse, error) {
	return nil, q.err
}

type distributionMsgServerStub struct {
	distributiontypes.UnimplementedMsgServer
	err error
}

func (m distributionMsgServerStub) SetWithdrawAddress(context.Context, *distributiontypes.MsgSetWithdrawAddress) (*distributiontypes.MsgSetWithdrawAddressResponse, error) {
	return nil, m.err
}

func TestDistributionQueryRegisteredErrorUsesConcreteSelector(t *testing.T) {
	validator := sdk.ValAddress(common.HexToAddress("0x100").Bytes()).String()
	method := ABI.Methods[ValidatorOutstandingRewardsMethod]
	for _, returned := range []error{
		stakingtypes.ErrNoValidatorFound,
		fmt.Errorf("outer: %w", stakingtypes.ErrNoValidatorFound),
	} {
		p := testDistributionPrecompile(&distributionQueryServerStub{err: returned}, nil)
		_, err := p.ValidatorOutstandingRewards(distributionTestContext(), nil, &method, []interface{}{validator})
		require.Error(t, err)
		revertData := err.(cmn.RevertDataCarrier).RevertData()
		selector := revertData[:4]
		require.Equal(t, distributionErrorSelector(SolidityErrDistributionNoValidatorExists), revertData)
		require.NotEqual(t, distributionErrorSelector(cmn.SolidityErrQueryFailed), selector)
		require.NotEqual(t, distributionErrorSelector(cmn.SolidityErrUnmappedCosmosError), selector)
	}
}

func TestDistributionMsgRegisteredErrorUsesConcreteSelector(t *testing.T) {
	caller := common.HexToAddress("0x100")
	contract := vm.NewContract(caller, common.HexToAddress(vmtypes.DistributionPrecompileAddress), uint256.NewInt(0), 100_000, nil)
	method := ABI.Methods[SetWithdrawAddressMethod]
	withdrawer := sdk.AccAddress(caller.Bytes()).String()

	for _, returned := range []error{
		distributiontypes.ErrSetWithdrawAddrDisabled,
		errors.Wrap(distributiontypes.ErrSetWithdrawAddrDisabled, "message changed"),
	} {
		p := testDistributionPrecompile(nil, &distributionMsgServerStub{err: returned})
		_, err := p.SetWithdrawAddress(distributionTestContext(), contract, nil, &method, []interface{}{caller, withdrawer})
		require.Error(t, err)
		revertData := err.(cmn.RevertDataCarrier).RevertData()
		selector := revertData[:4]
		require.Equal(t, distributionErrorSelector(SolidityErrDistributionSetWithdrawAddressDisabled), revertData)
		require.NotEqual(t, distributionErrorSelector(cmn.SolidityErrMsgServerFailed), selector)
		require.NotEqual(t, distributionErrorSelector(cmn.SolidityErrUnmappedCosmosError), selector)
	}
}

func testDistributionPrecompile(query distributiontypes.QueryServer, msg distributiontypes.MsgServer) Precompile {
	return Precompile{
		ABI:                   ABI,
		distributionQuerier:   query,
		distributionMsgServer: msg,
		addrCdc:               evmaddress.NewEvmCodec("cosmos"),
	}
}

func distributionTestContext() sdk.Context {
	return sdk.Context{}.WithLogger(log.NewNopLogger())
}

func TestDistributionCustomQueryServerTerminalErrors(t *testing.T) {
	method := ABI.Methods[ValidatorOutstandingRewardsMethod]
	validator := sdk.ValAddress(common.HexToAddress("0x100").Bytes()).String()
	for _, input := range []error{vm.ErrOutOfGas, fmt.Errorf("outer: %w", vm.ErrOutOfGas), precompiletest.StatusRevert{}, fmt.Errorf("outer: %w", precompiletest.StatusRevert{})} {
		p := testDistributionPrecompile(&distributionQueryServerStub{err: input}, nil)
		_, err := p.ValidatorOutstandingRewards(distributionTestContext(), nil, &method, []interface{}{validator})
		require.Equal(t, input, err)
	}
}

func TestDistributionUnmappedCallSitesPreserveRevertWithoutWarning(t *testing.T) {
	caller := common.HexToAddress("0x100")
	contract := vm.NewContract(caller, common.HexToAddress(vmtypes.DistributionPrecompileAddress), uint256.NewInt(0), 100_000, nil)
	queryMethod := ABI.Methods[ValidatorOutstandingRewardsMethod]
	msgMethod := ABI.Methods[SetWithdrawAddressMethod]
	validator := sdk.ValAddress(caller.Bytes()).String()
	withdrawer := sdk.AccAddress(caller.Bytes()).String()
	expected := cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrUnmappedCosmosError, "distribution-phase-three-drift", uint32(77))
	for _, returned := range []error{errSyntheticDistributionDrift, fmt.Errorf("private wrapper: %w", errSyntheticDistributionDrift)} {
		p := testDistributionPrecompile(&distributionQueryServerStub{err: returned}, &distributionMsgServerStub{err: returned})
		for _, tc := range []struct {
			name string
			call func(sdk.Context) ([]byte, error)
		}{
			{"query", func(ctx sdk.Context) ([]byte, error) {
				return p.ValidatorOutstandingRewards(ctx, nil, &queryMethod, []interface{}{validator})
			}},
			{"message", func(ctx sdk.Context) ([]byte, error) {
				return p.SetWithdrawAddress(ctx, contract, nil, &msgMethod, []interface{}{caller, withdrawer})
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var output bytes.Buffer
				ctx := sdk.Context{}.WithLogger(log.NewLogger(&output, log.OutputJSONOption()))
				result, err := tc.call(ctx)
				require.Nil(t, result)
				require.Error(t, err)
				require.Equal(t, expected.(cmn.RevertDataCarrier).RevertData(), err.(cmn.RevertDataCarrier).RevertData())
				require.Empty(t, output.String(), "unmapped warnings were intentionally removed")
			})
		}
	}
}
