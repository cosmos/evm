package common

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
)

// ErrorResolution preserves Cosmos classification for caller-owned logging.
// Translation is populated only when the Cosmos tier was reached.
type ErrorResolution struct {
	Err         error
	Translation ErrorTranslation
}

// validateBoundaryErrors checks the canonical definitions required by boundary
// resolution during MustNewCosmosErrorRegistry initialization.
func validateBoundaryErrors(api abi.ABI) error {
	for _, expected := range []struct{ name, signature string }{
		{SolidityErrUnmappedCosmosError, "UnmappedCosmosError(string,uint32)"},
		{SolidityErrQueryFailed, "QueryFailed(string,string)"},
		{SolidityErrMsgServerFailed, "MsgServerFailed(string,string)"},
		{SolidityErrEventEmitFailed, "EventEmitFailed(string,string)"},
	} {
		definition, ok := api.Errors[expected.name]
		if !ok {
			return fmt.Errorf("boundary resolution requires ABI error %s", expected.signature)
		}
		// Sig alone is caller-mutable metadata; packing actually uses Inputs and ID.
		derived := abi.NewError(expected.name, definition.Inputs)
		if definition.Name != expected.name || definition.Sig != expected.signature || derived.Sig != expected.signature || definition.ID != derived.ID {
			return fmt.Errorf("boundary resolution requires canonical ABI error %s", expected.signature)
		}
	}
	return nil
}

// ResolveQueryError applies the optional module translator, Cosmos mappings,
// and then QueryFailed. A nil translator skips the module tier.
func (registry *CosmosErrorRegistry) ResolveQueryError(api abi.ABI, method string, err error, translate func(error) (bool, error)) ErrorResolution {
	return registry.ResolveError(api, err, translate, registry.BoundaryFallback(api, SolidityErrQueryFailed, method, nil))
}

// ResolveMsgServerError applies the optional module translator, Cosmos mappings,
// and then MsgServerFailed. A nil translator skips the module tier.
func (registry *CosmosErrorRegistry) ResolveMsgServerError(api abi.ABI, method string, err error, translate func(error) (bool, error)) ErrorResolution {
	return registry.ResolveError(api, err, translate, registry.BoundaryFallback(api, SolidityErrMsgServerFailed, method, nil))
}

// ResolveEventError applies the optional module translator, Cosmos mappings,
// and then EventEmitFailed. A nil translator skips the module tier.
func (registry *CosmosErrorRegistry) ResolveEventError(api abi.ABI, kind string, err error, translate func(error) (bool, error)) ErrorResolution {
	return registry.ResolveError(api, err, translate, registry.BoundaryFallback(api, SolidityErrEventEmitFailed, kind, nil))
}

// ResolveError preserves terminal errors, then tries the optional module translator,
// Cosmos mappings encoded with api, and finally the optional fallback. Both
// callbacks receive the original error. A module match stops further translation
// and preserves its non-nil result, including a non-carrier encoding failure. A nil matched result
// preserves the original error instead of clearing it. A non-match's result is
// ignored. A nil fallback or a nil fallback result likewise preserves the original
// internal error. Translation retains the original Cosmos classification for logging.
func (registry *CosmosErrorRegistry) ResolveError(api abi.ABI, err error, translate func(error) (bool, error), fallback func(error) error) ErrorResolution {
	if !NeedsErrorTranslation(err) {
		return ErrorResolution{Err: err}
	}
	if translate != nil {
		if matched, revert := translate(err); matched {
			if revert == nil {
				revert = err
			}
			return ErrorResolution{Err: revert}
		}
	}
	translation := TranslateCosmosError(api, registry, err)
	if translation.Kind != MappingKindInternal {
		return ErrorResolution{Err: translation.Revert, Translation: translation}
	}
	if fallback != nil {
		if resolved := fallback(err); resolved != nil {
			err = resolved
		}
	}
	return ErrorResolution{Err: err, Translation: translation}
}

// BoundaryFallback creates a fallback for ResolveError using the supplied ABI.
// errorName selects QueryFailed, MsgServerFailed, or EventEmitFailed; operation is
// the method or event kind. A nil reason uses the original error's message. A
// non-nil reason can redact that message, including to an empty string, without
// changing module/Cosmos matching or existing revert data. ResolveError calls this
// policy only for internal errors after both translation tiers decline them.
// The factory panics for names outside these three canonical boundary errors.
func (*CosmosErrorRegistry) BoundaryFallback(api abi.ABI, errorName, operation string, reason func(error) string) func(error) error {
	switch errorName {
	case SolidityErrQueryFailed, SolidityErrMsgServerFailed, SolidityErrEventEmitFailed:
	default:
		panic(fmt.Errorf("unsupported boundary fallback %q", errorName))
	}
	return func(err error) error {
		var diagnostic string
		if reason == nil {
			diagnostic = err.Error()
		} else {
			diagnostic = reason(err)
		}
		return NewRevertWithSolidityError(api, errorName, operation, diagnostic)
	}
}

// NeedsErrorTranslation reports whether an error needs mapping or fallback
// encoding. It returns false for nil, out-of-gas, and existing revert carriers,
// including wrapped errors, so callers can return those values unchanged.
func NeedsErrorTranslation(err error) bool {
	if err == nil || errors.Is(err, vm.ErrOutOfGas) {
		return false
	}
	var carrier RevertDataCarrier
	return !errors.As(err, &carrier)
}
