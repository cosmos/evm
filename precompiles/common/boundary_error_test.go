package common

import (
	"errors"
	"fmt"
	"maps"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"

	errorsmod "cosmossdk.io/errors"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func TestBoundaryErrorResolution(t *testing.T) {
	api, module, _ := newMsgServerErrorTestRegistries(t)
	cosmos := MustNewCosmosErrorRegistry(api, CosmosErrorMappings{NewCosmosErrorMapping(errMsgServerSynthetic, "PrecompileFailure")}, CosmosErrorMappings{NewCosmosErrorMapping(sdkerrors.ErrUnauthorized, SolidityErrSDKUnauthorized)}, nil)
	carrier := &testRevertDataCarrier{data: []byte{0xde, 0xad, 0xbe, 0xef, 1}, err: errMsgServerSynthetic}
	for _, terminal := range []error{nil, vm.ErrOutOfGas, fmt.Errorf("wrapped: %w", vm.ErrOutOfGas), carrier, fmt.Errorf("wrapped: %w", carrier)} {
		for _, result := range []ErrorResolution{cosmos.ResolveQueryError(api, "query", terminal, nil), cosmos.ResolveMsgServerError(api, "msg", terminal, module.Translate)} {
			require.Equal(t, ErrorTranslation{}, result.Translation)
			require.Equal(t, terminal, result.Err)
		}
	}
	for _, input := range []error{errMsgServerSynthetic, sdkerrors.ErrUnauthorized, errMsgServerUnmapped, errors.New("internal")} {
		for _, msg := range []bool{false, true} {
			result := cosmos.ResolveQueryError(api, "method", input, nil)
			expected := QueryError(api, cosmos, "method", input)
			if msg {
				result = cosmos.ResolveMsgServerError(api, "method", input, nil)
				translation := TranslateCosmosError(api, cosmos, input)
				expected = translation.Revert
				if translation.Kind == MappingKindInternal {
					expected = NewRevertWithSolidityError(api, SolidityErrMsgServerFailed, "method", input.Error())
				}
			}
			require.Equal(t, expected, result.Err)
			require.Equal(t, cosmos.Translate(api, input), result.Translation)
		}
	}
	// A test MsgServer's typed error takes precedence over its registered cause.
	server := func() error { return msgServerModuleError{cause: errMsgServerSynthetic} }
	result := cosmos.ResolveMsgServerError(api, "msg", server(), module.Translate)
	require.Equal(t, ErrorTranslation{}, result.Translation)
	data, err := ReturnRevertError(&vm.EVM{}, result.Err)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, errorSelector(api, msgServerSolidityErrModuleFailure), data)
	query := cosmos.ResolveQueryError(api, "query", server(), nil)
	require.Equal(t, MappingKindPrecompile, query.Translation.Kind)
	require.Equal(t, errorSelector(api, "PrecompileFailure"), query.Err.(RevertDataCarrier).RevertData())
	// Caller ABI changes now affect both the resolver and legacy wrapper.
	input := errors.New("internal")
	before := cosmos.ResolveQueryError(api, "query", input, nil)
	delete(api.Errors, SolidityErrQueryFailed)
	after := cosmos.ResolveQueryError(api, "query", input, nil)
	require.NotEqual(t, before.Err, after.Err)
	require.Equal(t, QueryError(api, cosmos, "query", input), after.Err)
	beforeMsg := cosmos.ResolveMsgServerError(api, "msg", input, nil)
	delete(api.Errors, SolidityErrMsgServerFailed)
	require.NotEqual(t, beforeMsg.Err, cosmos.ResolveMsgServerError(api, "msg", input, nil).Err)
}

const (
	msgServerSolidityErrModuleFailure = "ModuleFailure"
	msgServerSolidityErrAFailure      = "AFailure"
)

