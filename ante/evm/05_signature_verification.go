package evm

import (
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// SignatureVerification checks that the registered chain id is the same as the one on the message, and
// that the signer address matches the one defined on the message.
// The function set the field from of the given message equal to the sender
// computed from the signature of the Ethereum transaction.
func SignatureVerification(msg *evmtypes.MsgEthereumTx, _ *ethtypes.Transaction, signer ethtypes.Signer) error {
	if err := msg.VerifySender(signer); err != nil {
		return errorsmod.Wrapf(errortypes.ErrorInvalidSigner, "signature verification failed: %s", err.Error())
	}

	return nil
}
