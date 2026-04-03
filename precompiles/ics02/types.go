package ics02

import (
	"fmt"
	"math/big"

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
		return "", cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(int64(len(args))))
	}

	clientID, ok := args[0].(string)
	if !ok {
		return "", cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidClientID, fmt.Sprintf("%v", args[0]))
	}

	return clientID, nil
}

// ParseUpdateClientArgs parses the arguments for the UpdateClient method.
func ParseUpdateClientArgs(args []interface{}) (string, []byte, error) {
	if len(args) != 2 {
		return "", nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(2), big.NewInt(int64(len(args))))
	}

	clientID, ok := args[0].(string)
	if !ok {
		return "", nil, cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidClientID, fmt.Sprintf("%v", args[0]))
	}
	updateBytes, ok := args[1].([]byte)
	if !ok {
		return "", nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("invalid update client bytes: %v", args[1]))
	}

	return clientID, updateBytes, nil
}

// ParseVerifyMembershipArgs parses the arguments for the VerifyMembership method.
func ParseVerifyMembershipArgs(method *abi.Method, args []interface{}) (string, []byte, clienttypes.Height, [][]byte, []byte, error) {
	if len(args) != 5 {
		return "", nil, clienttypes.Height{}, nil, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(5), big.NewInt(int64(len(args))))
	}

	clientID, ok := args[0].(string)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, nil, cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidClientID, fmt.Sprintf("%v", args[0]))
	}
	proof, ok := args[1].([]byte)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, nil, cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidProof, []byte{})
	}

	var proofHeight height
	heightArg := abi.Arguments{method.Inputs[2]}
	if err := heightArg.Copy(&proofHeight, []interface{}{args[2]}); err != nil {
		return "", nil, clienttypes.Height{}, nil, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidHeight, err.Error())
	}

	path, ok := args[3].([][]byte)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, nil, cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidPath, [][]byte{})
	}

	value, ok := args[4].([]byte)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, nil, cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidValue, []byte{})
	}

	return clientID, proof, proofHeight.ProofHeight, path, value, nil
}

// ParseVerifyNonMembershipArgs parses the arguments for the VerifyNonMembership method.
func ParseVerifyNonMembershipArgs(method *abi.Method, args []interface{}) (string, []byte, clienttypes.Height, [][]byte, error) {
	if len(args) != 4 {
		return "", nil, clienttypes.Height{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(4), big.NewInt(int64(len(args))))
	}

	clientID, ok := args[0].(string)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidClientID, fmt.Sprintf("%v", args[0]))
	}
	proof, ok := args[1].([]byte)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidProof, []byte{})
	}

	var proofHeight height
	heightArg := abi.Arguments{method.Inputs[2]}
	if err := heightArg.Copy(&proofHeight, []interface{}{args[2]}); err != nil {
		return "", nil, clienttypes.Height{}, nil, cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrInvalidHeight, err.Error())
	}

	// TODO: make sure path is deserilized like this
	path, ok := args[3].([][]byte)
	if !ok {
		return "", nil, clienttypes.Height{}, nil, cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidPath, [][]byte{})
	}

	return clientID, proof, proofHeight.ProofHeight, path, nil
}
