package types

import (
	"crypto/ecdsa"

	"github.com/ethereum/go-ethereum/common"
)

type Account struct {
	Address common.Address
	PrivKey *ecdsa.PrivateKey
}
