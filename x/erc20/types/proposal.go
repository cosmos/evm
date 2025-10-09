package types

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cosmos/evm/utils"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	v1beta1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

// constants
const (
	// ProposalTypeRegisterCoin is DEPRECATED, remove after v16 upgrade
	ProposalTypeRegisterCoin          string = "RegisterCoin"
	ProposalTypeRegisterERC20         string = "RegisterERC20"
	ProposalTypeToggleTokenConversion string = "ToggleTokenConversion" // #nosec
	Erc20NativeCoinDenomPrefix        string = "erc20:"                // #nosec
)

// Implements Proposal Interface
var (
	// RegisterCoinProposal is DEPRECATED, remove after v16 upgrade
	_ v1beta1.Content = &RegisterCoinProposal{}
	_ v1beta1.Content = &RegisterERC20Proposal{}
	_ v1beta1.Content = &ToggleTokenConversionProposal{}
)

func init() {
	v1beta1.RegisterProposalType(ProposalTypeRegisterERC20)
	v1beta1.RegisterProposalType(ProposalTypeToggleTokenConversion)
}

// CreateDenomDescription generates a string with the coin description
func CreateDenomDescription(address string) string {
	return fmt.Sprintf("Cosmos coin token representation of %s", address)
}

// CreateDenom generates a string the module name plus the address to avoid conflicts with names staring with a number
func CreateDenom(address string) string {
	return Erc20NativeCoinDenomPrefix + address
}

// ValidateErc20Denom checks if a denom is a valid erc20 denomination.
// Only the "erc20:0xabc..." format is accepted.
func ValidateErc20Denom(denom string) error {
	if strings.HasPrefix(denom, Erc20NativeCoinDenomPrefix) {
		trimmed := strings.TrimPrefix(denom, Erc20NativeCoinDenomPrefix)
		if len(trimmed) == 0 {
			return fmt.Errorf("invalid denom (given: %s): missing address after prefix %s", denom, Erc20NativeCoinDenomPrefix)
		}
		return utils.ValidateAddress(trimmed)
	}

	return fmt.Errorf("invalid denom (given: %s): denomination should be prefixed with %s", denom, Erc20NativeCoinDenomPrefix)
}

// NewRegisterERC20Proposal returns new instance of RegisterERC20Proposal
func NewRegisterERC20Proposal(title, description string, erc20Addreses ...string) v1beta1.Content {
	return &RegisterERC20Proposal{
		Title:          title,
		Description:    description,
		Erc20Addresses: erc20Addreses,
	}
}

// ProposalRoute returns router key for this proposal
func (*RegisterERC20Proposal) ProposalRoute() string { return RouterKey }

// ProposalType returns proposal type for this proposal
func (*RegisterERC20Proposal) ProposalType() string {
	return ProposalTypeRegisterERC20
}

// ValidateBasic performs a stateless check of the proposal fields
func (rtbp *RegisterERC20Proposal) ValidateBasic() error {
	for _, address := range rtbp.Erc20Addresses {
		if err := utils.ValidateAddress(address); err != nil {
			return errorsmod.Wrap(err, "ERC20 address")
		}
	}

	return v1beta1.ValidateAbstract(rtbp)
}

// NewToggleTokenConversionProposal returns new instance of ToggleTokenConversionProposal
func NewToggleTokenConversionProposal(title, description string, token string) v1beta1.Content {
	return &ToggleTokenConversionProposal{
		Title:       title,
		Description: description,
		Token:       token,
	}
}

// ProposalRoute returns router key for this proposal
func (*ToggleTokenConversionProposal) ProposalRoute() string { return RouterKey }

// ProposalType returns proposal type for this proposal
func (*ToggleTokenConversionProposal) ProposalType() string {
	return ProposalTypeToggleTokenConversion
}

// ValidateBasic performs a stateless check of the proposal fields
func (ttcp *ToggleTokenConversionProposal) ValidateBasic() error {
	// check if the token is a hex address, if not, check if it is a valid SDK
	// denom
	if err := utils.ValidateAddress(ttcp.Token); err != nil {
		if err := sdk.ValidateDenom(ttcp.Token); err != nil {
			return err
		}
	}

	return v1beta1.ValidateAbstract(ttcp)
}

// ProposalRoute returns router key for this proposal.
// RegisterCoinProposal is DEPRECATED remove after v16 upgrade
func (*RegisterCoinProposal) ProposalRoute() string { return RouterKey }

// ProposalType returns proposal type for this proposal.
// RegisterCoinProposal is DEPRECATED remove after v16 upgrade
func (*RegisterCoinProposal) ProposalType() string {
	return ProposalTypeRegisterCoin
}

// ValidateBasic performs a stateless check of the proposal fields.
// RegisterCoinProposal is DEPRECATED remove after v16 upgrade
func (rtbp *RegisterCoinProposal) ValidateBasic() error {
	return errors.New("deprecated")
}
