package types

import (
	"github.com/ethereum/go-ethereum/common"

	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

var (
	_ sdk.Msg              = &MsgUpdateParams{}
	_ sdk.Msg              = &MsgRegisterClientPrecompile{}
	_ sdk.HasValidateBasic = &MsgUpdateParams{}
	_ sdk.HasValidateBasic = &MsgRegisterClientPrecompile{}
)

// NewMsgUpdateParams creates a new MsgUpdateParams instance
func NewMsgUpdateParams(authority string, params Params) *MsgUpdateParams {
	return &MsgUpdateParams{
		Authority: authority,
		Params:    params,
	}
}

// NewMsgRegisterClientPrecompile creates a new MsgRegisterClientPrecompile instance
func NewMsgRegisterClientPrecompile(sender, clientID, address string) *MsgRegisterClientPrecompile {
	return &MsgRegisterClientPrecompile{
		ClientId: clientID,
		Address:  address,
		Sender:   sender,
	}
}

func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return errortypes.ErrInvalidAddress.Wrapf("authority address '%s' is invalid: %v", m.Authority, err)
	}
	return nil
}

func (m *MsgRegisterClientPrecompile) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Sender); err != nil {
		return errortypes.ErrInvalidAddress.Wrapf("sender address '%s' is invalid: %v", m.Sender, err)
	}

	if !common.IsHexAddress(m.Address) {
		return errortypes.ErrInvalidAddress.Wrapf("address '%s' is not a valid ethereum hex address", m.Address)
	}

	if !clienttypes.IsValidClientID(m.ClientId) {
		return clienttypes.ErrInvalidClient.Wrapf("client ID '%s' is invalid", m.ClientId)
	}

	return nil
}