const msgServerErrorABIJSON = `[
	{"type":"error","name":"ModuleFailure","inputs":[]},
	{"type":"error","name":"AFailure","inputs":[]},
	{"type":"error","name":"BFailure","inputs":[]},
	{"type":"error","name":"PrecompileFailure","inputs":[]},
	{"type":"error","name":"SDKUnauthorized","inputs":[]},
	{"type":"error","name":"UnmappedCosmosError","inputs":[{"name":"codespace","type":"string"},{"name":"code","type":"uint32"}]},
	{"type":"error","name":"MsgServerFailed","inputs":[{"name":"msgMethod","type":"string"},{"name":"reason","type":"string"}]}
]`

var (
	errMsgServerSynthetic = errorsmod.Register("msg-server-test", 7, "synthetic")
	errMsgServerUnmapped  = errorsmod.Register("msg-server-unmapped", 8, "unmapped")
)

type msgServerModuleError struct {
	cause error
}

func (err msgServerModuleError) Error() string { return "module error" }
func (err msgServerModuleError) Unwrap() error { return err.cause }

type msgServerCustomAsError struct {
	value msgServerModuleError
}

func (msgServerCustomAsError) Error() string { return "custom As" }

func (err msgServerCustomAsError) As(target any) bool {
	value, ok := target.(*msgServerModuleError)
	if !ok {
		return false
	}
	*value = err.value
	return true
}

func TestResolveMsgServerErrorPreservesTerminalErrors(t *testing.T) {
	api, _, cosmosRegistry := newMsgServerErrorTestRegistries(t)

	require.NoError(t, cosmosRegistry.ResolveMsgServerError(api, "mint", nil, nil).Err)
	require.Same(t, vm.ErrOutOfGas, cosmosRegistry.ResolveMsgServerError(api, "mint", vm.ErrOutOfGas, nil).Err)

	revertData := []byte{0xde, 0xad, 0xbe, 0xef, 0x01, 0x02}
	wrapped := fmt.Errorf("outer: %w", &testRevertDataCarrier{data: revertData, err: errMsgServerSynthetic})
	got := cosmosRegistry.ResolveMsgServerError(api, "mint", wrapped, nil).Err
	require.Same(t, wrapped, got)
	var carrier RevertDataCarrier
	require.ErrorAs(t, got, &carrier)
	require.Equal(t, revertData, carrier.RevertData())
}

func TestResolveMsgServerErrorPrefersModuleMappings(t *testing.T) {
	moduleABI, moduleRegistry, cosmosRegistry := newMsgServerErrorTestRegistries(t)

	tests := []struct {
		name      string
		err       error
		errorName string
	}{
		{
			name:      "before Cosmos mapping",
			err:       msgServerModuleError{cause: errMsgServerSynthetic},
			errorName: msgServerSolidityErrModuleFailure,
		},
		{
			name:      "wrapped error",
			err:       fmt.Errorf("wrapped: %w", msgServerModuleError{}),
			errorName: msgServerSolidityErrModuleFailure,
		},
		{
			name:      "custom As",
			err:       msgServerCustomAsError{value: msgServerModuleError{}},
			errorName: msgServerSolidityErrModuleFailure,
		},
		{
			name:      "join uses registry declaration order",
			err:       errors.Join(registryErrorB{}, registryErrorA{}),
			errorName: msgServerSolidityErrAFailure,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := cosmosRegistry.ResolveMsgServerError(moduleABI, "mint", tc.err, moduleRegistry.Translate).Err
			require.Equal(t, errorSelector(moduleABI, tc.errorName), got.(RevertDataCarrier).RevertData())
		})
	}
}

