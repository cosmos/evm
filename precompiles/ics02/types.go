package ics02

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"

	cmn "github.com/cosmos/evm/precompiles/common"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
)

// height is a struct used to parse the ProofHeight parameter used as input
// in the VerifyMembership and VerifyNonMembership methods.
type height struct {
	ProofHeight clienttypes.Height
}

// ParseGetClientStateArgs parses the arguments for the GetClientState method.
func ParseGetClientStateArgs(args []interface{}) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	clientID, ok := args[0].(string)
	if !ok {
		return "", fmt.Errorf("invalid client id: %v", args[0])
	}

	return clientID, nil
}

// ParseUpdateClientArgs parses the arguments for the UpdateClient method.
func ParseUpdateClientArgs(args []interface{}) (string, []byte, error) {
	if len(args) != 2 {
		return "", nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	clientID, ok := args[0].(string)
	if !ok {
		return "", nil, fmt.Errorf("invalid client id: %v", args[0])
	}
	updateBytes, ok := args[1].([]byte)
	if !ok {
		return "", nil, fmt.Errorf("invalid update client bytes: %v", args[1])
	}

	return clientID, updateBytes, nil
}

// ParseVerifyMembershipArgs parses the arguments for the VerifyMembership method.
func ParseVerifyMembershipArgs(method *abi.Method, args []interface{}) (string, []byte, clienttypes.Height, [][]byte, []byte, error) {
	if len(args) != 5 {
		return "", nil, clienttypes.Height{}, nil, nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 5, len(args))
	}

	clientID, ok := args[0].(string)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, nil, fmt.Errorf("invalid client id: %v", args[0])
	}
	proof, ok := args[1].([]byte)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, nil, fmt.Errorf("invalid proof bytes: %v", args[1])
	}

	var proofHeight height
	heightArg := abi.Arguments{method.Inputs[2]}
	if err := heightArg.Copy(&proofHeight, []interface{}{args[2]}); err != nil {
		return "", nil, clienttypes.Height{}, nil, nil, fmt.Errorf("error while unpacking args to TransferInput struct: %s", err)
	}

	path, ok := args[3].([][]byte)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, nil, fmt.Errorf("invalid path: %v", args[3])
	}

	value, ok := args[4].([]byte)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, nil, fmt.Errorf("invalid value: %v", args[4])
	}

	return clientID, proof, proofHeight.ProofHeight, path, value, nil
}

// ParseVerifyNonMembershipArgs parses the arguments for the VerifyNonMembership method.
func ParseVerifyNonMembershipArgs(method *abi.Method, args []interface{}) (string, []byte, clienttypes.Height, [][]byte, error) {
	if len(args) != 4 {
		return "", nil, clienttypes.Height{}, nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 4, len(args))
	}

	clientID, ok := args[0].(string)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, fmt.Errorf("invalid client id: %v", args[0])
	}
	proof, ok := args[1].([]byte)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, fmt.Errorf("invalid proof bytes: %v", args[1])
	}

	var proofHeight height
	heightArg := abi.Arguments{method.Inputs[2]}
	if err := heightArg.Copy(&proofHeight, []interface{}{args[2]}); err != nil {
		return "", nil, clienttypes.Height{}, nil, fmt.Errorf("error while unpacking args to TransferInput struct: %s", err)
	}

	// TODO: make sure path is deserilized like this
	path, ok := args[3].([][]byte)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, fmt.Errorf("invalid path: %v", args[3])
	}

	return clientID, proof, proofHeight.ProofHeight, path, nil
}
