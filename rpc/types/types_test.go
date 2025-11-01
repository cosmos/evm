package types_test

import (
	"encoding/json"
	"maps"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	rpc "github.com/cosmos/evm/rpc/types"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types/mocks"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type precompileContract struct{}

func (p *precompileContract) Address() common.Address { return common.Address{} }

func (p *precompileContract) RequiredGas(input []byte) uint64 { return 0 }

func (p *precompileContract) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return nil, nil
}

func TestApply(t *testing.T) {
	emptyTxConfig := statedb.NewEmptyTxConfig()
	db := statedb.New(sdk.Context{}, mocks.NewEVMKeeper(), emptyTxConfig)
	precompiles := map[common.Address]vm.PrecompiledContract{
		common.BytesToAddress([]byte{0x1}): &precompileContract{},
		common.BytesToAddress([]byte{0x2}): &precompileContract{},
	}
	bytes2Addr := func(b []byte) *common.Address {
		a := common.BytesToAddress(b)
		return &a
	}
	testCases := map[string]struct {
		overrides           *rpc.StateOverride
		expectedPrecompiles map[common.Address]struct{}
		fail                bool
	}{
		"move to already touched precompile": {
			overrides: &rpc.StateOverride{
				common.BytesToAddress([]byte{0x1}): {
					Code:             &hexutil.Bytes{0xff},
					MovePrecompileTo: bytes2Addr([]byte{0x2}),
				},
				common.BytesToAddress([]byte{0x2}): {
					Code: &hexutil.Bytes{0x00},
				},
			},
			fail: true,
		},
		"move non-precompile": {
			overrides: &rpc.StateOverride{
				common.BytesToAddress([]byte{0x1}): {
					Code:             &hexutil.Bytes{0xff},
					MovePrecompileTo: bytes2Addr([]byte{0xff}),
				},
				common.BytesToAddress([]byte{0x3}): {
					Code:             &hexutil.Bytes{0x00},
					MovePrecompileTo: bytes2Addr([]byte{0xfe}),
				},
			},
			fail: true,
		},
		"move two precompiles": {
			overrides: &rpc.StateOverride{
				common.BytesToAddress([]byte{0x1}): {
					Code:             &hexutil.Bytes{0xff},
					MovePrecompileTo: bytes2Addr([]byte{0xff}),
				},
				common.BytesToAddress([]byte{0x2}): {
					Code:             &hexutil.Bytes{0x00},
					MovePrecompileTo: bytes2Addr([]byte{0xfe}),
				},
			},
			expectedPrecompiles: map[common.Address]struct{}{
				common.BytesToAddress([]byte{0xfe}): {},
				common.BytesToAddress([]byte{0xff}): {},
			},
			fail: false,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			cpy := maps.Clone(precompiles)
			err := tc.overrides.Apply(db, cpy)
			if tc.fail {
				if err == nil {
					t.Errorf("%s: want error, have nothing", name)
				}
				return
			}
			if err != nil {
				t.Errorf("%s: want no error, have %v", name, err)
				return
			}
			if len(cpy) != len(tc.expectedPrecompiles) {
				t.Errorf("%s: precompile mismatch, want %d, have %d", name, len(tc.expectedPrecompiles), len(cpy))
			}
			for k := range tc.expectedPrecompiles {
				if _, ok := cpy[k]; !ok {
					t.Errorf("%s: precompile not found: %s", name, k.String())
				}
			}
		})
	}
}

func TestParseOverrides(t *testing.T) {
	tests := []struct {
		name                    string
		input                   string
		expectedEVMOverrides    bool
		expectedCosmosOverrides int
		expectError             bool
		isPrecompile            bool
	}{
		{
			name: "Standard EVM overrides (backward compatibility)",
			input: `{
				"0x1234567890abcdef1234567890abcdef12345678": {
					"stateDiff": {
						"0x0000000000000000000000000000000000000000000000000000000000000001": "0x0000000000000000000000000000000000000000000000000000000000000064"
					}
				}
			}`,
			expectedEVMOverrides:    true,
			expectedCosmosOverrides: 0,
			isPrecompile:            false,
		},
		{
			name: "Dynamic precompile with aligned cosmos overrides in state field",
			input: `{
				"0x1234567890abcdef1234567890abcdef12345678": {
					"state": "W3sibmFtZSI6ImJhbmsiLCJlbnRyaWVzIjpbeyJrZXkiOiJZbUZ1YXlCclpYaz0iLCJ2YWx1ZSI6IllXRnVhM0IyWVd4MVpRPT0iLCJkZWxldGUiOmZhbHNlfV19XQ=="
				}
			}`,
			expectedEVMOverrides:    false,
			expectedCosmosOverrides: 1,
			isPrecompile:            true,
		},
		{
			name: "Dynamic precompile with aligned cosmos overrides in stateDiff field",
			input: `{
				"0x1234567890abcdef1234567890abcdef12345678": {
					"stateDiff": "W3sibmFtZSI6ImJhbmsiLCJlbnRyaWVzIjpbeyJrZXkiOiJZbUZ1YXlCclpYaz0iLCJ2YWx1ZSI6IllXRnVhM0IyWVd4MVpRPT0iLCJkZWxldGUiOmZhbHNlfV19XQ=="
				}
			}`,
			expectedEVMOverrides:    false,
			expectedCosmosOverrides: 1,
			isPrecompile:            true,
		},
		{
			name:                    "Empty overrides",
			input:                   `{}`,
			expectedEVMOverrides:    false,
			expectedCosmosOverrides: 0,
			isPrecompile:            false,
		},
		{
			name:         "Invalid JSON",
			input:        `{invalid json}`,
			expectError:  true,
			isPrecompile: false,
		},
		{
			name:                    "Nil input",
			input:                   "",
			expectedEVMOverrides:    false,
			expectedCosmosOverrides: 0,
			expectError:             false,
			isPrecompile:            false,
		},
		{
			name: "Dynamic precompile with empty stateType returns empty cosmos overrides",
			input: `{
				"0x1234567890abcdef1234567890abcdef12345678": {
					"": "some_value"
				}
			}`,
			expectedEVMOverrides:    false,
			expectedCosmosOverrides: 0,
			isPrecompile:            true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var rawMessage *json.RawMessage
			if tc.input != "" {
				msg := json.RawMessage(tc.input)
				rawMessage = &msg
			}
			evmOverrides, cosmosOverrides, err := rpc.ParseOverrides(rawMessage, tc.isPrecompile)
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tc.expectedEVMOverrides {
				require.NotNil(t, evmOverrides)
				require.Len(t, *evmOverrides, 1)
			} else {
				require.Nil(t, evmOverrides)
			}
			if tc.expectedCosmosOverrides > 0 {
				require.NotNil(t, cosmosOverrides)
				require.Len(t, cosmosOverrides, tc.expectedCosmosOverrides)
			} else {
				require.Len(t, cosmosOverrides, 0)
			}
		})
	}
}
