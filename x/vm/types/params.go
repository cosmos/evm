package types

import (
	"fmt"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"

	"github.com/cosmos/evm/utils"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v10/modules/core/24-host"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	// DefaultEVMDenom is the default value for the evm denom
	DefaultEVMDenom = "uatom"
	// DefaultEVMExtendedDenom is the default value for the evm extended denom
	DefaultEVMExtendedDenom = "aatom"
	// DefaultEVMDisplayDenom is the default value for the display denom in the bank metadata
	DefaultEVMDisplayDenom = "atom"
	// DefaultEVMChainID is the default value for the evm chain ID
	DefaultEVMChainID uint64 = 262144
	// DefaultEVMDecimals is the default value for the evm denom decimal precision
	DefaultEVMDecimals uint64 = 18
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

const DefaultHistoryServeWindow = 8192 // same as EIP-2935

// NewParams creates a new Params instance
func NewParams(
	extraEIPs []int64,
	activeStaticPrecompiles,
	evmChannels []string,
	accessControl AccessControl,
) Params {
	return Params{
		ExtraEIPs:               extraEIPs,
		ActiveStaticPrecompiles: activeStaticPrecompiles,
		EVMChannels:             evmChannels,
		AccessControl:           accessControl,
	}
}

// DefaultParams returns default evm parameters
func DefaultParams() Params {
	return Params{
		EvmDenom:                sdk.DefaultBondDenom,
		ExtraEIPs:               DefaultExtraEIPs,
		ActiveStaticPrecompiles: DefaultStaticPrecompiles,
		EVMChannels:             DefaultEVMChannels,
		AccessControl:           DefaultAccessControl,
		HistoryServeWindow:      DefaultHistoryServeWindow,
		ExtendedDenomOptions:    &ExtendedDenomOptions{ExtendedDenom: sdk.DefaultBondDenom},
	}
}

// validateChannels checks if channels ids are valid
func validateChannels(channels []string) error {
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

func validateAccessType(accessType AccessType) error {
	switch accessType {
	case AccessTypePermissionless, AccessTypeRestricted, AccessTypePermissioned:
		return nil
	default:
		return fmt.Errorf("invalid access type: %s", accessType)
	}
}

func validateAllowlistAddresses(addresses []string) error {
	for _, address := range addresses {
		if err := utils.ValidateAddress(address); err != nil {
			return fmt.Errorf("invalid whitelist address: %s", address)
		}
	}
	return nil
}

func validateEIPs(eips []int64) error {
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
func ValidatePrecompiles(precompiles []string) error {
	seenPrecompiles := make(map[string]struct{})
	for _, precompile := range precompiles {
		if _, ok := seenPrecompiles[precompile]; ok {
			return fmt.Errorf("duplicate precompile %s", precompile)
		}

		if err := utils.ValidateAddress(precompile); err != nil {
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
