package testutil

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/tx"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/x/authz"
)

func GeneratePrivKeyAddressPairs(accCount int) ([]*ethsecp256k1.PrivKey, []sdk.AccAddress, error) {
	var (
		err           error
		testPrivKeys  = make([]*ethsecp256k1.PrivKey, accCount)
		testAddresses = make([]sdk.AccAddress, accCount)
	)

	for i := range testPrivKeys {
		testPrivKeys[i], err = ethsecp256k1.GenerateKey()
		if err != nil {
			return nil, nil, err
		}
		testAddresses[i] = testPrivKeys[i].PubKey().Address().Bytes()
	}
	return testPrivKeys, testAddresses, nil
}

func NewMsgExec(grantee sdk.AccAddress, msgs []sdk.Msg) *authz.MsgExec {
	msg := authz.NewMsgExec(grantee, msgs)
	return &msg
}

func NewMsgGrant(granter sdk.AccAddress, grantee sdk.AccAddress, a authz.Authorization, expiration *time.Time) *authz.MsgGrant {
	msg, err := authz.NewMsgGrant(granter, grantee, a, expiration)
	if err != nil {
		panic(err)
	}
	return msg
}

func CreateNestedMsgExec(a sdk.AccAddress, nestedLvl int, lastLvlMsgs []sdk.Msg) *authz.MsgExec {
	msgs := make([]*authz.MsgExec, nestedLvl)
	for i := range msgs {
		if i == 0 {
			msgs[i] = NewMsgExec(a, lastLvlMsgs)
			continue
		}
		msgs[i] = NewMsgExec(a, []sdk.Msg{msgs[i-1]})
	}
	return msgs[nestedLvl-1]
}

func CreateTx(ctx context.Context, txCfg client.TxConfig, priv cryptotypes.PrivKey, msgs ...sdk.Msg) (sdk.Tx, error) {
	txBuilder := txCfg.NewTxBuilder()
	defaultSignMode, err := authsigning.APISignModeToInternal(txCfg.SignModeHandler().DefaultMode())
	if err != nil {
		return nil, err
	}

	txBuilder.SetGasLimit(1000000)
	if err := txBuilder.SetMsgs(msgs...); err != nil {
		return nil, err
	}

	// First round: we gather all the signer infos. We use the "set empty
	// signature" hack to do that.
	sigV2 := signing.SignatureV2{
		PubKey: priv.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  defaultSignMode,
			Signature: nil,
		},
		Sequence: 0,
	}

	if err := txBuilder.SetSignatures(sigV2); err != nil {
		return nil, err
	}

	signerData := authsigning.SignerData{
		Address:       sdk.AccAddress(priv.PubKey().Bytes()).String(),
		ChainID:       "chainID",
		AccountNumber: 0,
		Sequence:      0,
		PubKey:        priv.PubKey(),
	}

	sigV2, err = tx.SignWithPrivKey(
		ctx, defaultSignMode, signerData,
		txBuilder, priv, txCfg,
		0,
	)
	if err != nil {
		return nil, err
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return nil, err
	}

	return txBuilder.GetTx(), nil
}

// DecodeRevertReason extracts and decodes the human-readable revert reason from an EVM transaction response.
// It processes the raw return data (Ret field) from a failed EVM transaction and attempts to decode
// any ABI-encoded revert messages into readable error strings.
//
// Returns:
//   - error: A formatted error containing either:
//   - "tx failed with VmError: <vmError>: <decoded_reason>" for successfully decoded reverts
//   - "tx failed with VmError: <vmError>: <hex_data>" for non-decodable data
//   - "failed to decode revert data: <decode_error>" if decoding fails
//
// Example usage:
//
//	res, err := executeTransaction(...)
//	if res.VmError != "" {
//	    decodedErr := DecodeRevertReason(res)
//	    // decodedErr might be: "tx failed with VmError: execution reverted: ERC20: insufficient allowance"
//	}
func DecodeRevertReason(evmRes evmtypes.MsgEthereumTxResponse) error {
	revertErr := evmtypes.NewExecErrorWithReason(evmRes.Ret)
	hexData, ok := revertErr.ErrorData().(string)
	if ok {
		decodedBytes, err := hexutil.Decode(hexData)
		if err == nil {
			if len(decodedBytes) >= 4 && bytes.Equal(decodedBytes[:4], evmtypes.RevertSelector) {
				var reason string
				reason, err = abi.UnpackRevert(decodedBytes)
				if err == nil {
					return fmt.Errorf("tx failed with VmError: %v: %s", evmRes.VmError, reason)
				}
			}
		}
		return errorsmod.Wrap(err, "failed to decode revert data")
	}
	return fmt.Errorf("tx failed with VmError: %v: %s", evmRes.VmError, revertErr.ErrorData())
}
