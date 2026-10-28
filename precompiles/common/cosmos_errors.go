package common

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"

	errorsmod "cosmossdk.io/errors"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	SolidityErrSDKUnauthorized      = "SDKUnauthorized"
	SolidityErrSDKInsufficientFunds = "SDKInsufficientFunds"
	SolidityErrSDKInvalidAddress    = "SDKInvalidAddress"
	SolidityErrSDKInvalidCoins      = "SDKInvalidCoins"
	SolidityErrSDKInvalidRequest    = "SDKInvalidRequest"
	SolidityErrSDKInvalidType       = "SDKInvalidType"
	SolidityErrSDKNotFound          = "SDKNotFound"
	SolidityErrUnmappedCosmosError  = "UnmappedCosmosError"
)

type CosmosErrorKey struct {
	Codespace string
	Code      uint32
}

type cosmosErrorSentinel interface {
	error
	Codespace() string
	ABCICode() uint32
}

func NewCosmosErrorKey(sentinel cosmosErrorSentinel) CosmosErrorKey {
	return CosmosErrorKey{Codespace: sentinel.Codespace(), Code: sentinel.ABCICode()}
}

// ExtractCosmosErrorKey identifies registered errors without using diagnostic text.
// Cosmos wrappers expose Cause while standard wrappers expose Unwrap, so both chains
// are explored with a hard bound to tolerate malformed wrapper implementations.
func ExtractCosmosErrorKey(err error) (CosmosErrorKey, bool) {
	if err == nil {
		return CosmosErrorKey{}, false
	}

	queue := []error{err}
	for steps := 0; len(queue) > 0 && steps < 128; steps++ {
		current := queue[0]
		queue = queue[1:]
		if current == nil {
			continue
		}
		codespace, code, _ := errorsmod.ABCIInfo(current, false)
		if codespace != errorsmod.UndefinedCodespace && codespace != "" && code != errorsmod.SuccessABCICode {
			return CosmosErrorKey{Codespace: codespace, Code: code}, true
		}
		if next := errors.Unwrap(current); next != nil {
			queue = append(queue, next)
		}
		if causer, ok := current.(interface{ Cause() error }); ok {
			if next := causer.Cause(); next != nil {
				queue = append(queue, next)
			}
		}
	}
	return CosmosErrorKey{}, false
}

type CosmosErrorMapping struct {
	Key           CosmosErrorKey
	SolidityError string
}

type CosmosErrorMappings []CosmosErrorMapping

func (mappings CosmosErrorMappings) Clone() CosmosErrorMappings {
	return append(CosmosErrorMappings(nil), mappings...)
}

func NewCosmosErrorMapping(sentinel cosmosErrorSentinel, solidityError string) CosmosErrorMapping {
	return CosmosErrorMapping{Key: NewCosmosErrorKey(sentinel), SolidityError: solidityError}
}

var sharedSDKErrorMappings = CosmosErrorMappings{
	NewCosmosErrorMapping(sdkerrors.ErrUnauthorized, SolidityErrSDKUnauthorized),
	NewCosmosErrorMapping(sdkerrors.ErrInsufficientFunds, SolidityErrSDKInsufficientFunds),
	NewCosmosErrorMapping(sdkerrors.ErrInvalidAddress, SolidityErrSDKInvalidAddress),
	NewCosmosErrorMapping(sdkerrors.ErrInvalidCoins, SolidityErrSDKInvalidCoins),
	NewCosmosErrorMapping(sdkerrors.ErrInvalidRequest, SolidityErrSDKInvalidRequest),
	NewCosmosErrorMapping(sdkerrors.ErrInvalidType, SolidityErrSDKInvalidType),
	NewCosmosErrorMapping(sdkerrors.ErrNotFound, SolidityErrSDKNotFound),
}

// SharedSDKErrorMappings returns a copy of the reviewed shared SDK declarations.
func SharedSDKErrorMappings() CosmosErrorMappings {
	return sharedSDKErrorMappings.Clone()
}

type OverrideDeclaration struct {
	ShadowedKey       CosmosErrorKey
	SoliditySignature string
	OwningABI         string
	CallSiteAnchor    string
	Rationale         string
}

type OverrideDeclarations []OverrideDeclaration

