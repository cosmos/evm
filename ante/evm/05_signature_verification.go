package evm

import (
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// EthSigVerificationDecorator validates an ethereum signatures
type EthSigVerificationDecorator struct {
	evmKeeper anteinterfaces.EVMKeeper
}

// NewEthSigVerificationDecorator creates a new EthSigVerificationDecorator
func NewEthSigVerificationDecorator(ek anteinterfaces.EVMKeeper) EthSigVerificationDecorator {
	return EthSigVerificationDecorator{
		evmKeeper: ek,
	}
}

// AnteHandle validates checks that the registered chain id is the same as the one on the message, and
// that the signer address matches the one defined on the message.
// It's not skipped for RecheckTx, because it set `From` address which is critical from other ante handler to work.
// Failure in RecheckTx will prevent tx to be included into block, especially when CheckTx succeed, in which case user
// won't see the error message.
func (esvd EthSigVerificationDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	ethCfg := evmtypes.GetEthChainConfig()
	blockNum := big.NewInt(ctx.BlockHeight())
	signer := ethtypes.MakeSigner(ethCfg, blockNum, uint64(ctx.BlockTime().Unix())) //#nosec G115 -- int overflow is not a concern here

	msgs := tx.GetMsgs()
	if msgs == nil {
		return ctx, errorsmod.Wrap(errortypes.ErrUnknownRequest, "invalid transaction. Transaction without messages")
	}

	for _, msg := range msgs {
		msgEthTx, ok := msg.(*evmtypes.MsgEthereumTx)
		if !ok {
			return ctx, errorsmod.Wrapf(errortypes.ErrUnknownRequest, "invalid message type %T, expected %T", msg, (*evmtypes.MsgEthereumTx)(nil))
		}

		err := SignatureVerification(msgEthTx, msgEthTx.AsTransaction(), signer)
		if err != nil {
			return ctx, err
		}
	}

	return next(ctx, tx, simulate)
}

// SignatureVerification checks that the registered chain id is the same as the one on the message, and
// that the signer address matches the one defined on the message.
// The function set the field from of the given message equal to the sender
// computed from the signature of the Ethereum transaction.
func SignatureVerification(
	msg *evmtypes.MsgEthereumTx,
	ethTx *ethtypes.Transaction,
	signer ethtypes.Signer,
) error {
	if err := msg.VerifySender(signer); err != nil {
		return errorsmod.Wrapf(errortypes.ErrorInvalidSigner, "signature verification failed: %s", err.Error())
	}

	return nil
}
