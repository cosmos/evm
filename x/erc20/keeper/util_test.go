package keeper

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/types"
)

func TestValidateApprovalEventDoesNotExist(t *testing.T) {
	tests := []struct {
		name        string
		res         *types.MsgEthereumTxResponse
		expectError bool
	}{
		{
			name: "empty logs",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{},
			},
			expectError: false,
		},
		{
			name: "no approval event",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Topics: []string{"0x1234567890abcdef"},
					},
				},
			},
			expectError: false,
		},
		{
			name: "has approval event",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Topics: []string{logApprovalSigHash.Hex()},
					},
				},
			},
			expectError: true,
		},
		{
			name: "approval event among others",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Topics: []string{"0x1234567890abcdef"},
					},
					{
						Topics: []string{logApprovalSigHash.Hex()},
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApprovalEventDoesNotExist(tt.res.Logs)
			if tt.expectError {
				require.Error(t, err)
				require.Contains(t, err.Error(), "unexpected Approval event")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateTransferEventExists(t *testing.T) {
	tests := []struct {
		name         string
		res          *types.MsgEthereumTxResponse
		tokenAddress common.Address
		expectError  string
	}{
		{
			name: "empty logs",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{},
			},
			expectError: "expected Transfer event",
		},
		{
			name: "no transfer event",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Topics: []string{"0x1234567890abcdef"},
					},
				},
			},
			tokenAddress: common.HexToAddress("0x1234567890abcdef"),
			expectError:  "expected Transfer event",
		},
		{
			name: "has transfer event from different address",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Address: common.HexToAddress("0x1234567890abcdef").Hex(),
						Topics:  []string{logTransferSigHash.Hex()},
					},
				},
			},
			tokenAddress: common.HexToAddress("fedcba0987654321"),
			expectError:  "Transfer event from unexpected address",
		},
		{
			name: "has duplicate transfer event",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Address: common.HexToAddress("0x1234567890abcdef").Hex(),
						Topics:  []string{logTransferSigHash.Hex()},
					},
					{
						Address: common.HexToAddress("0x1234567890abcdef").Hex(),
						Topics:  []string{logTransferSigHash.Hex()},
					},
				},
			},
			tokenAddress: common.HexToAddress("0x1234567890abcdef"),
			expectError:  "duplicate Transfer event",
		},
		{
			name: "has transfer event",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Address: common.HexToAddress("0x1234567890abcdef").Hex(),
						Topics:  []string{logTransferSigHash.Hex()},
					},
				},
			},
			tokenAddress: common.HexToAddress("0x1234567890abcdef"),
			expectError:  "",
		},
		{
			name: "transfer event among others",
			res: &types.MsgEthereumTxResponse{
				Logs: []*types.Log{
					{
						Address: common.HexToAddress("0x1234567890abcdef").Hex(),
						Topics:  []string{"0x1234567890abcdef"},
					},
					{
						Address: common.HexToAddress("0x1234567890abcdef").Hex(),
						Topics:  []string{logTransferSigHash.Hex()},
					},
				},
			},
			tokenAddress: common.HexToAddress("0x1234567890abcdef"),
			expectError:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTransferEventExists(tt.res.Logs, tt.tokenAddress)
			if tt.expectError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.expectError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
