package factory

import (
	"context"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/client"
	cosmostx "github.com/cosmos/cosmos-sdk/client/tx"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
)

// setSignatures is a helper function that sets the signature for
// the transaction in the tx builder. It returns the signerData to be used
// when signing the transaction (e.g. when calling signWithPrivKey)
func (tf *baseTxFactory) setSignatures(privKey cryptotypes.PrivKey, nonce *uint64, txBuilder client.TxBuilder, signMode signing.SignMode) (signerData authsigning.SignerData, err error) {
	senderAddress := sdktypes.AccAddress(privKey.PubKey().Address().Bytes())
	account, err := tf.grpcHandler.GetAccount(senderAddress.String())
	if err != nil {
		return signerData, err
	}
	sequence := account.GetSequence()
	if nonce != nil {
		sequence = *nonce
	}

	signerData = authsigning.SignerData{
		ChainID:       tf.network.GetChainID(),
		AccountNumber: account.GetAccountNumber(),
		Sequence:      sequence,
		Address:       senderAddress.String(),
		PubKey:        privKey.PubKey(),
	}

	sigsV2 := signing.SignatureV2{
		PubKey: privKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signMode,
			Signature: nil,
		},
		Sequence: sequence,
	}

	return signerData, txBuilder.SetSignatures(sigsV2)
}

// setMultiSignerSignatures sets nil signatures for each provided signer in
// order so the tx can be simulated, returning the per-signer SignerData used
// during the actual signing pass.
func (tf *baseTxFactory) setMultiSignerSignatures(privKeys []cryptotypes.PrivKey, txBuilder client.TxBuilder, signMode signing.SignMode) ([]authsigning.SignerData, error) {
	signerDatas := make([]authsigning.SignerData, len(privKeys))
	sigsV2 := make([]signing.SignatureV2, len(privKeys))
	for i, privKey := range privKeys {
		addr := sdktypes.AccAddress(privKey.PubKey().Address().Bytes())
		account, err := tf.grpcHandler.GetAccount(addr.String())
		if err != nil {
			return nil, err
		}
		seq := account.GetSequence()
		signerDatas[i] = authsigning.SignerData{
			ChainID:       tf.network.GetChainID(),
			AccountNumber: account.GetAccountNumber(),
			Sequence:      seq,
			Address:       addr.String(),
			PubKey:        privKey.PubKey(),
		}
		sigsV2[i] = signing.SignatureV2{
			PubKey: privKey.PubKey(),
			Data: &signing.SingleSignatureData{
				SignMode:  signMode,
				Signature: nil,
			},
			Sequence: seq,
		}
	}
	if err := txBuilder.SetSignatures(sigsV2...); err != nil {
		return nil, err
	}
	return signerDatas, nil
}

// signMultiSignerWithPrivKeys produces a signature per signer and sets them
// all on the tx builder in the same order as privKeys.
func (tf *baseTxFactory) signMultiSignerWithPrivKeys(privKeys []cryptotypes.PrivKey, txBuilder client.TxBuilder, signerDatas []authsigning.SignerData, signMode signing.SignMode) error {
	txConfig := tf.ec.TxConfig
	sigs := make([]signing.SignatureV2, len(privKeys))
	for i, privKey := range privKeys {
		sig, err := cosmostx.SignWithPrivKey(context.TODO(), signMode, signerDatas[i], txBuilder, privKey, txConfig, signerDatas[i].Sequence)
		if err != nil {
			return errorsmod.Wrap(err, "failed to sign tx")
		}
		sigs[i] = sig
	}
	return txBuilder.SetSignatures(sigs...)
}

// signWithPrivKey is a helper function that signs a transaction
// with the provided private key
func (tf *baseTxFactory) signWithPrivKey(privKey cryptotypes.PrivKey, txBuilder client.TxBuilder, signerData authsigning.SignerData, signMode signing.SignMode) error {
	txConfig := tf.ec.TxConfig
	signature, err := cosmostx.SignWithPrivKey(context.TODO(), signMode, signerData, txBuilder, privKey, txConfig, signerData.Sequence)
	if err != nil {
		return errorsmod.Wrap(err, "failed to sign tx")
	}

	return txBuilder.SetSignatures(signature)
}