func TestResolveMsgServerErrorTranslatesCosmosErrors(t *testing.T) {
	moduleABI, _, cosmosRegistry := newMsgServerErrorTestRegistries(t)

	t.Run("precompile mapping", func(t *testing.T) {
		got := cosmosRegistry.ResolveMsgServerError(moduleABI, "mint", errorsmod.Wrap(errMsgServerSynthetic, "diagnostic"), nil).Err
		require.Equal(t, errorSelector(moduleABI, "PrecompileFailure"), got.(RevertDataCarrier).RevertData())
	})

	t.Run("shared SDK mapping", func(t *testing.T) {
		got := cosmosRegistry.ResolveMsgServerError(moduleABI, "mint", errorsmod.Wrap(sdkerrors.ErrUnauthorized, "diagnostic"), nil).Err
		require.Equal(t, errorSelector(moduleABI, SolidityErrSDKUnauthorized), got.(RevertDataCarrier).RevertData())
	})

	t.Run("registered unmapped Cosmos error", func(t *testing.T) {
		got := cosmosRegistry.ResolveMsgServerError(moduleABI, "mint", errMsgServerUnmapped, nil).Err
		data := got.(RevertDataCarrier).RevertData()
		require.Equal(t, errorSelector(moduleABI, SolidityErrUnmappedCosmosError), data[:4])
		decoded, err := moduleABI.Errors[SolidityErrUnmappedCosmosError].Inputs.Unpack(data[4:])
		require.NoError(t, err)
		require.Equal(t, []interface{}{"msg-server-unmapped", uint32(8)}, decoded)
	})
}

func TestResolveMsgServerErrorHandlesMissingModuleMappings(t *testing.T) {
	moduleABI, _, cosmosRegistry := newMsgServerErrorTestRegistries(t)
	emptyModuleRegistry := MustNewModuleErrorRegistry(moduleABI)

	tests := []struct {
		name      string
		translate func(error) (bool, error)
	}{
		{name: "nil callback", translate: nil},
		{name: "empty registry", translate: emptyModuleRegistry.Translate},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := errors.New("mint backend unavailable")
			got := cosmosRegistry.ResolveMsgServerError(moduleABI, "mint", input, tc.translate).Err
			data := got.(RevertDataCarrier).RevertData()
			require.Equal(t, []byte{0x23, 0x7d, 0x7e, 0xdd}, data[:4], "MsgServerFailed(string,string) selector must remain stable")
			require.Equal(t, errorSelector(moduleABI, SolidityErrMsgServerFailed), data[:4])
			decoded, err := moduleABI.Errors[SolidityErrMsgServerFailed].Inputs.Unpack(data[4:])
			require.NoError(t, err)
			require.Equal(t, []interface{}{"mint", input.Error()}, decoded)
		})
	}
}

func newMsgServerErrorTestRegistries(t *testing.T) (abi.ABI, *ModuleErrorRegistry, *CosmosErrorRegistry) {
	t.Helper()
	moduleABI := mustTestABI(t, msgServerErrorABIJSON)
	moduleRegistry := MustNewModuleErrorRegistry(
		moduleABI,
		NewNoArgsModuleErrorMapping[msgServerModuleError](msgServerSolidityErrModuleFailure),
		NewNoArgsModuleErrorMapping[registryErrorA](msgServerSolidityErrAFailure),
		NewNoArgsModuleErrorMapping[registryErrorB]("BFailure"),
	)
	maps.Copy(moduleABI.Errors, mustTestABI(t, sharedErrorABIJSON).Errors)
	cosmosRegistry := MustNewCosmosErrorRegistry(
		moduleABI,
		CosmosErrorMappings{NewCosmosErrorMapping(errMsgServerSynthetic, "PrecompileFailure")},
		CosmosErrorMappings{NewCosmosErrorMapping(sdkerrors.ErrUnauthorized, SolidityErrSDKUnauthorized)},
		nil,
	)
	return moduleABI, moduleRegistry, cosmosRegistry
}

