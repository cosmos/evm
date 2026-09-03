package common

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// ModuleErrorMapping is an opaque typed module-error declaration.
type ModuleErrorMapping struct {
	goType        reflect.Type
	solidityError string
	match         func(error) ([]any, bool)
}

// ModuleErrorMappingInfo describes a validated module-error declaration.
type ModuleErrorMappingInfo struct {
	GoType        reflect.Type
	SolidityError string
}

// NewModuleErrorMapping declares how a named concrete module error maps to a
// Solidity custom error. PT is inferred as *T and guarantees that T has an
// error implementation through either its value or pointer method set.
func NewModuleErrorMapping[T any, PT interface {
	*T
	error
}](solidityError string, args func(T) []any) ModuleErrorMapping {
	goType := reflect.TypeFor[T]()
	mapping := ModuleErrorMapping{
		goType:        goType,
		solidityError: solidityError,
	}
	if args == nil {
		return mapping
	}

	mapping.match = func(err error) ([]any, bool) {
		var value T
		if _, ok := any(value).(error); ok {
			if moduleErrorAs(err, &value) {
				return args(value), true
			}
		}

		var pointer PT
		if moduleErrorAs(err, &pointer) && pointer != nil {
			return args(*pointer), true
		}
		return nil, false
	}
	return mapping
}

func moduleErrorAs(err error, target any) bool {
	return errors.As(err, target)
}

// NewNoArgsModuleErrorMapping declares a module error whose Solidity custom
// error has no arguments. PT is inferred as *T.
func NewNoArgsModuleErrorMapping[T any, PT interface {
	*T
	error
}](solidityError string) ModuleErrorMapping {
	return NewModuleErrorMapping[T, PT](solidityError, func(T) []any { return nil })
}

type validatedModuleErrorMapping struct {
	info  ModuleErrorMappingInfo
	match func(error) ([]any, bool)
}

// ModuleErrorRegistry is an immutable ordered registry of typed module errors.
type ModuleErrorRegistry struct {
	effectiveABI abi.ABI
	mappings     []validatedModuleErrorMapping
	infos        []ModuleErrorMappingInfo
}

// ValidateModuleErrorRegistry validates typed module-error declarations against
// an effective Solidity ABI.
func ValidateModuleErrorRegistry(effectiveABI abi.ABI, mappings ...ModuleErrorMapping) error {
	_, err := NewModuleErrorRegistry(effectiveABI, mappings...)
	return err
}

// NewModuleErrorRegistry builds an immutable ordered typed module-error registry.
func NewModuleErrorRegistry(effectiveABI abi.ABI, mappings ...ModuleErrorMapping) (*ModuleErrorRegistry, error) {
	if err := validateEffectiveABI(effectiveABI); err != nil {
		return nil, err
	}

	validated := make([]validatedModuleErrorMapping, 0, len(mappings))
	infos := make([]ModuleErrorMappingInfo, 0, len(mappings))
	seenTypes := make(map[reflect.Type]struct{}, len(mappings))
	snapshotErrors := make(map[string]abi.Error, len(mappings))

	for _, mapping := range mappings {
		if err := validateModuleErrorMapping(effectiveABI, mapping); err != nil {
			return nil, err
		}
		if _, exists := seenTypes[mapping.goType]; exists {
			return nil, fmt.Errorf("duplicate module error mapping for %s", mapping.goType)
		}
		seenTypes[mapping.goType] = struct{}{}

		if _, exists := snapshotErrors[mapping.solidityError]; !exists {
			definition := effectiveABI.Errors[mapping.solidityError]
			definition.Inputs = append(abi.Arguments(nil), definition.Inputs...)
			snapshotErrors[mapping.solidityError] = definition
		}

		info := ModuleErrorMappingInfo{
			GoType:        mapping.goType,
			SolidityError: mapping.solidityError,
		}
		validated = append(validated, validatedModuleErrorMapping{info: info, match: mapping.match})
		infos = append(infos, info)
	}

	return &ModuleErrorRegistry{
		effectiveABI: abi.ABI{Errors: snapshotErrors},
		mappings:     validated,
		infos:        infos,
	}, nil
}

func validateModuleErrorMapping(effectiveABI abi.ABI, mapping ModuleErrorMapping) error {
	if mapping.solidityError == "" {
		return fmt.Errorf("module error mapping requires Solidity error name")
	}
	if mapping.goType == nil {
		return fmt.Errorf("module error mapping %s requires Go type", mapping.solidityError)
	}
	// Anonymous types can promote Error from an embedded field, which generic
	// constraints cannot distinguish from a named concrete type.
	if mapping.goType.Name() == "" {
		return fmt.Errorf("module error mapping Go type must be named: %s", mapping.goType)
	}
	if mapping.match == nil {
		return fmt.Errorf("module error mapping %s requires non-nil args function", mapping.solidityError)
	}
	if _, exists := effectiveABI.Errors[mapping.solidityError]; !exists {
		return fmt.Errorf("missing ABI error %s for module mapping", mapping.solidityError)
	}
	return nil
}

// MustNewModuleErrorRegistry builds a typed module-error registry or panics.
func MustNewModuleErrorRegistry(effectiveABI abi.ABI, mappings ...ModuleErrorMapping) *ModuleErrorRegistry {
	registry, err := NewModuleErrorRegistry(effectiveABI, mappings...)
	if err != nil {
		panic(err)
	}
	return registry
}

// Translate returns whether a declaration matched and its Solidity revert.
func (registry *ModuleErrorRegistry) Translate(err error) (matched bool, revert error) {
	if err == nil {
		return false, nil
	}
	for _, mapping := range registry.mappings {
		args, ok := mapping.match(err)
		if !ok {
			continue
		}
		return true, NewRevertWithSolidityError(registry.effectiveABI, mapping.info.SolidityError, args...)
	}
	return false, nil
}

// Mappings returns an ordered copy of the registry metadata.
func (registry *ModuleErrorRegistry) Mappings() []ModuleErrorMappingInfo {
	return append([]ModuleErrorMappingInfo(nil), registry.infos...)
}
