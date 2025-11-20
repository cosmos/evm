//go:build system_test

package eip712

import (
	"context"
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/cosmos/evm/ethereum/eip712"
	"github.com/cosmos/evm/tests/systemtests/clients"
)

// BankSendWithEIP712 sends a bank send transaction using EIP-712 signing.
func BankSendWithEIP712(
	cosmosClient *clients.CosmosClient,
	account *clients.CosmosAccount,
	nodeID string,
	from, to sdk.AccAddress,
	amount sdkmath.Int,
	nonce uint64,
	gasPrice *big.Int,
) (*sdk.TxResponse, error) {
	cosmosClient.ClientCtx = cosmosClient.ClientCtx.WithClient(cosmosClient.RpcClients[nodeID])

	privKey := account.PrivKey

	// Query account number from chain
	accountInfo, err := cosmosClient.ClientCtx.AccountRetriever.GetAccount(cosmosClient.ClientCtx, from)
	if err != nil {
		return nil, fmt.Errorf("failed to get account: %v", err)
	}
	accountNumber := accountInfo.GetAccountNumber()

	msg := banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewCoin("atest", amount)))

	txBytes, err := signMsgsWithEIP712(cosmosClient, privKey, accountNumber, nonce, gasPrice, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to sign tx msg with EIP-712: %v", err)
	}

	resp, err := cosmosClient.ClientCtx.BroadcastTx(txBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast tx: %v", err)
	}

	// This debug string is useful for transactions that don't yield an error until after they're broadcasted to the chain
	fmt.Printf("DEBUG: CosmosClient BankSend: %s\n", resp.String())

	return resp, nil
}

// signMsgsWithEIP712 signs the provided messages using EIP-712 and returns the signed transaction bytes.
func signMsgsWithEIP712(
	cosmosClient *clients.CosmosClient,
	privKey cryptotypes.PrivKey,
	accountNumber, sequence uint64,
	gasPrice *big.Int,
	msg sdk.Msg,
) ([]byte, error) {
	senderAddr := sdk.AccAddress(privKey.PubKey().Address().Bytes())
	signMode := signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON

	txBuilder := cosmosClient.ClientCtx.TxConfig.NewTxBuilder()

	txBuilder.SetGasLimit(150_000)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin("atest", sdkmath.NewIntFromBigInt(gasPrice).MulRaw(150_001))))

	err := txBuilder.SetMsgs(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to set messages: %v", err)
	}

	signerData := xauthsigning.SignerData{
		Address:       senderAddr.String(),
		ChainID:       cosmosClient.ChainID,
		AccountNumber: accountNumber,
		Sequence:      sequence,
		PubKey:        privKey.PubKey(),
	}

	// Set empty signature first
	sigsV2 := signing.SignatureV2{
		PubKey: privKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signMode,
			Signature: nil,
		},
		Sequence: sequence,
	}

	err = txBuilder.SetSignatures(sigsV2)
	if err != nil {
		return nil, fmt.Errorf("failed to set empty signatures: %v", err)
	}

	// Get sign bytes
	signBytes, err := xauthsigning.GetSignBytesAdapter(
		context.Background(),
		cosmosClient.ClientCtx.TxConfig.SignModeHandler(),
		signMode,
		signerData,
		txBuilder.GetTx(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get sign bytes: %v", err)
	}

	// Get EIP-712 bytes for the message
	eip712Bytes, err := eip712.GetEIP712BytesForMsg(signBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to get EIP-712 bytes: %v", err)
	}

	// Sign the EIP-712 hash
	signature, err := privKey.Sign(eip712Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to sign EIP-712 bytes: %v", err)
	}

	// Set the signature
	sigsV2 = signing.SignatureV2{
		PubKey: privKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signMode,
			Signature: signature,
		},
		Sequence: sequence,
	}

	err = txBuilder.SetSignatures(sigsV2)
	if err != nil {
		return nil, fmt.Errorf("failed to set signatures: %v", err)
	}

	txBytes, err := cosmosClient.ClientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("failed to encode tx: %v", err)
	}

	return txBytes, nil
}
