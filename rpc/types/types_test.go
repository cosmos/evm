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
	evmtypes "github.com/cosmos/evm/x/vm/types"
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
		},
		{
			name: "Combined overrides with both EVM and Cosmos",
			input: `{
				"stateOverride": {
					"0x1234567890abcdef1234567890abcdef12345678": {
						"stateDiff": {
							"0x0000000000000000000000000000000000000000000000000000000000000001": "0x0000000000000000000000000000000000000000000000000000000000000064"
						}
					}
				},
				"cosmosStateOverrides": [
					{
						"name": "bank",
						"entries": [
							{
								"key": "YmFuayBrZXk=",
								"value": "YmFuayB2YWx1ZQ==",
								"delete": false
							}
						]
					}
				]
			}`,
			expectedEVMOverrides:    true,
			expectedCosmosOverrides: 1,
		},
		{
			name: "Cosmos overrides only",
			input: `{
				"cosmosStateOverrides": [
					{
						"name": "bank",
						"entries": [
							{
								"key": "YmFuayBrZXk=",
								"value": "YmFuayB2YWx1ZQ==",
								"delete": false
							}
						]
					},
					{
						"name": "staking", 
						"entries": [
							{
								"key": "c3Rha2luZyBrZXk=",
								"value": "c3Rha2luZyB2YWx1ZQ==",
								"delete": false
							}
						]
					}
				]
			}`,
			expectedEVMOverrides:    false,
			expectedCosmosOverrides: 2,
		},
		{
			name:                    "Empty overrides",
			input:                   `{}`,
			expectedEVMOverrides:    false,
			expectedCosmosOverrides: 0,
		},
		{
			name:        "Invalid JSON",
			input:       `{invalid json}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawMessage := json.RawMessage(tt.input)
			evmOverrides, cosmosOverrides, err := rpc.ParseOverrides(&rawMessage)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.expectedEVMOverrides {
				require.NotNil(t, evmOverrides, "Expected EVM overrides to be present")
				require.Greater(t, len(*evmOverrides), 0, "Expected non-empty EVM overrides")
			} else {
				require.Nil(t, evmOverrides, "Expected no EVM overrides")
			}

			require.Len(t, cosmosOverrides, tt.expectedCosmosOverrides, "Cosmos overrides count mismatch")
			if len(cosmosOverrides) > 0 {
				for i, override := range cosmosOverrides {
					require.NotEmpty(t, override.Name, "Store name should not be empty for override %d", i)
					require.Greater(t, len(override.Entries), 0, "Entries should not be empty for override %d", i)

					for j, entry := range override.Entries {
						require.NotEmpty(t, entry.Key, "Key should not be empty for override %d entry %d", i, j)
						require.NotEmpty(t, entry.Value, "Value should not be empty for override %d entry %d", i, j)
					}
				}
			}
		})
	}
}

func TestParseOverrides_NilInput(t *testing.T) {
	evmOverrides, cosmosOverrides, err := rpc.ParseOverrides(nil)
	require.NoError(t, err)
	require.Nil(t, evmOverrides)
	require.Nil(t, cosmosOverrides)
}

func TestCombinedOverrides_JSON(t *testing.T) {
	original := rpc.CombinedOverrides{
		StateOverride: &rpc.StateOverride{},
		CosmosStateOverrides: []evmtypes.StoreStateDiff{
			{
				Name: "bank",
				Entries: []evmtypes.StateEntry{
					{
						Key:    []byte("test key"),
						Value:  []byte("test value"),
						Delete: false,
					},
				},
			},
		},
	}
	data, err := json.Marshal(original)
	require.NoError(t, err)
	require.NotEmpty(t, data)
	var unmarshaled rpc.CombinedOverrides
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)
	require.Len(t, unmarshaled.CosmosStateOverrides, 1)
	require.Equal(t, "bank", unmarshaled.CosmosStateOverrides[0].Name)
	require.Len(t, unmarshaled.CosmosStateOverrides[0].Entries, 1)
	require.Equal(t, []byte("test key"), unmarshaled.CosmosStateOverrides[0].Entries[0].Key)
	require.Equal(t, []byte("test value"), unmarshaled.CosmosStateOverrides[0].Entries[0].Value)
	require.False(t, unmarshaled.CosmosStateOverrides[0].Entries[0].Delete)
}
