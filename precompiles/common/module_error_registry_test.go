package common

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

const moduleErrorRegistryABIJSON = `[
	{"type":"error","name":"ModuleFailure","inputs":[{"name":"label","type":"string"},{"name":"amount","type":"uint256"}]},
	{"type":"error","name":"LabelFailure","inputs":[{"name":"label","type":"string"}]},
	{"type":"error","name":"PointerFailure","inputs":[{"name":"label","type":"string"}]},
	{"type":"error","name":"AFailure","inputs":[]},
	{"type":"error","name":"BFailure","inputs":[]},
	{"type":"error","name":"SharedFailure","inputs":[]},
	{"type":"error","name":"OtherFailure","inputs":[{"name":"value","type":"uint256"}]},
	{"type":"error","name":"NoArgsFailure","inputs":[]}
]`

const (
	registryPointerLabel = "pointer"
	registryValueLabel   = "value"
)

type registryValueError struct {
	Label  string
	Amount *big.Int
}

func (err registryValueError) Error() string { return err.Label }

type registryPointerError struct {
	Label string
}

func (err *registryPointerError) Error() string { return err.Label }

type registryErrorA struct{}

func (registryErrorA) Error() string { return "a" }

type registryErrorB struct{}

func (registryErrorB) Error() string { return "b" }

type registrySameTargetA struct{}

func (registrySameTargetA) Error() string { return "same-a" }

type registrySameTargetB struct{}

func (registrySameTargetB) Error() string { return "same-b" }

type registryNilInvariantError struct{}

func (registryNilInvariantError) Error() string { return "nil invariant" }
func (registryNilInvariantError) Unwrap() error { return nil }

type registryCustomValueAsError struct {
	Value registryValueError
}

func (err registryCustomValueAsError) Error() string { return "custom value As" }

func (err registryCustomValueAsError) As(target any) bool {
	value, ok := target.(*registryValueError)
	if !ok {
		return false
	}
	*value = err.Value
	return true
}

type registryCustomPointerAsError struct {
	Value registryPointerError
}

func (err registryCustomPointerAsError) Error() string { return "custom pointer As" }

func (err registryCustomPointerAsError) As(target any) bool {
	pointer, ok := target.(**registryPointerError)
	if !ok {
		return false
	}
	value := err.Value
	*pointer = &value
	return true
}

func TestModuleErrorRegistryTranslatesValueAndPointerForms(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	mapping := NewModuleErrorMapping[registryValueError]("ModuleFailure", func(err registryValueError) []any {
		return []any{err.Label, err.Amount}
	})
	registry := MustNewModuleErrorRegistry(contractABI, mapping)
	want := registryValueError{Label: registryValueLabel, Amount: big.NewInt(7)}

	tests := map[string]error{
		registryValueLabel:   want,
		registryPointerLabel: &want,
		"wrapped value":      fmt.Errorf("wrapped: %w", want),
		"wrapped pointer":    fmt.Errorf("wrapped: %w", &want),
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			revert, matched := registry.Translate(input)
			require.True(t, matched)
			requireModuleRevert(t, contractABI, revert, "ModuleFailure", want.Label, want.Amount)
		})
	}
}

func TestModuleErrorRegistryTranslatesPointerReceiverOnlyError(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewModuleErrorMapping[registryPointerError]("PointerFailure", func(err registryPointerError) []any {
			return []any{err.Label}
		}),
	)

	tests := []struct {
		name  string
		input error
		label string
	}{
		{name: registryPointerLabel, input: &registryPointerError{Label: registryPointerLabel}, label: registryPointerLabel},
		{name: "wrapped pointer", input: fmt.Errorf("wrapped: %w", &registryPointerError{Label: "wrapped-pointer"}), label: "wrapped-pointer"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			revert, matched := registry.Translate(tc.input)
			require.True(t, matched)
			requireModuleRevert(t, contractABI, revert, "PointerFailure", tc.label)
		})
	}
}

