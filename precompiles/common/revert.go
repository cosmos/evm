package common

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
)

var (
	revertSelector = crypto.Keccak256([]byte("Error(string)"))[:4]
)

// RevertReasonBytes converts a message to ABI-encoded revert bytes.
func RevertReasonBytes(reason string) ([]byte, error) {
	typ, err := abi.NewType("string", "", nil)
	if err != nil {
		return nil, err
	}
	packed, err := (abi.Arguments{{Type: typ}}).Pack(reason)
	if err != nil {
		return nil, err
	}
	bz := make([]byte, 0, len(revertSelector)+len(packed))
	bz = append(bz, revertSelector...)
	bz = append(bz, packed...)
	return bz, nil
}
