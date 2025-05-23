package types

import (
	"github.com/ethereum/go-ethereum/common"
	protov2 "google.golang.org/protobuf/proto"

	erc20api "github.com/cosmos/evm/api/cosmos/evm/erc20/v1"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	txsigning "cosmossdk.io/x/tx/signing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	_ sdk.Msg              = &MsgConvertERC20{}
	_ sdk.Msg              = &MsgUpdateParams{}
	_ sdk.Msg              = &MsgRegisterERC20{}
	_ sdk.Msg              = &MsgToggleConversion{}
	_ sdk.HasValidateBasic = &MsgConvertERC20{}
	_ sdk.HasValidateBasic = &MsgConvertCoin{}
	_ sdk.HasValidateBasic = &MsgUpdateParams{}
	_ sdk.HasValidateBasic = &MsgRegisterERC20{}
	_ sdk.HasValidateBasic = &MsgToggleConversion{}
)

const (
	TypeMsgConvertERC20 = "convert_ERC20"
	TypeMsgConvertCoin  = "convert_coin"
)

var MsgConvertERC20CustomGetSigner = txsigning.CustomGetSigner{
	MsgType: protov2.MessageName(&erc20api.MsgConvertERC20{}),
	Fn:      erc20api.GetSigners,
}

// NewMsgConvertERC20 creates a new instance of MsgConvertERC20
func NewMsgConvertERC20(amount math.Int, receiver sdk.AccAddress, contract, sender common.Address) *MsgConvertERC20 { //nolint: interfacer
	return &MsgConvertERC20{
		ContractAddress: contract.String(),
		Amount:          amount,
		Receiver:        receiver.String(),
		Sender:          sender.Hex(),
	}
}

// NewMsgConvertERC20 creates a new instance of MsgConvertERC20
func NewMsgConvertCoin(coin sdk.Coin, receiver common.Address, sender sdk.AccAddress) *MsgConvertCoin { //nolint: interfacer
	return &MsgConvertCoin{
		Coin:     coin,
		Receiver: receiver.Hex(),
		Sender:   sender.String(),
	}
}

// Route should return the name of the module
func (msg MsgConvertERC20) Route() string { return RouterKey }

// Type should return the action
func (msg MsgConvertERC20) Type() string { return TypeMsgConvertERC20 }

// ValidateBasic runs stateless checks on the message
func (msg MsgConvertERC20) ValidateBasic() error {
	if !common.IsHexAddress(msg.ContractAddress) {
		return errorsmod.Wrapf(errortypes.ErrInvalidAddress, "invalid contract hex address '%s'", msg.ContractAddress)
	}
	if !msg.Amount.IsPositive() {
		return errorsmod.Wrapf(errortypes.ErrInvalidCoins, "cannot mint a non-positive amount")
	}
	_, err := sdk.AccAddressFromBech32(msg.Receiver)
	if err != nil {
		return errorsmod.Wrap(err, "invalid receiver address")
	}
	if !common.IsHexAddress(msg.Sender) {
		return errorsmod.Wrapf(errortypes.ErrInvalidAddress, "invalid sender hex address %s", msg.Sender)
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg MsgConvertERC20) GetSignBytes() []byte {
	return sdk.MustSortJSON(AminoCdc.MustMarshalJSON(&msg))
}

// ValidateBasic does a sanity check of the provided data
func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return errorsmod.Wrap(err, "Invalid authority address")
	}

	return m.Params.Validate()
}

// GetSignBytes implements the LegacyMsg interface.
func (m MsgUpdateParams) GetSignBytes() []byte {
	return sdk.MustSortJSON(AminoCdc.MustMarshalJSON(&m))
}

// ValidateBasic does a sanity check of the provided data
func (m *MsgRegisterERC20) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Signer)
	if err != nil {
		return errorsmod.Wrap(err, "invalid signer address")
	}

	for _, addr := range m.Erc20Addresses {
		if !common.IsHexAddress(addr) {
			return errortypes.ErrInvalidAddress.Wrapf("invalid ERC20 contract address: %s", addr)
		}
	}
	return nil
}

// ValidateBasic does a sanity check of the provided data
func (m *MsgToggleConversion) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return errorsmod.Wrap(err, "Invalid authority address")
	}

	return nil
}

// Route should return the name of the module
func (msg MsgConvertCoin) Route() string { return RouterKey }

// Type should return the action
func (msg MsgConvertCoin) Type() string { return TypeMsgConvertCoin }

// ValidateBasic runs stateless checks on the message
func (msg MsgConvertCoin) ValidateBasic() error {
	if len(msg.Coin.Denom) == 0 {
		return errorsmod.Wrapf(errortypes.ErrInvalidCoins, "denom cannot be empty")
	}
	if !msg.Coin.Amount.IsPositive() {
		return errorsmod.Wrapf(errortypes.ErrInvalidCoins, "cannot mint a non-positive amount")
	}
	_, err := sdk.AccAddressFromBech32(msg.Sender)
	if err != nil {
		return errorsmod.Wrap(err, "invalid sender address")
	}
	if !common.IsHexAddress(msg.Receiver) {
		return errorsmod.Wrapf(errortypes.ErrInvalidAddress, "invalid receiver hex address %s", msg.Receiver)
	}
	return nil
}

// GetSignBytes encodes the message for signing
func (msg MsgConvertCoin) GetSignBytes() []byte {
	return sdk.MustSortJSON(AminoCdc.MustMarshalJSON(&msg))
}