func TestResolveErrorPreservesTerminalBeforeCallbacks(t *testing.T) {
	api, _, registry := newMsgServerErrorTestRegistries(t)
	carrier := NewRevertWithSolidityError(api, "ModuleFailure")
	for _, input := range []error{
		nil, vm.ErrOutOfGas, fmt.Errorf("outer: %w", vm.ErrOutOfGas),
		errors.Join(errMsgServerSynthetic, vm.ErrOutOfGas),
		carrier, fmt.Errorf("outer: %w", carrier), errors.Join(errMsgServerSynthetic, carrier),
		fmt.Errorf("outer: %w", errors.Join(msgServerModuleError{}, carrier)),
		errors.Join(carrier, vm.ErrOutOfGas),
	} {
		for _, boundary := range []string{"", SolidityErrQueryFailed, SolidityErrMsgServerFailed, SolidityErrEventEmitFailed} {
			var calls []string
			module := func(error) (bool, error) { calls = append(calls, "module"); return true, errors.New("unexpected") }
			fallback := func(error) error { calls = append(calls, "fallback"); return errors.New("unexpected") }
			if boundary != "" {
				fallback = registry.BoundaryFallback(api, boundary, "operation", func(error) string {
					calls = append(calls, "reason")
					return "unexpected"
				})
			}
			result := registry.ResolveError(api, input, module, fallback)
			require.Equal(t, input, result.Err)
			require.Equal(t, ErrorTranslation{}, result.Translation)
			require.Empty(t, calls)
		}
	}
}

func TestResolveErrorModuleCosmosFallbackOrder(t *testing.T) {
	api, typed, registry := newMsgServerErrorTestRegistries(t)
	encodingFailure := errors.New("child encoding failed")
	custom := errors.New("custom module failure")
	fallbackResult := errors.New("caller internal fallback")
	for _, tc := range []struct {
		name     string
		input    error
		want     error
		kind     MappingKind
		fallback bool
	}{
		{"custom encoding failure", custom, encodingFailure, MappingKindInternal, false},
		{"typed before registered cause", msgServerModuleError{cause: errMsgServerSynthetic}, NewRevertWithSolidityError(api, "ModuleFailure"), MappingKindInternal, false},
		{"ordered typed registry", errors.Join(registryErrorB{}, registryErrorA{}), NewRevertWithSolidityError(api, "AFailure"), MappingKindInternal, false},
		{"precompile", errMsgServerSynthetic, registry.Translate(api, errMsgServerSynthetic).Revert, MappingKindPrecompile, false},
		{"SDK", sdkerrors.ErrUnauthorized, registry.Translate(api, sdkerrors.ErrUnauthorized).Revert, MappingKindSharedSDK, false},
		{"registered unmapped", errMsgServerUnmapped, registry.Translate(api, errMsgServerUnmapped).Revert, MappingKindUnmapped, false},
		{"internal fallback", errors.New("internal"), fallbackResult, MappingKindInternal, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			result := registry.ResolveError(api, tc.input, func(input error) (bool, error) {
				calls = append(calls, "module")
				require.Equal(t, tc.input, input)
				// A caller can attempt recursive encoding before its ordered registry.
				if input == custom {
					return true, encodingFailure
				}
				if matched, translated := typed.Translate(input); matched {
					return true, translated
				}
				return false, errors.New("ignored because unmatched")
			}, func(input error) error {
				calls = append(calls, "fallback")
				require.Equal(t, tc.input, input)
				return fallbackResult
			})
			require.Equal(t, tc.want, result.Err)
			if tc.fallback || tc.input == custom {
				require.Same(t, tc.want, result.Err)
			}
			require.Equal(t, tc.kind, result.Translation.Kind)
			if tc.kind != MappingKindInternal || tc.fallback {
				require.Equal(t, registry.Translate(api, tc.input), result.Translation)
			} else {
				require.Equal(t, ErrorTranslation{}, result.Translation)
			}
			wantCalls := []string{"module"}
			if tc.fallback {
				wantCalls = append(wantCalls, "fallback")
			}
			require.Equal(t, wantCalls, calls)
		})
	}
	// Matching preserves a non-carrier failure and stops later translation tiers.
	for _, returned := range []error{encodingFailure, nil} {
		result := registry.ResolveError(api, errMsgServerSynthetic, func(error) (bool, error) { return true, returned }, func(error) error {
			t.Fatal("matched module result reached fallback")
			return nil
		})
		if returned == nil {
			require.Same(t, errMsgServerSynthetic, result.Err)
		} else {
			require.Same(t, returned, result.Err)
		}
		require.Equal(t, ErrorTranslation{}, result.Translation)
	}
	internal := errors.New("unregistered")
	result := registry.ResolveError(api, internal, nil, nil)
	require.Same(t, internal, result.Err)
	require.Equal(t, registry.Translate(api, internal), result.Translation)
}

