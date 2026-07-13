package evm

import (
	"fmt"
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// EthSigVerificationResultCacheKey caches sig verification per incarnation,
// since the result is a pure function of the tx bytes and signer.
const EthSigVerificationResultCacheKey = "ante:EthSigVerificationResult"

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

// AnteHandle validates that the registered chain id is the same as the one on the message, and
// that the signer address matches the one defined on the message.
// Failure in RecheckTx will prevent tx to be included into block, especially when CheckTx succeed, in which case user
// won't see the error message.
func (esvd EthSigVerificationDecorator) AnteHandle(ctx sdk.Context, tx sdk.Tx, simulate bool, next sdk.AnteHandler) (newCtx sdk.Context, err error) {
	if err := verifyEthSigCached(ctx, func() error {
		ethCfg := evmtypes.GetEthChainConfig()
		blockNum := big.NewInt(ctx.BlockHeight())
		signer := ethtypes.MakeSigner(ethCfg, blockNum, uint64(ctx.BlockTime().Unix())) //#nosec G115 -- int overflow is not a concern here

		msgs := tx.GetMsgs()
		if msgs == nil {
			return errorsmod.Wrap(errortypes.ErrUnknownRequest, "invalid transaction. Transaction without messages")
		}

		for _, msg := range msgs {
			msgEthTx, ok := msg.(*evmtypes.MsgEthereumTx)
			if !ok {
				return errorsmod.Wrapf(errortypes.ErrUnknownRequest, "invalid message type %T, expected %T", msg, (*evmtypes.MsgEthereumTx)(nil))
			}
			if err := SignatureVerification(msgEthTx, msgEthTx.AsTransaction(), signer); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return ctx, err
	}

	return next(ctx, tx, simulate)
}

// verifyEthSigCached runs verify memoized in the incarnation cache, skipping it
// when the app already verified the tx via ctx.IsSigverifyTx() (mirrors x/auth).
func verifyEthSigCached(ctx sdk.Context, verify func() error) error {
	if !ctx.IsSigverifyTx() {
		// skip ecrecover — already verified these tx bytes
		return nil
	}
	if v, ok := ctx.GetIncarnationCache(EthSigVerificationResultCacheKey); ok {
		if v == nil {
			return nil
		}
		cachedErr, ok := v.(error)
		if !ok {
			return fmt.Errorf("unexpected type %T cached under %s, want error", v, EthSigVerificationResultCacheKey)
		}
		return cachedErr
	}
	err := verify()
	ctx.SetIncarnationCache(EthSigVerificationResultCacheKey, err)
	return err
}

// SignatureVerification checks that the registered chain id is the same as the one on the message, and
// that the message's `From` field (populated at decode time) matches sender recovered from
// signature of Ethereum transaction. It only verifies `From`, it does not set it.
func SignatureVerification(msg *evmtypes.MsgEthereumTx, _ *ethtypes.Transaction, signer ethtypes.Signer) error {
	if err := msg.VerifySender(signer); err != nil {
		return errorsmod.Wrapf(errortypes.ErrorInvalidSigner, "signature verification failed: %s", err.Error())
	}

	return nil
}