func (declarations OverrideDeclarations) Clone() OverrideDeclarations {
	return append(OverrideDeclarations(nil), declarations...)
}

func (declarations OverrideDeclarations) ForABI(owner string) OverrideDeclarations {
	filtered := make(OverrideDeclarations, 0, len(declarations))
	for _, declaration := range declarations {
		if declaration.OwningABI == owner {
			filtered = append(filtered, declaration)
		}
	}
	return filtered
}

var approvedOverrideDeclarations = OverrideDeclarations{
	{ShadowedKey: NewCosmosErrorKey(sdkerrors.ErrInsufficientFunds), SoliditySignature: "ERC20InsufficientBalance(address,uint256,uint256)", OwningABI: "ERC20I", CallSiteAnchor: "precompiles/erc20/tx.go#Precompile.transfer", Rationale: "Decoded sender/value and keeper spendable balance"},
	{SoliditySignature: "ERC20InsufficientAllowance(address,uint256,uint256)", OwningABI: "ERC20I", CallSiteAnchor: "precompiles/erc20/tx.go#Precompile.transfer", Rationale: "Decoded spender/value and stored allowance"},
	{SoliditySignature: "ERC20InvalidSender(address)", OwningABI: "ERC20I", CallSiteAnchor: "precompiles/erc20/tx.go#Precompile.Transfer", Rationale: "ERC-6093 call-site validation"},
	{SoliditySignature: "ERC20InvalidReceiver(address)", OwningABI: "ERC20I", CallSiteAnchor: "precompiles/erc20/tx.go#Precompile.Transfer", Rationale: "ERC-6093 call-site validation"},
	{SoliditySignature: "ERC20InvalidApprover(address)", OwningABI: "ERC20I", CallSiteAnchor: "precompiles/erc20/approve.go#Precompile.Approve", Rationale: "ERC-6093 call-site validation"},
	{SoliditySignature: "ERC20InvalidSpender(address)", OwningABI: "ERC20I", CallSiteAnchor: "precompiles/erc20/approve.go#Precompile.Approve", Rationale: "ERC-6093 call-site validation"},
	{ShadowedKey: NewCosmosErrorKey(sdkerrors.ErrInsufficientFunds), SoliditySignature: "ERC20InsufficientBalance(address,uint256,uint256)", OwningABI: "IWERC20", CallSiteAnchor: "precompiles/werc20/tx.go#Precompile.Withdraw", Rationale: "Decoded caller/amount and keeper native balance"},
}

// ApprovedOverrideDeclarations returns a copy of the reviewed call-site
// override declarations. Runtime registries freeze their own validated copies.
func ApprovedOverrideDeclarations() OverrideDeclarations {
	return approvedOverrideDeclarations.Clone()
}

// CosmosErrorRegistry is the immutable runtime lookup built from validated
// precompile and shared SDK declaration tiers.
type CosmosErrorRegistry struct {
	precompile map[CosmosErrorKey]CosmosErrorMapping
	sharedSDK  map[CosmosErrorKey]CosmosErrorMapping
}

