package clients

import (
	"crypto/ecdsa"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/ethereum/go-ethereum/common"
)

type EthAccount struct {
	Address common.Address
	PrivKey *ecdsa.PrivateKey
}

type CosmosAccount struct {
	AccAddress    sdk.AccAddress
	AccountNumber uint64
	PrivKey       *ethsecp256k1.PrivKey
}