func TestBoundaryFallbackReasonDoesNotChangeClassification(t *testing.T) {
	api, typed, registry := newMsgServerErrorTestRegistries(t)
	secret := "confidential backend diagnostic"
	for _, boundary := range []string{SolidityErrQueryFailed, SolidityErrMsgServerFailed, SolidityErrEventEmitFailed} {
		for _, tc := range []struct {
			name   string
			input  error
			mapped bool
		}{
			{"internal", errors.New(secret), false},
			{"module", fmt.Errorf("%s: %w", secret, msgServerModuleError{cause: sdkerrors.ErrUnauthorized}), true},
			{"precompile", fmt.Errorf("%s: %w", secret, errMsgServerSynthetic), true},
			{"SDK", fmt.Errorf("%s: %w", secret, sdkerrors.ErrUnauthorized), true},
			{"unmapped", fmt.Errorf("%s: %w", secret, errMsgServerUnmapped), true},
		} {
			t.Run(boundary+"/"+tc.name, func(t *testing.T) {
				var calls int
				fallback := registry.BoundaryFallback(api, boundary, "operation", func(input error) string {
					calls++
					require.Same(t, tc.input, input)
					return "public diagnostic"
				})
				actual := registry.ResolveError(api, tc.input, typed.Translate, fallback)
				var expected error
				if matched, translated := typed.Translate(tc.input); matched {
					expected = translated
				} else if tc.mapped {
					expected = registry.Translate(api, tc.input).Revert
				} else {
					expected = NewRevertWithSolidityError(api, boundary, "operation", "public diagnostic")
				}
				var carrier RevertDataCarrier
				require.ErrorAs(t, actual.Err, &carrier)
				require.Equal(t, expected.(RevertDataCarrier).RevertData(), carrier.RevertData())
				require.NotContains(t, string(carrier.RevertData()), secret)
				if tc.mapped {
					require.Zero(t, calls)
				} else {
					require.Equal(t, 1, calls)
				}
			})
		}
	}
}

func TestBoundaryFallbackDefaultsAndCallerABI(t *testing.T) {
	api, _, registry := newMsgServerErrorTestRegistries(t)
	input := errors.New("internal")
	// ABI-taking methods remain compatible with the unchanged legacy wrapper.
	var (
		query   func(abi.ABI, string, error, func(error) (bool, error)) ErrorResolution
		message func(abi.ABI, string, error, func(error) (bool, error)) ErrorResolution
	)
	query, message = registry.ResolveQueryError, registry.ResolveMsgServerError
	require.Equal(t, query(api, "query", input, nil), registry.ResolveError(api, input, nil, registry.BoundaryFallback(api, SolidityErrQueryFailed, "query", nil)))
	require.Equal(t, message(api, "msg", input, nil), registry.ResolveError(api, input, nil, registry.BoundaryFallback(api, SolidityErrMsgServerFailed, "msg", nil)))
	for _, boundary := range []string{SolidityErrQueryFailed, SolidityErrMsgServerFailed, SolidityErrEventEmitFailed} {
		fallback := registry.BoundaryFallback(api, boundary, "operation", nil)
		before := registry.ResolveError(api, input, nil, fallback)
		expected := NewRevertWithSolidityError(api, boundary, "operation", input.Error())
		require.Equal(t, expected, before.Err)
		data, err := ReturnRevertError(&vm.EVM{}, before.Err)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		decoded, err := api.Errors[boundary].Inputs.Unpack(data[4:])
		require.NoError(t, err)
		require.Equal(t, []interface{}{"operation", input.Error()}, decoded)
		// An explicit empty public reason must not fall back to the private one.
		empty := registry.ResolveError(api, input, nil, registry.BoundaryFallback(api, boundary, "operation", func(error) string { return "" }))
		require.Equal(t, NewRevertWithSolidityError(api, boundary, "operation", ""), empty.Err)
		api.Errors[boundary].Inputs[0] = abi.Argument{}
		delete(api.Errors, boundary)
		after := registry.ResolveError(api, input, nil, fallback)
		require.NotEqual(t, before.Err, after.Err)
		require.Equal(t, NewRevertWithSolidityError(api, boundary, "operation", input.Error()), after.Err)
		require.Equal(t, after, registry.ResolveError(api, input, nil, registry.BoundaryFallback(api, boundary, "operation", nil)))
	}
}