func validateCosmosErrorMappings(
	effectiveABI abi.ABI,
	precompileMappings CosmosErrorMappings,
	sharedSDKMappings CosmosErrorMappings,
	overrides OverrideDeclarations,
) (map[CosmosErrorKey]CosmosErrorMapping, map[CosmosErrorKey]CosmosErrorMapping, error) {
	precompile, err := validateMappingTier("precompile", effectiveABI, precompileMappings)
	if err != nil {
		return nil, nil, err
	}
	sharedSDK, err := validateMappingTier("shared SDK", effectiveABI, sharedSDKMappings)
	if err != nil {
		return nil, nil, err
	}
	for key := range precompile {
		if _, ok := sharedSDK[key]; ok {
			return nil, nil, fmt.Errorf("precompile/shared SDK ownership overlap for %s:%d", key.Codespace, key.Code)
		}
	}
	if err := validateEffectiveABI(effectiveABI); err != nil {
		return nil, nil, err
	}
	seenOverrides := make(map[string]struct{}, len(overrides))
	seenShadows := make(map[string]struct{}, len(overrides))
	for _, override := range overrides {
		if override.OwningABI == "" {
			return nil, nil, fmt.Errorf("override %s requires exactly one owning ABI", override.SoliditySignature)
		}
		if strings.Contains(override.OwningABI, "|") {
			return nil, nil, fmt.Errorf("override %s requires exactly one owning ABI", override.SoliditySignature)
		}
		identity := override.OwningABI + "\x00" + override.SoliditySignature
		if _, exists := seenOverrides[identity]; exists {
			return nil, nil, fmt.Errorf("duplicate override ownership/signature for %s %s", override.OwningABI, override.SoliditySignature)
		}
		seenOverrides[identity] = struct{}{}
		if !isStableCallSiteAnchor(override.CallSiteAnchor) {
			return nil, nil, fmt.Errorf("override %s requires stable source anchor", override.SoliditySignature)
		}
		if override.Rationale == "" {
			return nil, nil, fmt.Errorf("override %s requires rationale", override.SoliditySignature)
		}
		matches := 0
		for _, definition := range effectiveABI.Errors {
			if definition.Sig == override.SoliditySignature {
				matches++
			}
		}
		if matches != 1 {
			return nil, nil, fmt.Errorf("override signature must have exactly one ABI owner: %s", override.SoliditySignature)
		}
		if override.ShadowedKey == (CosmosErrorKey{}) {
			continue
		}
		shadowIdentity := fmt.Sprintf("%s\x00%s\x00%d", override.OwningABI, override.ShadowedKey.Codespace, override.ShadowedKey.Code)
		if _, exists := seenShadows[shadowIdentity]; exists {
			return nil, nil, fmt.Errorf("duplicate override shadow for %s %s:%d", override.OwningABI, override.ShadowedKey.Codespace, override.ShadowedKey.Code)
		}
		seenShadows[shadowIdentity] = struct{}{}
		lowerTierOwners := 0
		if _, exists := precompile[override.ShadowedKey]; exists {
			lowerTierOwners++
		}
		if _, exists := sharedSDK[override.ShadowedKey]; exists {
			lowerTierOwners++
		}
		if lowerTierOwners != 1 {
			return nil, nil, fmt.Errorf(
				"override %s does not shadow exactly one lower-tier mapping for %s:%d",
				override.SoliditySignature,
				override.ShadowedKey.Codespace,
				override.ShadowedKey.Code,
			)
		}
	}
	return precompile, sharedSDK, nil
}

func ValidateCosmosErrorRegistry(
	effectiveABI abi.ABI,
	precompileMappings CosmosErrorMappings,
	sharedSDKMappings CosmosErrorMappings,
	overrides OverrideDeclarations,
) error {
	_, _, err := validateCosmosErrorMappings(effectiveABI, precompileMappings, sharedSDKMappings, overrides)
	return err
}

func isStableCallSiteAnchor(anchor string) bool {
	path, symbol, found := strings.Cut(anchor, "#")
	if !found || strings.Contains(path, "..") || !strings.HasPrefix(path, "precompiles/") || !strings.HasSuffix(path, ".go") {
		return false
	}
	receiver, method, found := strings.Cut(symbol, ".")
	return found &&
		!strings.Contains(method, ".") &&
		isAnchorIdentifier(receiver) &&
		isAnchorIdentifier(method)
}

func isAnchorIdentifier(value string) bool {
	for index, character := range value {
		if character == '_' || character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || index > 0 && character >= '0' && character <= '9' {
			continue
		}
		return false
	}
	return value != ""
}

func validateMappingTier(tier string, effectiveABI abi.ABI, mappings CosmosErrorMappings) (map[CosmosErrorKey]CosmosErrorMapping, error) {
	validated := make(map[CosmosErrorKey]CosmosErrorMapping, len(mappings))
	for _, mapping := range mappings {
		if _, exists := validated[mapping.Key]; exists {
			return nil, fmt.Errorf("duplicate %s mapping for %s:%d", tier, mapping.Key.Codespace, mapping.Key.Code)
		}
		definition, exists := effectiveABI.Errors[mapping.SolidityError]
		if !exists {
			return nil, fmt.Errorf("missing ABI error %s for %s mapping", mapping.SolidityError, tier)
		}
		if len(definition.Inputs) != 0 {
			return nil, fmt.Errorf("default mapping %s must be no-argument", mapping.SolidityError)
		}
		validated[mapping.Key] = mapping
	}
	return validated, nil
}

