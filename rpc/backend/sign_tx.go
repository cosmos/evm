package backend

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"

	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
)

// Sign signs the provided data using the private key of address via Geth's signature standard.
func (b *Backend) Sign(address common.Address, data hexutil.Bytes) (hexutil.Bytes, error) {
	from := sdk.AccAddress(address.Bytes())

	_, err := b.ClientCtx.Keyring.KeyByAddress(from)
	if err != nil {
		b.Logger.Error("failed to find key in keyring", "address", address.String())
		return nil, fmt.Errorf("%s; %s", keystore.ErrNoMatch, err.Error())
	}

	// Sign the requested hash with the wallet
	signature, _, err := b.ClientCtx.Keyring.SignByAddress(from, data, signingtypes.SignMode_SIGN_MODE_TEXTUAL)
	if err != nil {
		b.Logger.Error("keyring.SignByAddress failed", "address", address.Hex())
		return nil, err
	}

	signature[crypto.RecoveryIDOffset] += 27 // Transform V from 0/1 to 27/28 according to the yellow paper
	return signature, nil
}

// SignTypedData signs EIP-712 conformant typed data
func (b *Backend) SignTypedData(address common.Address, typedData apitypes.TypedData) (hexutil.Bytes, error) {
	from := sdk.AccAddress(address.Bytes())

	_, err := b.ClientCtx.Keyring.KeyByAddress(from)
	if err != nil {
		b.Logger.Error("failed to find key in keyring", "address", address.String())
		return nil, fmt.Errorf("%s; %s", keystore.ErrNoMatch, err.Error())
	}

	sigHash, _, err := apitypes.TypedDataAndHash(typedData)
	if err != nil {
		return nil, err
	}

	// Sign the requested hash with the wallet
	signature, _, err := b.ClientCtx.Keyring.SignByAddress(from, sigHash, signingtypes.SignMode_SIGN_MODE_TEXTUAL)
	if err != nil {
		b.Logger.Error("keyring.SignByAddress failed", "address", address.Hex())
		return nil, err
	}

	signature[crypto.RecoveryIDOffset] += 27 // Transform V from 0/1 to 27/28 according to the yellow paper
	return signature, nil
}