func TestBoundaryFallbackRejectsUnsupportedDefinition(t *testing.T) {
	api, _, registry := newMsgServerErrorTestRegistries(t)
	for _, name := range []string{"", "UnknownError", "ModuleFailure", SolidityErrUnmappedCosmosError} {
		t.Run(name, func(t *testing.T) {
			require.PanicsWithError(t, fmt.Sprintf("unsupported boundary fallback %q", name), func() {
				registry.BoundaryFallback(api, name, "operation", nil)
			})
		})
	}
}

func TestResolveErrorPreservesInputWhenFallbackReturnsNil(t *testing.T) {
	api, _, registry := newMsgServerErrorTestRegistries(t)
	input := errors.New("internal")
	result := registry.ResolveError(api, input, nil, func(original error) error {
		require.Same(t, input, original)
		return nil
	})
	require.Same(t, input, result.Err)
	require.Equal(t, registry.Translate(api, input), result.Translation)
}

// The method values also lock the exact non-variadic public signatures.
func TestBoundaryResolversWithModuleTranslator(t *testing.T) {
	api, typed, registry := newMsgServerErrorTestRegistries(t)
	for _, boundary := range []struct {
		name    string
		resolve func(abi.ABI, string, error, func(error) (bool, error)) ErrorResolution
	}{
		{SolidityErrQueryFailed, registry.ResolveQueryError},
		{SolidityErrMsgServerFailed, registry.ResolveMsgServerError},
		{SolidityErrEventEmitFailed, registry.ResolveEventError},
	} {
		t.Run(boundary.name, func(t *testing.T) {
			operation := "operation/" + boundary.name
			fallback := registry.BoundaryFallback(api, boundary.name, operation, nil)
			carrier := &testRevertDataCarrier{data: []byte{0xde, 0xad, 0xbe, 0xef}, err: errMsgServerSynthetic}
			t.Run("terminal before callback", func(t *testing.T) {
				for _, input := range []error{
					nil, vm.ErrOutOfGas, fmt.Errorf("wrapped: %w", vm.ErrOutOfGas),
					errors.Join(errMsgServerSynthetic, vm.ErrOutOfGas), carrier,
					fmt.Errorf("wrapped: %w", carrier), errors.Join(errMsgServerSynthetic, carrier),
					fmt.Errorf("wrapped: %w", errors.Join(carrier, vm.ErrOutOfGas)),
				} {
					for _, translate := range []func(error) (bool, error){nil, func(error) (bool, error) {
						t.Fatal("terminal error reached module translator")
						return false, nil
					}} {
						actual := boundary.resolve(abi.ABI{}, operation, input, translate)
						require.Equal(t, ErrorResolution{Err: input}, actual)
						require.Equal(t, registry.ResolveError(abi.ABI{}, input, translate,
							registry.BoundaryFallback(abi.ABI{}, boundary.name, operation, nil)), actual)
					}
				}
			})
			t.Run("module result wins", func(t *testing.T) {
				input := fmt.Errorf("wrapped: %w", errMsgServerSynthetic)
				for _, returned := range []error{NewRevertWithSolidityError(api, "ModuleFailure"), errors.New("child encoding failed"), nil} {
					translate := func(original error) (bool, error) {
						require.Same(t, input, original)
						return true, returned
					}
					actual := boundary.resolve(api, operation, input, translate)
					expected := returned
					if expected == nil {
						expected = input
					}
					require.Same(t, expected, actual.Err)
					require.Equal(t, ErrorTranslation{}, actual.Translation)
					require.Equal(t, registry.ResolveError(api, input, translate, fallback), actual)
				}
				inputTyped := msgServerModuleError{cause: errMsgServerSynthetic}
				actual := boundary.resolve(api, operation, inputTyped, typed.Translate)
				require.Equal(t, ErrorResolution{Err: NewRevertWithSolidityError(api, "ModuleFailure")}, actual)
			})
			t.Run("Cosmos before fallback", func(t *testing.T) {
				internal := errors.New("backend unavailable")
				for _, tc := range []struct {
					input       error
					translation ErrorTranslation
				}{
					{errMsgServerSynthetic, ErrorTranslation{Revert: NewRevertWithSolidityError(api, "PrecompileFailure"), Kind: MappingKindPrecompile, Key: CosmosErrorKey{Codespace: "msg-server-test", Code: 7}}},
					{sdkerrors.ErrUnauthorized, ErrorTranslation{Revert: NewRevertWithSolidityError(api, SolidityErrSDKUnauthorized), Kind: MappingKindSharedSDK, Key: CosmosErrorKey{Codespace: sdkerrors.ErrUnauthorized.Codespace(), Code: sdkerrors.ErrUnauthorized.ABCICode()}}},
					{errMsgServerUnmapped, ErrorTranslation{Revert: NewRevertWithSolidityError(api, SolidityErrUnmappedCosmosError, "msg-server-unmapped", uint32(8)), Kind: MappingKindUnmapped, Key: CosmosErrorKey{Codespace: "msg-server-unmapped", Code: 8}, IsUnmapped: true}},
					{internal, ErrorTranslation{Revert: internal, Kind: MappingKindInternal}},
				} {
					for _, translate := range []func(error) (bool, error){nil, func(original error) (bool, error) {
						require.Same(t, tc.input, original)
						return false, errors.New("ignored non-match")
					}} {
						actual := boundary.resolve(api, operation, tc.input, translate)
						expected := tc.translation.Revert
						if tc.translation.Kind == MappingKindInternal {
							expected = NewRevertWithSolidityError(api, boundary.name, operation, internal.Error())
							data := actual.Err.(RevertDataCarrier).RevertData()
							decoded, err := api.Errors[boundary.name].Inputs.Unpack(data[4:])
							require.NoError(t, err)
							require.Equal(t, []any{operation, internal.Error()}, decoded)
						}
						require.Equal(t, expected.(RevertDataCarrier).RevertData(), actual.Err.(RevertDataCarrier).RevertData())
						require.Equal(t, tc.translation, actual.Translation)
						require.Equal(t, registry.ResolveError(api, tc.input, translate, fallback), actual)
					}
				}
			})
			t.Run("caller ABI", func(t *testing.T) {
				caller := abi.ABI{Errors: maps.Clone(api.Errors)}
				caller.Errors["PrecompileFailure"] = api.Errors["AFailure"]
				mapped := boundary.resolve(caller, operation, errMsgServerSynthetic, nil)
				require.Equal(t, errorSelector(api, "AFailure"), mapped.Err.(RevertDataCarrier).RevertData())
				input := errors.New("caller reason")
				caller.Errors[boundary.name] = abi.NewError("CallerBoundary", api.Errors[boundary.name].Inputs)
				actual := boundary.resolve(caller, operation, input, nil)
				require.Equal(t, NewRevertWithSolidityError(caller, boundary.name, operation, input.Error()), actual.Err)
				require.NotEqual(t, boundary.resolve(api, operation, input, nil).Err, actual.Err)
				require.Equal(t, registry.ResolveError(caller, input, nil, registry.BoundaryFallback(caller, boundary.name, operation, nil)), actual)
				delete(caller.Errors, boundary.name)
				missing := boundary.resolve(caller, operation, input, nil)
				require.Equal(t, NewRevertWithSolidityError(caller, boundary.name, operation, input.Error()), missing.Err)
				require.Equal(t, ErrorTranslation{Revert: input, Kind: MappingKindInternal}, missing.Translation)
			})
		})
	}
}
