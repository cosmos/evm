package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreinstall_Validate(t *testing.T) {
	tests := []struct {
		name       string
		preinstall Preinstall
		errorMsg   string
	}{
		{
			name: "valid preinstall with 0x prefix",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "0x1234567890123456789012345678901234567890",
				Code:    "0x608060405234801561001057600080fd5b50",
			},
			errorMsg: "",
		},
		{
			name: "valid preinstall without 0x prefix",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "1234567890123456789012345678901234567890",
				Code:    "608060405234801561001057600080fd5b50",
			},
			errorMsg: "",
		},
		{
			name: "valid preinstall with uppercase hex",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "0xABCDEF1234567890123456789012345678901234",
				Code:    "0x608060405234801561001057600080FD5B50",
			},
			errorMsg: "",
		},
		{
			name: "valid preinstall with mixed case hex",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "0xaBcDeF1234567890123456789012345678901234",
				Code:    "0x608060405234801561001057600080Fd5b50",
			},
			errorMsg: "",
		},
		{
			name: "empty address",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "",
				Code:    "0x608060405234801561001057600080fd5b50",
			},
			errorMsg: "preinstall address cannot be empty",
		},
		{
			name: "empty code",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "0x1234567890123456789012345678901234567890",
				Code:    "",
			},
			errorMsg: "preinstall code cannot be empty",
		},
		{
			name: "invalid address - not hex",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "0xGHIJ567890123456789012345678901234567890",
				Code:    "0x608060405234801561001057600080fd5b50",
			},
			errorMsg: "preinstall address \"0xGHIJ567890123456789012345678901234567890\" is not a valid hex address",
		},
		{
			name: "invalid address - too short",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "0x1234",
				Code:    "0x608060405234801561001057600080fd5b50",
			},
			errorMsg: "preinstall address \"0x1234\" is not a valid hex address",
		},
		{
			name: "invalid address - too long",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "0x123456789012345678901234567890123456789012",
				Code:    "0x608060405234801561001057600080fd5b50",
			},
			errorMsg: "preinstall address \"0x123456789012345678901234567890123456789012\" is not a valid hex address",
		},
		{
			name: "invalid code - not hex",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "0x1234567890123456789012345678901234567890",
				Code:    "0xGHIJ60405234801561001057600080fd5b50",
			},
			errorMsg: "preinstall code \"0xGHIJ60405234801561001057600080fd5b50\" is not a valid hex string",
		},
		{
			name: "invalid code - odd length without 0x",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "0x1234567890123456789012345678901234567890",
				Code:    "60806040523480156100057600080fd5b50",
			},
			errorMsg: "preinstall code \"60806040523480156100057600080fd5b50\" is not a valid hex string",
		},
		{
			name: "invalid code - empty code hash",
			preinstall: Preinstall{
				Name:    "Test Contract",
				Address: "0x1234567890123456789012345678901234567890",
				Code:    "0x",
			},
			errorMsg: "preinstall code \"0x\" has empty code hash",
		},
		{
			name: "valid preinstall with empty name (name not validated)",
			preinstall: Preinstall{
				Name:    "",
				Address: "0x1234567890123456789012345678901234567890",
				Code:    "0x608060405234801561001057600080fd5b50",
			},
			errorMsg: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.preinstall.Validate()
			if tt.errorMsg == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorMsg)
			}
		})
	}
}

func TestDefaultPreinstalls_Validate(t *testing.T) {
	// Test that all default preinstalls are valid
	for i, preinstall := range DefaultPreinstalls {
		t.Run(preinstall.Name, func(t *testing.T) {
			err := preinstall.Validate()
			require.NoError(t, err, "DefaultPreinstalls[%d] (%s) should be valid", i, preinstall.Name)
		})
	}
}
