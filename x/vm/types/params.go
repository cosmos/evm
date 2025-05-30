package types

import (
	"fmt"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"

	"github.com/cosmos/evm/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v10/modules/core/24-host"

	errorsmod "cosmossdk.io/errors"
)

var (
	// DefaultEVMDenom is the default value for the evm denom
	DefaultEVMDenom = "atest"
	// DefaultEVMChainID is the default value for the evm chain ID
	DefaultEVMChainID = "cosmos_262144-1"
	// DefaultEVMDecimals is the default value for the evm denom decimal precision
	DefaultEVMDecimals uint64 = 18
	// DefaultAllowUnprotectedTxs rejects all unprotected txs (i.e false)
	DefaultAllowUnprotectedTxs = false
	// DefaultStaticPrecompiles defines the default active precompiles.
	DefaultStaticPrecompiles []string
	// DefaultExtraEIPs defines the default extra EIPs to be included.
	DefaultExtraEIPs []int64
	// DefaultEVMChannels defines a list of IBC channels that connect to EVM chains like injective or cronos.
	DefaultEVMChannels              []string
	DefaultCreateAllowlistAddresses []string
	DefaultCallAllowlistAddresses   []string
	DefaultAccessControl            = AccessControl{
		Create: AccessControlType{
			AccessType:        AccessTypePermissionless,
			AccessControlList: DefaultCreateAllowlistAddresses,
		},
		Call: AccessControlType{
			AccessType:        AccessTypePermissionless,
			AccessControlList: DefaultCallAllowlistAddresses,
		},
	}
)

// NewParams creates a new Params instance
func NewParams(
	allowUnprotectedTxs bool,
	extraEIPs []int64,
	activeStaticPrecompiles,
	evmChannels []string,
	accessControl AccessControl,
) Params {
	return Params{
		AllowUnprotectedTxs:     allowUnprotectedTxs,
		ExtraEIPs:               extraEIPs,
		ActiveStaticPrecompiles: activeStaticPrecompiles,
		EVMChannels:             evmChannels,
		AccessControl:           accessControl,
	}
}

// DefaultParams returns default evm parameters
func DefaultParams() Params {
	return Params{
		EvmDenom:                DefaultEVMDenom,
		ExtraEIPs:               DefaultExtraEIPs,
		AllowUnprotectedTxs:     DefaultAllowUnprotectedTxs,
		ActiveStaticPrecompiles: DefaultStaticPrecompiles,
		EVMChannels:             DefaultEVMChannels,
		AccessControl:           DefaultAccessControl,
	}
}

// validateChannels checks if channels ids are valid
func validateChannels(i interface{}) error {
	channels, ok := i.([]string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	for _, channel := range channels {
		if err := host.ChannelIdentifierValidator(channel); err != nil {
			return errorsmod.Wrap(
				channeltypes.ErrInvalidChannelIdentifier, err.Error(),
			)
		}
	}

	return nil
}

// Validate performs basic validation on evm parameters.
func (p Params) Validate() error {
	if err := validateEIPs(p.ExtraEIPs); err != nil {
		return err
	}

	if err := validateBool(p.AllowUnprotectedTxs); err != nil {
		return err
	}

	if err := ValidatePrecompiles(p.ActiveStaticPrecompiles); err != nil {
		return err
	}

	if err := p.AccessControl.Validate(); err != nil {
		return err
	}

	return validateChannels(p.EVMChannels)
}

// EIPs returns the ExtraEIPS as a int slice
func (p Params) EIPs() []int {
	eips := make([]int, len(p.ExtraEIPs))
	for i, eip := range p.ExtraEIPs {
		eips[i] = int(eip)
	}
	return eips
}

// GetActiveStaticPrecompilesAddrs is a util function that the Active Precompiles
// as a slice of addresses.
func (p Params) GetActiveStaticPrecompilesAddrs() []common.Address {
	precompiles := make([]common.Address, len(p.ActiveStaticPrecompiles))
	for i, precompile := range p.ActiveStaticPrecompiles {
		precompiles[i] = common.HexToAddress(precompile)
	}
	return precompiles
}

// IsEVMChannel returns true if the channel provided is in the list of
// EVM channels
func (p Params) IsEVMChannel(channel string) bool {
	return slices.Contains(p.EVMChannels, channel)
}

func (ac AccessControl) Validate() error {
	if err := ac.Create.Validate(); err != nil {
		return err
	}
	return ac.Call.Validate()
}

func (act AccessControlType) Validate() error {
	if err := validateAccessType(act.AccessType); err != nil {
		return err
	}
	return validateAllowlistAddresses(act.AccessControlList)
}

func validateAccessType(i interface{}) error {
	accessType, ok := i.(AccessType)
	if !ok {
		return fmt.Errorf("invalid access type type: %T", i)
	}

	switch accessType {
	case AccessTypePermissionless, AccessTypeRestricted, AccessTypePermissioned:
		return nil
	default:
		return fmt.Errorf("invalid access type: %s", accessType)
	}
}

func validateAllowlistAddresses(i interface{}) error {
	addresses, ok := i.([]string)
	if !ok {
		return fmt.Errorf("invalid whitelist addresses type: %T", i)
	}

	for _, address := range addresses {
		if err := types.ValidateAddress(address); err != nil {
			return fmt.Errorf("invalid whitelist address: %s", address)
		}
	}
	return nil
}

func validateBool(i interface{}) error {
	_, ok := i.(bool)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}

func validateEIPs(i interface{}) error {
	eips, ok := i.([]int64)
	if !ok {
		return fmt.Errorf("invalid EIP slice type: %T", i)
	}

	uniqueEIPs := make(map[int64]struct{})

	for _, eip := range eips {
		if !vm.ValidEip(int(eip)) {
			return fmt.Errorf("EIP %d is not activateable, valid EIPs are: %s", eip, vm.ActivateableEips())
		}

		if _, ok := uniqueEIPs[eip]; ok {
			return fmt.Errorf("found duplicate EIP: %d", eip)
		}
		uniqueEIPs[eip] = struct{}{}

	}

	return nil
}

// ValidatePrecompiles checks if the precompile addresses are valid and unique.
func ValidatePrecompiles(i interface{}) error {
	precompiles, ok := i.([]string)
	if !ok {
		return fmt.Errorf("invalid precompile slice type: %T", i)
	}

	seenPrecompiles := make(map[string]struct{})
	for _, precompile := range precompiles {
		if _, ok := seenPrecompiles[precompile]; ok {
			return fmt.Errorf("duplicate precompile %s", precompile)
		}

		if err := types.ValidateAddress(precompile); err != nil {
			return fmt.Errorf("invalid precompile %s", precompile)
		}

		seenPrecompiles[precompile] = struct{}{}
	}

	// NOTE: Check that the precompiles are sorted. This is required
	// to ensure determinism
	if !slices.IsSorted(precompiles) {
		return fmt.Errorf("precompiles need to be sorted: %s", precompiles)
	}

	return nil
}

// IsLondon returns if london hardfork is enabled.
func IsLondon(ethConfig *params.ChainConfig, height int64) bool {
	return ethConfig.IsLondon(big.NewInt(height))
}