func TestModuleErrorRegistryPreservesCustomAs(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)

	t.Run("value target", func(t *testing.T) {
		registry := MustNewModuleErrorRegistry(contractABI,
			NewModuleErrorMapping[registryValueError]("ModuleFailure", func(err registryValueError) []any {
				return []any{err.Label, err.Amount}
			}),
		)
		input := registryCustomValueAsError{Value: registryValueError{Label: "custom-value", Amount: big.NewInt(9)}}

		revert, matched := registry.Translate(input)
		require.True(t, matched)
		requireModuleRevert(t, contractABI, revert, "ModuleFailure", input.Value.Label, input.Value.Amount)
	})

	t.Run("pointer target", func(t *testing.T) {
		registry := MustNewModuleErrorRegistry(contractABI,
			NewModuleErrorMapping[registryPointerError]("PointerFailure", func(err registryPointerError) []any {
				return []any{err.Label}
			}),
		)
		input := registryCustomPointerAsError{Value: registryPointerError{Label: "custom-pointer"}}

		revert, matched := registry.Translate(input)
		require.True(t, matched)
		requireModuleRevert(t, contractABI, revert, "PointerFailure", input.Value.Label)
	})
}

func TestModuleErrorRegistryUsesDeclarationOrder(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewNoArgsModuleErrorMapping[registryErrorA]("AFailure"),
		NewNoArgsModuleErrorMapping[registryErrorB]("BFailure"),
	)

	revert, matched := registry.Translate(errors.Join(registryErrorB{}, registryErrorA{}))
	require.True(t, matched)
	requireModuleRevert(t, contractABI, revert, "AFailure")
}

func TestModuleErrorRegistryMatchesValueBeforePointerForSameBaseType(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewModuleErrorMapping[registryValueError]("LabelFailure", func(err registryValueError) []any {
			return []any{err.Label}
		}),
	)

	revert, matched := registry.Translate(errors.Join(
		&registryValueError{Label: registryPointerLabel},
		registryValueError{Label: registryValueLabel},
	))
	require.True(t, matched)
	requireModuleRevert(t, contractABI, revert, "LabelFailure", registryValueLabel)
}

func TestModuleErrorRegistryTypedNilPointerReceiverErrorIsUnmatched(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewModuleErrorMapping[registryPointerError]("PointerFailure", func(err registryPointerError) []any {
			return []any{err.Label}
		}),
	)
	var typedNil *registryPointerError
	var input error = typedNil

	revert, matched := registry.Translate(input)
	require.False(t, matched)
	require.Nil(t, revert)
}

func TestModuleErrorRegistryDoesNotRecoverNilValueReceiverInvariant(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewNoArgsModuleErrorMapping[registryNilInvariantError]("NoArgsFailure"),
	)
	var typedNil *registryNilInvariantError
	var input error = typedNil

	require.Panics(t, func() {
		_, _ = registry.Translate(input)
	})
}

func TestModuleErrorRegistryReturnsUnknownAndNilAsUnmatched(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewNoArgsModuleErrorMapping[registryErrorA]("AFailure"),
	)

	for name, input := range map[string]error{"nil": nil, "unknown": errors.New("unknown")} {
		t.Run(name, func(t *testing.T) {
			revert, matched := registry.Translate(input)
			require.False(t, matched)
			require.Nil(t, revert)
		})
	}
}

func TestModuleErrorRegistryPacksArguments(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewModuleErrorMapping[registryValueError]("ModuleFailure", func(err registryValueError) []any {
			return []any{err.Label, err.Amount}
		}),
	)
	input := registryValueError{Label: "packed", Amount: big.NewInt(42)}

	revert, matched := registry.Translate(input)
	require.True(t, matched)
	requireModuleRevert(t, contractABI, revert, "ModuleFailure", input.Label, input.Amount)
}

func TestModuleErrorRegistryUsesErrorStringFallbackForInvalidArguments(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	tests := map[string]func(registryValueError) []any{
		"argument count": func(err registryValueError) []any { return []any{err.Label} },
		"argument type":  func(err registryValueError) []any { return []any{uint64(1), err.Amount} },
	}
	for name, argsFunc := range tests {
		t.Run(name, func(t *testing.T) {
			registry := MustNewModuleErrorRegistry(contractABI,
				NewModuleErrorMapping[registryValueError]("ModuleFailure", argsFunc),
			)

			revert, matched := registry.Translate(registryValueError{Label: "invalid", Amount: big.NewInt(1)})
			require.True(t, matched)
			var carrier RevertDataCarrier
			require.ErrorAs(t, revert, &carrier)
			reason, err := abi.UnpackRevert(carrier.RevertData())
			require.NoError(t, err)
			require.Contains(t, reason, "failed to pack solidity custom error ModuleFailure")
		})
	}
}

