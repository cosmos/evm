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
		args           []interface{}
		wantErr        bool
		errMsg         string
		wantDepositor  string
		wantProposalID uint64
	}{
		{
			name:           "valid",
			args:           []interface{}{depositorAddr, proposalID, validCoins},
			wantErr:        false,
			wantDepositor:  expectedDepositorAddr,
			wantProposalID: proposalID,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 3, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{depositorAddr, proposalID, validCoins, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 3, 4),
		},
		{
			name:    "invalid depositor type",
			args:    []interface{}{"not-an-address", proposalID, validCoins},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidDepositor, "not-an-address"),
		},
		{
			name:    "empty depositor address",
			args:    []interface{}{common.Address{}, proposalID, validCoins},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidDepositor, common.Address{}),
		},
		{
			name:    "invalid proposal ID type",
			args:    []interface{}{depositorAddr, "not-a-uint64", validCoins},
			wantErr: true,
			errMsg:  "invalid proposal id",
		},
		{
			name:    "invalid coins",
			args:    []interface{}{depositorAddr, proposalID, "invalid-coins"},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidDeposits, "deposit arg"),
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
		args           []interface{}
		wantErr        bool
		errMsg         string
		wantProposer   string
		wantProposalID uint64
	}{
		{
			name:           "valid",
			args:           []interface{}{proposerAddr, proposalID},
			wantErr:        false,
			wantProposer:   expectedProposerAddr,
			wantProposalID: proposalID,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{proposerAddr, proposalID, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 3),
		},
		{
			name:    "invalid proposer type",
			args:    []interface{}{"not-an-address", proposalID},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidProposer, "not-an-address"),
		},
		{
			name:    "empty proposer address",
			args:    []interface{}{common.Address{}, proposalID},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidProposer, common.Address{}),
		},
		{
			name:    "invalid proposal ID type",
			args:    []interface{}{proposerAddr, "not-a-uint64"},
			wantErr: true,
			errMsg:  "invalid proposal id",
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
		args           []interface{}
		wantErr        bool
		errMsg         string
		wantVoter      string
		wantProposalID uint64
		wantOption     uint8
		wantMetadata   string
	}{
		{
			name:           "valid",
			args:           []interface{}{voterAddr, proposalID, option, metadata},
			wantErr:        false,
			wantVoter:      expectedVoterAddr,
			wantProposalID: proposalID,
			wantOption:     option,
			wantMetadata:   metadata,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 4, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{voterAddr, proposalID, option, metadata, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 4, 5),
		},
		{
			name:    "invalid voter type",
			args:    []interface{}{"not-an-address", proposalID, option, metadata},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidVoter, "not-an-address"),
		},
		{
			name:    "empty voter address",
			args:    []interface{}{common.Address{}, proposalID, option, metadata},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidVoter, common.Address{}),
		},
		{
			name:    "invalid proposal ID type",
			args:    []interface{}{voterAddr, "not-a-uint64", option, metadata},
			wantErr: true,
			errMsg:  "invalid proposal id",
		},
		{
			name:    "invalid option type",
			args:    []interface{}{voterAddr, proposalID, "not-a-uint8", metadata},
			wantErr: true,
			errMsg:  "invalid option",
		},
		{
			name:    "invalid metadata type",
			args:    []interface{}{voterAddr, proposalID, option, 123},
			wantErr: true,
			errMsg:  "invalid metadata",
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
		args           []interface{}
		wantErr        bool
		errMsg         string
		wantVoter      string
		wantProposalID uint64
	}{
		{
			name:           "valid",
			args:           []interface{}{proposalID, voterAddr},
			wantErr:        false,
			wantVoter:      expectedVoterAddr,
			wantProposalID: proposalID,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{proposalID, voterAddr, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 3),
		},
		{
			name:    "invalid proposal ID type",
			args:    []interface{}{"not-a-uint64", voterAddr},
			wantErr: true,
			errMsg:  "invalid proposal id",
		},
		{
			name:    "invalid voter type",
			args:    []interface{}{proposalID, "not-an-address"},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidVoter, "not-an-address"),
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
		args           []interface{}
		wantErr        bool
		errMsg         string
		wantDepositor  string
		wantProposalID uint64
	}{
		{
			name:           "valid",
			args:           []interface{}{proposalID, depositorAddr},
			wantErr:        false,
			wantDepositor:  expectedDepositorAddr,
			wantProposalID: proposalID,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{proposalID, depositorAddr, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 3),
		},
		{
			name:    "invalid proposal ID type",
			args:    []interface{}{"not-a-uint64", depositorAddr},
			wantErr: true,
			errMsg:  "invalid proposal id",
		},
		{
			name:    "invalid depositor type",
			args:    []interface{}{proposalID, "not-an-address"},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidDepositor, "not-an-address"),
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
