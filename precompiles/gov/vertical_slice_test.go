package gov

import (
	"context"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	evmaddress "github.com/cosmos/evm/encoding/address"
	cmn "github.com/cosmos/evm/precompiles/common"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/errors"
	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

type govMsgServerStub struct {
	govv1.UnimplementedMsgServer
	err error
}

func (m govMsgServerStub) Vote(context.Context, *govv1.MsgVote) (*govv1.MsgVoteResponse, error) {
	return nil, m.err
}

type govQueryServerStub struct {
	govv1.UnimplementedQueryServer
	err error
}

func (q govQueryServerStub) Proposal(context.Context, *govv1.QueryProposalRequest) (*govv1.QueryProposalResponse, error) {
	return nil, q.err
}

func TestGovMsgRegisteredErrorUsesConcreteSelector(t *testing.T) {
	caller := common.HexToAddress("0x100")
	contract := vm.NewContract(caller, common.HexToAddress(vmtypes.GovPrecompileAddress), uint256.NewInt(0), 100_000, nil)
	method := ABI.Methods[VoteMethod]
	args := []interface{}{caller, uint64(1), uint8(govv1.OptionYes), "metadata"}

	for _, returned := range []error{
		govtypes.ErrInvalidVote,
		errors.Wrap(govtypes.ErrInvalidVote, "message changed"),
		fmt.Errorf("outer: %w", govtypes.ErrInvalidVote),
	} {
		p := testGovPrecompile(&govMsgServerStub{err: returned}, nil)
		_, err := p.Vote(govTestContext(), contract, nil, &method, args)
		require.Error(t, err)
		revertData := err.(cmn.RevertDataCarrier).RevertData()
		selector := revertData[:4]
		require.Equal(t, govErrorSelector(SolidityErrGovInvalidVote), revertData)
		require.NotEqual(t, govErrorSelector(cmn.SolidityErrMsgServerFailed), selector)
		require.NotEqual(t, govErrorSelector(cmn.SolidityErrUnmappedCosmosError), selector)
	}
}

func TestGovUnlistedQueryGRPCStatusRemainsQueryFailed(t *testing.T) {
	method := ABI.Methods[GetProposalMethod]
	for _, message := range []string{"proposal missing", "upstream wording changed"} {
		p := testGovPrecompile(nil, &govQueryServerStub{err: status.Error(codes.NotFound, message)})
		_, err := p.GetProposal(govTestContext(), &method, nil, []interface{}{uint64(1)})
		require.Error(t, err)
		selector := err.(cmn.RevertDataCarrier).RevertData()[:4]
		require.Equal(t, govErrorSelector(cmn.SolidityErrQueryFailed), selector)
		require.NotEqual(t, govErrorSelector(SolidityErrGovInvalidProposal), selector)
		require.NotEqual(t, govErrorSelector(cmn.SolidityErrUnmappedCosmosError), selector)
	}
}

func testGovPrecompile(msg govv1.MsgServer, query govv1.QueryServer) Precompile {
	return Precompile{
		ABI:          ABI,
		govMsgServer: msg,
		govQuerier:   query,
		addrCdc:      evmaddress.NewEvmCodec("cosmos"),
	}
}

func govTestContext() sdk.Context {
	return sdk.Context{}.WithLogger(log.NewNopLogger())
}