func TestModuleErrorRegistryDoesNotRecoverArgsFunctionPanic(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewModuleErrorMapping[registryValueError]("ModuleFailure", func(registryValueError) []any {
			panic("args function panic")
		}),
	)

	require.PanicsWithValue(t, "args function panic", func() {
		_, _ = registry.Translate(registryValueError{})
	})
}

func TestNewNoArgsModuleErrorMapping(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewNoArgsModuleErrorMapping[registryErrorA]("AFailure"),
	)

	revert, matched := registry.Translate(registryErrorA{})
	require.True(t, matched)
	var carrier RevertDataCarrier
	require.ErrorAs(t, revert, &carrier)
	require.Equal(t, errorSelector(contractABI, "AFailure"), carrier.RevertData())
}

func TestModuleErrorRegistryValidation(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	validArgsFunction := func(err registryValueError) []any { return []any{err.Label, err.Amount} }
	tests := map[string]struct {
		mappings []ModuleErrorMapping
		contains string
	}{
		"zero mapping": {
			mappings: []ModuleErrorMapping{{}},
			contains: "requires Solidity error name",
		},
		"empty Solidity error": {
			mappings: []ModuleErrorMapping{NewModuleErrorMapping[registryValueError]("", validArgsFunction)},
			contains: "requires Solidity error name",
		},
		"missing Solidity error": {
			mappings: []ModuleErrorMapping{NewModuleErrorMapping[registryValueError]("MissingFailure", validArgsFunction)},
			contains: "missing ABI error MissingFailure",
		},
		"nil args function": {
			mappings: []ModuleErrorMapping{NewModuleErrorMapping[registryValueError]("ModuleFailure", nil)},
			contains: "non-nil args function",
		},
		"unnamed type": {
			mappings: []ModuleErrorMapping{NewNoArgsModuleErrorMapping[struct{ registryValueError }]("NoArgsFailure")},
			contains: "must be named",
		},
		"duplicate base type": {
			mappings: []ModuleErrorMapping{
				NewModuleErrorMapping[registryValueError]("ModuleFailure", validArgsFunction),
				NewModuleErrorMapping[registryValueError]("LabelFailure", func(err registryValueError) []any { return []any{err.Label} }),
			},
			contains: "duplicate module error mapping",
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewModuleErrorRegistry(contractABI, tc.mappings...)
			require.ErrorContains(t, err, tc.contains)
		})
	}

	validMapping := NewModuleErrorMapping[registryValueError]("ModuleFailure", validArgsFunction)
	require.NoError(t, ValidateModuleErrorRegistry(contractABI, validMapping))
	require.Error(t, ValidateModuleErrorRegistry(contractABI, ModuleErrorMapping{}))
	require.Panics(t, func() { MustNewModuleErrorRegistry(contractABI, ModuleErrorMapping{}) })
}

func TestModuleErrorRegistryRejectsInvalidEffectiveABIAtConstruction(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	first := contractABI.Errors["AFailure"]
	second := contractABI.Errors["BFailure"]
	second.Sig = first.Sig
	contractABI.Errors["BFailure"] = second

	mapping := NewNoArgsModuleErrorMapping[registryErrorA]("AFailure")
	_, err := NewModuleErrorRegistry(contractABI, mapping)
	require.ErrorContains(t, err, "duplicate ABI signature")
	require.ErrorContains(t, ValidateModuleErrorRegistry(contractABI, mapping), "duplicate ABI signature")
}

func TestModuleErrorRegistryAllowsDifferentTypesToShareSolidityError(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry, err := NewModuleErrorRegistry(contractABI,
		NewNoArgsModuleErrorMapping[registrySameTargetA]("SharedFailure"),
		NewNoArgsModuleErrorMapping[registrySameTargetB]("SharedFailure"),
	)
	require.NoError(t, err)

	for _, input := range []error{registrySameTargetA{}, registrySameTargetB{}} {
		revert, matched := registry.Translate(input)
		require.True(t, matched)
		requireModuleRevert(t, contractABI, revert, "SharedFailure")
	}
}

