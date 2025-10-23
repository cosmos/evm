package ics02

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/solidity-ibc-eureka/packages/go-abigen/sp1ics07tendermint"
)

// ParseGetClientStateArgs parses the arguments for the GetClientState method.
func ParseGetClientStateArgs(args []interface{}) error {
	if len(args) != 0 {
		return fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 0, len(args))
	}
	return nil
}

// ParseUpdateClientArgs parses the arguments for the UpdateClient method.
func ParseUpdateClientArgs(args []interface{}) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	updateBytes, ok := args[0].([]byte)
	if !ok {
		return nil, fmt.Errorf("invalid update client bytes: %v", args[0])
	}
	return updateBytes, nil
}

// ParseVerifyMembershipArgs parses the arguments for the VerifyMembership method.
func ParseVerifyMembershipArgs(args []interface{}) (*sp1ics07tendermint.ILightClientMsgsMsgVerifyMembership, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	msg := *abi.ConvertType(args[0], new(sp1ics07tendermint.ILightClientMsgsMsgVerifyMembership)).(*sp1ics07tendermint.ILightClientMsgsMsgVerifyMembership)

	return &msg, nil
}

// ParseVerifyNonMembershipArgs parses the arguments for the VerifyNonMembership method.
func ParseVerifyNonMembershipArgs(args []interface{}) (*sp1ics07tendermint.ILightClientMsgsMsgVerifyNonMembership, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	msg := *abi.ConvertType(args[0], new(sp1ics07tendermint.ILightClientMsgsMsgVerifyNonMembership)).(*sp1ics07tendermint.ILightClientMsgsMsgVerifyNonMembership)

	return &msg, nil
}

// ParseMisbehaviourArgs parses the arguments for the Misbehaviour method.
func ParseMisbehaviourArgs(args []interface{}) ([]byte, error) {
	return ParseUpdateClientArgs(args)
}
