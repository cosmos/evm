package common

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	errorsmod "cosmossdk.io/errors"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var errPhaseOneSynthetic = errorsmod.Register("phase-one-test", 7, "synthetic drift")

type loopingError struct{}

func (loopingError) Error() string   { return "loop" }
func (e loopingError) Unwrap() error { return e }

func TestExtractCosmosErrorKey(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want CosmosErrorKey
		ok   bool
	}{
		{"direct", sdkerrors.ErrUnauthorized, CosmosErrorKey{"sdk", 4}, true},
		{"cosmos wrap", errorsmod.Wrap(sdkerrors.ErrUnauthorized, "changed text"), CosmosErrorKey{"sdk", 4}, true},
		{"standard wrap", fmt.Errorf("changed text: %w", sdkerrors.ErrUnauthorized), CosmosErrorKey{"sdk", 4}, true},
		{"mixed wrap", fmt.Errorf("outer: %w", errorsmod.Wrap(sdkerrors.ErrUnauthorized, "inner")), CosmosErrorKey{"sdk", 4}, true},
		{"plain", errors.New("unauthorized"), CosmosErrorKey{}, false},
		{"nil", nil, CosmosErrorKey{}, false},
		{"grpc", status.Error(codes.NotFound, "unauthorized"), CosmosErrorKey{}, false},
		{"loop", loopingError{}, CosmosErrorKey{}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ExtractCosmosErrorKey(tc.err)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCosmosErrorRegistryValidation(t *testing.T) {
	moduleABI := mustTestABI(t, `[
		{"type":"error","name":"ModuleFailure","inputs":[]},
		{"type":"error","name":"SDKUnauthorized","inputs":[]},
		{"type":"error","name":"TypedOverride","inputs":[{"name":"value","type":"uint256"}]},
		{"type":"error","name":"AlternateOverride","inputs":[{"name":"value","type":"uint256"}]},
		{"type":"error","name":"UnmappedCosmosError","inputs":[{"name":"codespace","type":"string"},{"name":"code","type":"uint32"}]}
	]`)
	module := CosmosErrorMappings{NewCosmosErrorMapping(errPhaseOneSynthetic, "ModuleFailure")}
	shared := CosmosErrorMappings{NewCosmosErrorMapping(sdkerrors.ErrUnauthorized, "SDKUnauthorized")}
	overrides := OverrideDeclarations{{
		ShadowedKey:       NewCosmosErrorKey(sdkerrors.ErrUnauthorized),
		SoliditySignature: "TypedOverride(uint256)",
		OwningABI:         "TestI",
		CallSiteAnchor:    "precompiles/common/cosmos_errors.go#OverrideDeclarations.ForABI",
		Rationale:         "trusted call context",
	}}

	require.NoError(t, ValidateCosmosErrorRegistry(moduleABI, module, shared, overrides))
	require.ErrorContains(t, ValidateCosmosErrorRegistry(moduleABI, append(module, module[0]), shared, overrides), "duplicate module mapping")
	require.ErrorContains(t, ValidateCosmosErrorRegistry(moduleABI, module, append(shared, shared[0]), overrides), "duplicate shared mapping")
	require.ErrorContains(t, ValidateCosmosErrorRegistry(moduleABI, shared, shared, overrides), "module/shared ownership overlap")
	require.ErrorContains(t, ValidateCosmosErrorRegistry(moduleABI, CosmosErrorMappings{{Key: module[0].Key, SolidityError: "Missing"}}, shared, overrides), "missing ABI error")
	require.ErrorContains(t, ValidateCosmosErrorRegistry(moduleABI, module, shared, OverrideDeclarations{{
		ShadowedKey:       overrides[0].ShadowedKey,
		SoliditySignature: "TypedOverride(uint256)",
		OwningABI:         "TestI",
		Rationale:         "trusted call context",
	}}), "stable source anchor")
	require.ErrorContains(t, ValidateCosmosErrorRegistry(moduleABI, module, shared, OverrideDeclarations{{
		ShadowedKey:       overrides[0].ShadowedKey,
		SoliditySignature: "TypedOverride(uint256)",
		CallSiteAnchor:    "precompiles/common/cosmos_errors.go#OverrideDeclarations.ForABI",
		Rationale:         "trusted call context",
	}}), "exactly one owning ABI")
	require.ErrorContains(t, ValidateCosmosErrorRegistry(moduleABI, module, shared, OverrideDeclarations{{
		ShadowedKey:       overrides[0].ShadowedKey,
		SoliditySignature: "TypedOverride(uint256)",
		OwningABI:         "TestI",
		CallSiteAnchor:    "precompiles/common/cosmos_errors_test.go:54-80",
		Rationale:         "trusted call context",
	}}), "stable source anchor")
	require.ErrorContains(t, ValidateCosmosErrorRegistry(moduleABI, module, shared, OverrideDeclarations{{
		ShadowedKey:       NewCosmosErrorKey(sdkerrors.ErrInsufficientFunds),
		SoliditySignature: "TypedOverride(uint256)",
		OwningABI:         "TestI",
		CallSiteAnchor:    "precompiles/common/cosmos_errors.go#OverrideDeclarations.ForABI",
		Rationale:         "trusted call context",
	}}), "does not shadow exactly one lower-tier mapping")
	require.ErrorContains(t, ValidateCosmosErrorRegistry(moduleABI, module, shared, append(overrides, overrides[0])), "duplicate override ownership/signature")
	require.ErrorContains(t, ValidateCosmosErrorRegistry(moduleABI, module, shared, append(overrides, OverrideDeclaration{
		ShadowedKey:       overrides[0].ShadowedKey,
		SoliditySignature: "AlternateOverride(uint256)",
		OwningABI:         "TestI",
		CallSiteAnchor:    "precompiles/common/cosmos_errors.go#OverrideDeclarations.ForABI",
		Rationale:         "alternate trusted call context",
	})), "duplicate override shadow")
}

func TestCosmosErrorRegistryValidationRejectsPipeDelimitedOverrideOwners(t *testing.T) {
	overrides := OverrideDeclarations{{
		SoliditySignature: "TypedOverride(uint256)",
		OwningABI:         "TestI|OtherI",
		CallSiteAnchor:    "precompiles/common/cosmos_errors.go#OverrideDeclarations.ForABI",
		Rationale:         "invalid multi-owner declaration",
	}}

	require.EqualError(
		t,
		ValidateCosmosErrorRegistry(abi.ABI{}, nil, nil, overrides),
		"override TypedOverride(uint256) requires exactly one owning ABI",
	)
}

func TestCosmosErrorRegistryValidationRejectsOverrideSignatureMissingFromEffectiveABI(t *testing.T) {
	overrides := OverrideDeclarations{{
		SoliditySignature: "MissingOverride(uint256)",
		OwningABI:         "TestI",
		CallSiteAnchor:    "precompiles/common/cosmos_errors.go#OverrideDeclarations.ForABI",
		Rationale:         "signature must exist in the effective ABI",
	}}

	require.EqualError(
		t,
		ValidateCosmosErrorRegistry(abi.ABI{}, nil, nil, overrides),
		"override signature must have exactly one ABI owner: MissingOverride(uint256)",
	)
}

func TestTranslateCosmosError(t *testing.T) {
	moduleABI := mustTestABI(t, `[
		{"type":"error","name":"ModuleFailure","inputs":[]},
		{"type":"error","name":"SDKUnauthorized","inputs":[]},
		{"type":"error","name":"UnmappedCosmosError","inputs":[{"name":"codespace","type":"string"},{"name":"code","type":"uint32"}]}
	]`)
	module := CosmosErrorMappings{NewCosmosErrorMapping(errPhaseOneSynthetic, "ModuleFailure")}
	registry, err := NewCosmosErrorRegistry(
		moduleABI,
		module,
		CosmosErrorMappings{NewCosmosErrorMapping(sdkerrors.ErrUnauthorized, SolidityErrSDKUnauthorized)},
		nil,
	)
	require.NoError(t, err)

	translation := TranslateCosmosError(moduleABI, registry, errorsmod.Wrap(sdkerrors.ErrUnauthorized, "unstable text"))
	require.Equal(t, MappingKindSharedSDK, translation.Kind)
	require.False(t, translation.IsUnmapped)
	require.Equal(t, errorSelector(moduleABI, "SDKUnauthorized"), translation.Revert.(RevertDataCarrier).RevertData()[:4])

	translation = TranslateCosmosError(moduleABI, registry, errPhaseOneSynthetic)
	require.Equal(t, MappingKindModule, translation.Kind)
	require.Equal(t, errorSelector(moduleABI, "ModuleFailure"), translation.Revert.(RevertDataCarrier).RevertData()[:4])

	unmapped := errorsmod.Register("phase-one-unmapped", 8, "unmapped")
	translation = TranslateCosmosError(moduleABI, registry, unmapped)
	require.True(t, translation.IsUnmapped)
	require.Equal(t, CosmosErrorKey{"phase-one-unmapped", 8}, translation.Key)
	require.Equal(t, errorSelector(moduleABI, "UnmappedCosmosError"), translation.Revert.(RevertDataCarrier).RevertData()[:4])
	decoded, err := moduleABI.Errors["UnmappedCosmosError"].Inputs.Unpack(translation.Revert.(RevertDataCarrier).RevertData()[4:])
	require.NoError(t, err)
	require.Equal(t, []interface{}{"phase-one-unmapped", uint32(8)}, decoded)
	firstRevert := translation.Revert.(RevertDataCarrier).RevertData()
	repeated := TranslateCosmosError(moduleABI, registry, unmapped)
	require.Equal(t, firstRevert, repeated.Revert.(RevertDataCarrier).RevertData())

	plain := errors.New("infrastructure failure")
	translation = TranslateCosmosError(moduleABI, registry, plain)
	require.Equal(t, MappingKindInternal, translation.Kind)
	require.ErrorIs(t, translation.Revert, plain)
}

func TestCosmosErrorRegistryFreezesDeclarationInputs(t *testing.T) {
	moduleABI := mustTestABI(t, `[
		{"type":"error","name":"ModuleFailure","inputs":[]},
		{"type":"error","name":"SDKUnauthorized","inputs":[]},
		{"type":"error","name":"UnmappedCosmosError","inputs":[{"name":"codespace","type":"string"},{"name":"code","type":"uint32"}]}
	]`)
	moduleDeclarations := CosmosErrorMappings{NewCosmosErrorMapping(errPhaseOneSynthetic, "ModuleFailure")}
	sharedDeclarations := CosmosErrorMappings{NewCosmosErrorMapping(sdkerrors.ErrUnauthorized, "SDKUnauthorized")}
	registry, err := NewCosmosErrorRegistry(moduleABI, moduleDeclarations, sharedDeclarations, nil)
	require.NoError(t, err)

	moduleDeclarations[0].SolidityError = "SDKUnauthorized"
	sharedDeclarations[0].Key = NewCosmosErrorKey(sdkerrors.ErrInvalidRequest)

	moduleTranslation := TranslateCosmosError(moduleABI, registry, errPhaseOneSynthetic)
	require.Equal(t, MappingKindModule, moduleTranslation.Kind)
	require.Equal(t, errorSelector(moduleABI, "ModuleFailure"), moduleTranslation.Revert.(RevertDataCarrier).RevertData()[:4])

	sharedTranslation := TranslateCosmosError(moduleABI, registry, sdkerrors.ErrUnauthorized)
	require.Equal(t, MappingKindSharedSDK, sharedTranslation.Kind)
	require.Equal(t, errorSelector(moduleABI, "SDKUnauthorized"), sharedTranslation.Revert.(RevertDataCarrier).RevertData()[:4])
}

func TestSharedSDKErrorMappingsReturnsCopy(t *testing.T) {
	first := SharedSDKErrorMappings()
	require.NotEmpty(t, first)
	first[0].Key = NewCosmosErrorKey(sdkerrors.ErrInvalidRequest)
	first[0].SolidityError = SolidityErrSDKInvalidRequest

	second := SharedSDKErrorMappings()
	require.Equal(t, NewCosmosErrorKey(sdkerrors.ErrUnauthorized), second[0].Key)
	require.Equal(t, SolidityErrSDKUnauthorized, second[0].SolidityError)

	registry := MustNewCosmosErrorRegistry(SharedErrorABI, nil, SharedSDKErrorMappings(), nil)
	translation := TranslateCosmosError(SharedErrorABI, registry, sdkerrors.ErrUnauthorized)
	require.Equal(t, MappingKindSharedSDK, translation.Kind)
	require.Equal(t, errorSelector(SharedErrorABI, SolidityErrSDKUnauthorized), translation.Revert.(RevertDataCarrier).RevertData()[:4])
}

func TestApprovedOverrideDeclarationsReturnsCopy(t *testing.T) {
	first := ApprovedOverrideDeclarations()
	require.NotEmpty(t, first)
	first[0].SoliditySignature = "Mutated()"
	first[0].CallSiteAnchor = "precompiles/common/cosmos_errors.go#mutated"

	second := ApprovedOverrideDeclarations()
	require.Equal(t, "ERC20InsufficientBalance(address,uint256,uint256)", second[0].SoliditySignature)
	require.Equal(t, "precompiles/erc20/tx.go#Precompile.transfer", second[0].CallSiteAnchor)
}

func TestGRPCErrorDispositionsAreMessageIndependent(t *testing.T) {
	table := GRPCErrorDispositions{
		{Boundary: ErrorBoundaryQueryServer, Method: "delegation", Code: codes.NotFound, Kind: GRPCDispositionPreserveSuccess},
		{
			Boundary: ErrorBoundaryMsgServer, Method: "cancelUnbondingDelegation", Code: codes.NotFound,
			Kind: GRPCDispositionSolidityError, SolidityError: "StakingUnbondingDelegationNotFound",
			SoliditySignature: "StakingUnbondingDelegationNotFound()", OwningABI: "StakingI",
			Selector: [4]byte{0x46, 0x41, 0xdb, 0x46},
		},
	}
	registry, err := NewGRPCErrorRegistry(table)
	require.NoError(t, err)

	for _, message := range []string{"not found", "completely changed"} {
		got, ok := registry.Resolve(ErrorBoundaryQueryServer, "delegation", status.Error(codes.NotFound, message))
		require.True(t, ok)
		require.Equal(t, GRPCDispositionPreserveSuccess, got.Kind)
	}
	got, ok := registry.Resolve(ErrorBoundaryMsgServer, "cancelUnbondingDelegation", status.Error(codes.NotFound, "changed"))
	require.True(t, ok)
	require.Equal(t, "StakingUnbondingDelegationNotFound", got.SolidityError)
	require.Equal(t, "StakingUnbondingDelegationNotFound()", got.SoliditySignature)
	require.Equal(t, "StakingI", got.OwningABI)
	require.Equal(t, [4]byte{0x46, 0x41, 0xdb, 0x46}, got.Selector)
	_, ok = registry.Resolve(ErrorBoundaryQueryServer, "delegation", status.Error(codes.InvalidArgument, "not found"))
	require.False(t, ok)

	preserved := TranslateGRPCError(abi.ABI{}, *registry, ErrorBoundaryQueryServer, "delegation", status.Error(codes.NotFound, "changed again"))
	require.True(t, preserved.Matched)
	require.True(t, preserved.PreserveSuccess)
	require.NoError(t, preserved.Revert)

	for _, message := range []string{"not found", "transport text changed"} {
		reviewed, ok := ReviewedGRPCErrorRegistry().Resolve(
			ErrorBoundaryMsgServer,
			"cancelUnbondingDelegation",
			status.Error(codes.NotFound, message),
		)
		require.True(t, ok)
		require.Equal(t, "StakingUnbondingDelegationNotFound", reviewed.SolidityError)
		require.Equal(t, "StakingUnbondingDelegationNotFound()", reviewed.SoliditySignature)
		require.Equal(t, "StakingI", reviewed.OwningABI)
		require.Equal(t, [4]byte{0x46, 0x41, 0xdb, 0x46}, reviewed.Selector)
	}
}

func TestGRPCErrorRegistryFreezesConstructorInput(t *testing.T) {
	declarations := GRPCErrorDispositions{{
		Boundary: ErrorBoundaryQueryServer,
		Method:   "delegation",
		Code:     codes.NotFound,
		Kind:     GRPCDispositionPreserveSuccess,
	}}
	registry, err := NewGRPCErrorRegistry(declarations)
	require.NoError(t, err)

	declarations[0].Method = "mutated"
	declarations[0].Kind = GRPCDispositionInternal

	got, ok := registry.Resolve(ErrorBoundaryQueryServer, "delegation", status.Error(codes.NotFound, "changed"))
	require.True(t, ok)
	require.Equal(t, GRPCDispositionPreserveSuccess, got.Kind)
	_, ok = registry.Resolve(ErrorBoundaryQueryServer, "mutated", status.Error(codes.NotFound, "changed"))
	require.False(t, ok)
}

func TestGRPCErrorDispositionABIValidation(t *testing.T) {
	contractABI := mustTestABI(t, `[
		{"type":"error","name":"StakingUnbondingDelegationNotFound","inputs":[]}
	]`)
	concrete := GRPCErrorDisposition{
		Boundary: ErrorBoundaryMsgServer, Method: "cancelUnbondingDelegation", Code: codes.NotFound,
		Kind: GRPCDispositionSolidityError, SolidityError: "StakingUnbondingDelegationNotFound",
		SoliditySignature: "StakingUnbondingDelegationNotFound()", OwningABI: "StakingI",
		Selector: [4]byte{0x46, 0x41, 0xdb, 0x46},
	}

	registry, err := NewGRPCErrorRegistry(GRPCErrorDispositions{concrete})
	require.NoError(t, err)
	require.NoError(t, registry.ValidateABI(contractABI, "StakingI"))
	preservedRegistry, err := NewGRPCErrorRegistry(GRPCErrorDispositions{{
		Boundary: ErrorBoundaryQueryServer, Method: "delegation", Code: codes.NotFound,
		Kind: GRPCDispositionPreserveSuccess,
	}})
	require.NoError(t, err)
	require.NoError(t, preservedRegistry.ValidateABI(abi.ABI{}, "StakingI"))
	require.ErrorContains(t, registry.ValidateABI(abi.ABI{}, "StakingI"), "missing ABI error")

	badSignature := concrete
	badSignature.SoliditySignature = "StakingUnbondingDelegationNotFound(uint256)"
	badSignatureRegistry, err := NewGRPCErrorRegistry(GRPCErrorDispositions{badSignature})
	require.NoError(t, err)
	require.ErrorContains(t, badSignatureRegistry.ValidateABI(contractABI, "StakingI"), "signature mismatch")

	badSelector := concrete
	badSelector.Selector = [4]byte{0xde, 0xad, 0xbe, 0xef}
	badSelectorRegistry, err := NewGRPCErrorRegistry(GRPCErrorDispositions{badSelector})
	require.NoError(t, err)
	require.ErrorContains(t, badSelectorRegistry.ValidateABI(contractABI, "StakingI"), "selector mismatch")
}

func TestStaticRegistryValidatorPanics(t *testing.T) {
	shared := SharedSDKErrorMappings()
	require.Panics(t, func() {
		MustNewCosmosErrorRegistry(SharedErrorABI, nil, append(shared, shared[0]), nil)
	})
}

func TestStaticRegistryValidatorRejectsSharedABIDrift(t *testing.T) {
	tests := map[string]struct {
		mutate    func(abi.ABI)
		panicText string
	}{
		"missing shared error": {
			mutate: func(contractABI abi.ABI) {
				delete(contractABI.Errors, SolidityErrUnmappedCosmosError)
			},
			panicText: "missing inherited shared ABI error UnmappedCosmosError",
		},
		"altered shared signature": {
			mutate: func(contractABI abi.ABI) {
				definition := contractABI.Errors[SolidityErrSDKUnauthorized]
				definition.Sig = "SDKUnauthorized(uint256)"
				contractABI.Errors[SolidityErrSDKUnauthorized] = definition
			},
			panicText: "inherited shared ABI error mismatch for SDKUnauthorized",
		},
		"altered shared selector": {
			mutate: func(contractABI abi.ABI) {
				definition := contractABI.Errors[SolidityErrSDKUnauthorized]
				definition.ID[0] ^= 0xff
				contractABI.Errors[SolidityErrSDKUnauthorized] = definition
			},
			panicText: "inherited shared ABI error mismatch for SDKUnauthorized",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			contractABI := mustTestABI(t, sharedErrorABIJSON)
			tc.mutate(contractABI)
			require.PanicsWithError(t, tc.panicText, func() {
				MustNewCosmosErrorRegistry(contractABI, nil, SharedSDKErrorMappings(), nil)
			})
		})
	}
}

func TestEffectiveABIValidationRejectsSignatureAndSelectorCollisions(t *testing.T) {
	contractABI := mustTestABI(t, `[{"type":"error","name":"First","inputs":[]},{"type":"error","name":"Second","inputs":[]}]`)
	first := contractABI.Errors["First"]
	second := contractABI.Errors["Second"]
	second.Sig = first.Sig
	contractABI.Errors["Second"] = second
	require.ErrorContains(t, validateEffectiveABI(contractABI), "duplicate ABI signature")

	second.Sig = "Second()"
	second.ID = first.ID
	contractABI.Errors["Second"] = second
	require.ErrorContains(t, validateEffectiveABI(contractABI), "duplicate ABI selector")
}

func TestSharedABIContainsApprovedErrors(t *testing.T) {
	for _, name := range []string{
		"SDKUnauthorized", "SDKInsufficientFunds", "SDKInvalidAddress", "SDKInvalidCoins",
		"SDKInvalidRequest", "SDKInvalidType", "SDKNotFound", "UnmappedCosmosError",
	} {
		_, ok := SharedErrorABI.Errors[name]
		require.True(t, ok, name)
	}
}

func mustTestABI(t *testing.T, raw string) abi.ABI {
	t.Helper()
	parsed, err := abi.JSON(strings.NewReader(raw))
	require.NoError(t, err)
	return parsed
}

func errorSelector(contractABI abi.ABI, name string) []byte {
	definition := contractABI.Errors[name]
	return definition.ID[:4]
}
