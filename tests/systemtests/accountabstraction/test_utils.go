package accountabstraction

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

func createSetCodeAuthorization(chainID, nonce uint64, contractAddr common.Address) ethtypes.SetCodeAuthorization {
	return ethtypes.SetCodeAuthorization{
		ChainID: *uint256.NewInt(chainID),
		Address: contractAddr,
		Nonce:   nonce,
	}
}

func signSetCodeAuthorization(key *ecdsa.PrivateKey, authorization ethtypes.SetCodeAuthorization) (ethtypes.SetCodeAuthorization, error) {
	authorization, err := ethtypes.SignSetCode(key, authorization)
	if err != nil {
		return ethtypes.SetCodeAuthorization{}, fmt.Errorf("failed to sign set code authorization: %w", err)
	}

	return authorization, nil
}