func TestModuleErrorRegistrySnapshotsInputMappings(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	mappings := []ModuleErrorMapping{NewNoArgsModuleErrorMapping[registryErrorA]("AFailure")}
	registry := MustNewModuleErrorRegistry(contractABI, mappings...)
	mappings[0] = NewNoArgsModuleErrorMapping[registryErrorB]("BFailure")

	revert, matched := registry.Translate(registryErrorA{})
	require.True(t, matched)
	requireModuleRevert(t, contractABI, revert, "AFailure")
	require.Equal(t, []ModuleErrorMappingInfo{{GoType: reflect.TypeFor[registryErrorA](), SolidityError: "AFailure"}}, registry.Mappings())
}

func TestModuleErrorRegistryMappingsReturnsCopy(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewNoArgsModuleErrorMapping[registryErrorA]("AFailure"),
		NewNoArgsModuleErrorMapping[registryErrorB]("BFailure"),
	)
	want := []ModuleErrorMappingInfo{
		{GoType: reflect.TypeFor[registryErrorA](), SolidityError: "AFailure"},
		{GoType: reflect.TypeFor[registryErrorB](), SolidityError: "BFailure"},
	}
	infos := registry.Mappings()
	require.Equal(t, want, infos)
	infos[0] = ModuleErrorMappingInfo{GoType: reflect.TypeFor[registryErrorB](), SolidityError: "BFailure"}
	infos = append(infos, ModuleErrorMappingInfo{})
	require.Len(t, infos, 3)

	require.Equal(t, want, registry.Mappings())
	revert, matched := registry.Translate(registryErrorA{})
	require.True(t, matched)
	requireModuleRevert(t, contractABI, revert, "AFailure")
}

func TestModuleErrorRegistrySnapshotsABIErrorEntry(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewModuleErrorMapping[registryValueError]("ModuleFailure", func(err registryValueError) []any {
			return []any{err.Label, err.Amount}
		}),
	)
	original := contractABI.Errors["ModuleFailure"]
	contractABI.Errors["ModuleFailure"] = contractABI.Errors["AFailure"]

	input := registryValueError{Label: "snapshot", Amount: big.NewInt(11)}
	revert, matched := registry.Translate(input)
	require.True(t, matched)
	requireModuleRevertDefinition(t, original, revert, input.Label, input.Amount)
}

func TestModuleErrorRegistrySnapshotsABIInputsSlice(t *testing.T) {
	contractABI := newModuleErrorRegistryTestABI(t)
	registry := MustNewModuleErrorRegistry(contractABI,
		NewModuleErrorMapping[registryValueError]("ModuleFailure", func(err registryValueError) []any {
			return []any{err.Label, err.Amount}
		}),
	)
	original := contractABI.Errors["ModuleFailure"]
	original.Inputs = append(abi.Arguments(nil), original.Inputs...)
	mutated := contractABI.Errors["ModuleFailure"]
	mutated.Inputs[0] = contractABI.Errors["OtherFailure"].Inputs[0]
	require.NotEqual(t, original.Inputs[0].Type.String(), contractABI.Errors["ModuleFailure"].Inputs[0].Type.String())

	input := registryValueError{Label: "snapshot", Amount: big.NewInt(12)}
	revert, matched := registry.Translate(input)
	require.True(t, matched)
	requireModuleRevertDefinition(t, original, revert, input.Label, input.Amount)
}

func newModuleErrorRegistryTestABI(t *testing.T) abi.ABI {
	t.Helper()
	return mustTestABI(t, moduleErrorRegistryABIJSON)
}

func requireModuleRevert(t *testing.T, contractABI abi.ABI, revert error, errorName string, want ...any) {
	t.Helper()
	requireModuleRevertDefinition(t, contractABI.Errors[errorName], revert, want...)
}

func requireModuleRevertDefinition(t *testing.T, definition abi.Error, revert error, want ...any) {
	t.Helper()
	var carrier RevertDataCarrier
	require.ErrorAs(t, revert, &carrier)
	data := carrier.RevertData()
	require.GreaterOrEqual(t, len(data), 4)
	require.Equal(t, definition.ID[:4], data[:4])
	got, err := definition.Inputs.Unpack(data[4:])
	require.NoError(t, err)
	require.Len(t, got, len(want))
	for index := range want {
		require.Equal(t, want[index], got[index])
	}
}