func validateEffectiveABI(effectiveABI abi.ABI) error {
	signatures := make(map[string]string, len(effectiveABI.Errors))
	selectors := make(map[[4]byte]string, len(effectiveABI.Errors))
	for name, definition := range effectiveABI.Errors {
		if previous, exists := signatures[definition.Sig]; exists {
			return fmt.Errorf("duplicate ABI signature %s (%s and %s)", definition.Sig, previous, name)
		}
		signatures[definition.Sig] = name
		var selector [4]byte
		copy(selector[:], definition.ID[:4])
		if previous, exists := selectors[selector]; exists {
			return fmt.Errorf("duplicate ABI selector for %s and %s", previous, name)
		}
		selectors[selector] = name
	}
	return nil
}

// ValidateSharedErrorABI proves that an inheriting ABI contains every shared
// error with the exact canonical signature and selector.
func ValidateSharedErrorABI(effectiveABI abi.ABI) error {
	for name, shared := range SharedErrorABI.Errors {
		inherited, ok := effectiveABI.Errors[name]
		if !ok {
			return fmt.Errorf("missing inherited shared ABI error %s", name)
		}
		if inherited.Sig != shared.Sig || inherited.ID != shared.ID {
			return fmt.Errorf("inherited shared ABI error mismatch for %s", name)
		}
	}
	return validateEffectiveABI(effectiveABI)
}

// MustNewCosmosErrorRegistry validates a precompile ABI and its mappings, then
// retains its mappings. Runtime encoding uses the caller ABI. Invalid setup panics.
func MustNewCosmosErrorRegistry(
	effectiveABI abi.ABI,
	precompileMappings CosmosErrorMappings,
	sharedSDKMappings CosmosErrorMappings,
	overrides OverrideDeclarations,
) *CosmosErrorRegistry {
	if err := ValidateSharedErrorABI(effectiveABI); err != nil {
		panic(err)
	}
	precompile, sharedSDK, err := validateCosmosErrorMappings(effectiveABI, precompileMappings, sharedSDKMappings, overrides)
	if err != nil {
		panic(err)
	}
	if err := validateBoundaryErrors(effectiveABI); err != nil {
		panic(err)
	}
	return &CosmosErrorRegistry{precompile: precompile, sharedSDK: sharedSDK}
}

type MappingKind uint8

const (
	MappingKindInternal MappingKind = iota
	MappingKindPrecompile
	MappingKindSharedSDK
	MappingKindUnmapped
)

type ErrorTranslation struct {
	Revert     error
	Kind       MappingKind
	Key        CosmosErrorKey
	IsUnmapped bool
}

// Translate encodes a Cosmos mapping using the supplied ABI.
func (registry *CosmosErrorRegistry) Translate(api abi.ABI, err error) ErrorTranslation {
	return TranslateCosmosError(api, registry, err)
}

// TranslateCosmosError retains the legacy caller-supplied ABI packing behavior.
func TranslateCosmosError(moduleABI abi.ABI, registry *CosmosErrorRegistry, err error) ErrorTranslation {
	key, ok := ExtractCosmosErrorKey(err)
	if !ok {
		return ErrorTranslation{Revert: err, Kind: MappingKindInternal}
	}
	if mapping, found := registry.precompile[key]; found {
		return ErrorTranslation{Revert: NewRevertWithSolidityError(moduleABI, mapping.SolidityError), Kind: MappingKindPrecompile, Key: key}
	}
	if mapping, found := registry.sharedSDK[key]; found {
		return ErrorTranslation{Revert: NewRevertWithSolidityError(moduleABI, mapping.SolidityError), Kind: MappingKindSharedSDK, Key: key}
	}
	return ErrorTranslation{
		Revert: NewRevertWithSolidityError(moduleABI, SolidityErrUnmappedCosmosError, key.Codespace, key.Code),
		Kind:   MappingKindUnmapped, Key: key, IsUnmapped: true,
	}
}

// QueryError preserves terminal EVM errors and existing Solidity revert data,
// translates registered Cosmos errors, and wraps only internal errors as QueryFailed.
func QueryError(moduleABI abi.ABI, registry *CosmosErrorRegistry, method string, err error) error {
	return registry.ResolveError(moduleABI, err, nil, registry.BoundaryFallback(moduleABI, SolidityErrQueryFailed, method, nil)).Err
}
