package distribution

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	evmaddress "github.com/cosmos/evm/encoding/address"
	cmn "github.com/cosmos/evm/precompiles/common"
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
