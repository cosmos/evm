package gov

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	evmaddress "github.com/cosmos/evm/encoding/address"
	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestNewMsgDeposit(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	depositorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validCoins := []cmn.Coin{{Denom: "stake", Amount: big.NewInt(1000)}}
	proposalID := uint64(1)

	expectedDepositorAddr, err := addrCodec.BytesToString(depositorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           DepositCall
		wantErr        bool
		errMsg         string
		wantDepositor  string
		wantProposalID uint64
	}{
		{
			name: "valid",
			args: DepositCall{
				Depositor:  depositorAddr,
				ProposalId: proposalID,
				Amount:     validCoins,
			},
			wantErr:        false,
			wantDepositor:  expectedDepositorAddr,
			wantProposalID: proposalID,
		},
		{
			name: "empty depositor address",
			args: DepositCall{
				Depositor:  common.Address{},
				ProposalId: proposalID,
				Amount:     validCoins,
			},
			wantErr:        true,
			errMsg:         fmt.Sprintf(ErrInvalidDepositor, common.Address{}),
			wantDepositor:  expectedDepositorAddr,
			wantProposalID: proposalID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgDeposit(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, depositorAddr, returnAddr)
				require.Equal(t, tt.wantDepositor, msg.Depositor)
				require.Equal(t, tt.wantProposalID, msg.ProposalId)
				require.NotEmpty(t, msg.Amount)
			}
		})
	}
}

func TestNewMsgCancelProposal(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	proposerAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	proposalID := uint64(1)

	expectedProposerAddr, err := addrCodec.BytesToString(proposerAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           CancelProposalCall
		wantErr        bool
		errMsg         string
		wantProposer   string
		wantProposalID uint64
	}{
		{
			name: "valid",
			args: CancelProposalCall{
				Proposer:   proposerAddr,
				ProposalId: proposalID,
			},
			wantErr:        false,
			wantProposer:   expectedProposerAddr,
			wantProposalID: proposalID,
		},
		{
			name: "empty proposer address",
			args: CancelProposalCall{
				Proposer:   common.Address{},
				ProposalId: proposalID,
			},
			wantErr:        true,
			errMsg:         fmt.Sprintf(ErrInvalidProposer, common.Address{}),
			wantProposer:   expectedProposerAddr,
			wantProposalID: proposalID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgCancelProposal(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, proposerAddr, returnAddr)
				require.Equal(t, tt.wantProposer, msg.Proposer)
				require.Equal(t, tt.wantProposalID, msg.ProposalId)
			}
		})
	}
}

func TestNewMsgVote(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	voterAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	proposalID := uint64(1)
	option := uint8(1)
	metadata := "test-metadata"

	expectedVoterAddr, err := addrCodec.BytesToString(voterAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           VoteCall
		wantErr        bool
		errMsg         string
		wantVoter      string
		wantProposalID uint64
		wantOption     uint8
		wantMetadata   string
	}{
		{
			name: "valid",
			args: VoteCall{
				Voter:      voterAddr,
				ProposalId: proposalID,
				Option:     option,
				Metadata:   metadata,
			},
			wantErr:        false,
			wantVoter:      expectedVoterAddr,
			wantProposalID: proposalID,
			wantOption:     option,
			wantMetadata:   metadata,
		},
		{
			name: "empty voter address",
			args: VoteCall{
				Voter:      common.Address{},
				ProposalId: proposalID,
				Option:     option,
				Metadata:   metadata,
			},
			wantErr:        true,
			errMsg:         fmt.Sprintf(ErrInvalidVoter, common.Address{}),
			wantVoter:      expectedVoterAddr,
			wantProposalID: proposalID,
			wantOption:     option,
			wantMetadata:   metadata,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgVote(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, voterAddr, returnAddr)
				require.Equal(t, tt.wantVoter, msg.Voter)
				require.Equal(t, tt.wantProposalID, msg.ProposalId)
				require.Equal(t, tt.wantOption, uint8(msg.Option)) //nolint:gosec // doesn't matter here
				require.Equal(t, tt.wantMetadata, msg.Metadata)
			}
		})
	}
}

func TestParseVoteArgs(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	voterAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	proposalID := uint64(1)

	expectedVoterAddr, err := addrCodec.BytesToString(voterAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           GetVoteCall
		wantErr        bool
		errMsg         string
		wantVoter      string
		wantProposalID uint64
	}{
		{
			name: "valid",
			args: GetVoteCall{
				ProposalId: proposalID,
				Voter:      voterAddr,
			},
			wantErr:        false,
			wantVoter:      expectedVoterAddr,
			wantProposalID: proposalID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseVoteArgs(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				require.Equal(t, tt.wantVoter, req.Voter)
				require.Equal(t, tt.wantProposalID, req.ProposalId)
			}
		})
	}
}

func TestParseDepositArgs(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	depositorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	proposalID := uint64(1)

	expectedDepositorAddr, err := addrCodec.BytesToString(depositorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           GetDepositCall
		wantErr        bool
		errMsg         string
		wantDepositor  string
		wantProposalID uint64
	}{
		{
			name: "valid",
			args: GetDepositCall{
				ProposalId: proposalID,
				Depositor:  depositorAddr,
			},
			wantErr:        false,
			wantDepositor:  expectedDepositorAddr,
			wantProposalID: proposalID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := ParseDepositArgs(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				require.Equal(t, tt.wantDepositor, req.Depositor)
				require.Equal(t, tt.wantProposalID, req.ProposalId)
			}
		})
	}
}
