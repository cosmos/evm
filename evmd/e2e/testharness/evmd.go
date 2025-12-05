package testharness

import (
	"context"
	"crypto/ecdsa"
	"fmt"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	eapp "github.com/cosmos/evm/evmd/app"
	"github.com/cosmos/evm/evmd/app/params"
	"github.com/cosmos/evm/evmd/e2e/utils"
	"github.com/ethereum/go-ethereum/crypto"

	"cosmossdk.io/log"

	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func evmdEncodingConfig() params.EncodingConfig {
	// we "pre"-instantiate the application for getting the injected/configured encoding configuration
	tempApp := eapp.New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.EmptyAppOptions{},
	)
	return params.EncodingConfig{
		InterfaceRegistry: tempApp.InterfaceRegistry(),
		Codec:             tempApp.AppCodec(),
		TxConfig:          tempApp.GetTxConfig(),
		Amino:             tempApp.LegacyAmino(),
	}
}

func (c *Chain) BroadcastSdkMgs(ctx context.Context, key *ecdsa.PrivateKey, gas uint64, msgs ...sdk.Msg) (*sdk.TxResponse, error) {
	txBuilder := c.EVMEncodingConfig.TxConfig.NewTxBuilder()

	if err := txBuilder.SetMsgs(msgs...); err != nil {
		return nil, err
	}
	txBuilder.SetGasLimit(gas)

	// TODO: This seems like a very high fee, but otherwise txs get rejected for insufficient fee
	// This should be configured better and calculated based on `gas`
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin(utils.TestDenom, 10_000_000_000_000_000)))

	// Sign Tx
	pKey := ethsecp256k1.PrivKey{Key: crypto.FromECDSA(key)}
	senderAddr := crypto.PubkeyToAddress(key.PublicKey)
	senderBech32, err := bech32.ConvertAndEncode(utils.TestBech32Prefix, senderAddr.Bytes())
	if err != nil {
		return nil, err
	}

	authQueryClient := authtypes.NewQueryClient(c.GrpcClient)
	res, err := authQueryClient.AccountInfo(ctx, &authtypes.QueryAccountInfoRequest{Address: senderBech32})
	if err != nil {
		return nil, fmt.Errorf("failed to get account info: %w", err)
	}

	signingData := authsigning.SignerData{
		Address:       senderBech32,
		ChainID:       utils.TestChainID,
		AccountNumber: res.Info.AccountNumber,
		Sequence:      res.Info.Sequence,
		PubKey:        pKey.PubKey(),
	}

	// First round: we gather all the signer infos. We use the "set empty
	// signature" hack to do that.
	sigV2Empty := signing.SignatureV2{
		PubKey: pKey.PubKey(),
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		},
		Sequence: res.Info.Sequence,
	}
	if err := txBuilder.SetSignatures(sigV2Empty); err != nil {
		return nil, fmt.Errorf("failed to set empty tx signature: %w", err)
	}

	sigV2, err := clienttx.SignWithPrivKey(
		ctx, signing.SignMode_SIGN_MODE_DIRECT, signingData,
		txBuilder, &pKey, c.EVMEncodingConfig.TxConfig,
		res.Info.Sequence,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign tx: %w", err)
	}

	if err := txBuilder.SetSignatures(sigV2); err != nil {
		return nil, fmt.Errorf("failed to set tx signature: %w", err)
	}

	// Broadcast Tx

	txJSONBytes, _ := c.EVMEncodingConfig.TxConfig.TxJSONEncoder()(txBuilder.GetTx())
	fmt.Println("Broadcasting transaction:", string(txJSONBytes))

	txBytes, err := c.EVMEncodingConfig.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("failed to encode tx: %w", err)
	}

	txClient := tx.NewServiceClient(c.GrpcClient)
	grpcRes, err := txClient.BroadcastTx(
		ctx,
		&tx.BroadcastTxRequest{
			Mode:    tx.BroadcastMode_BROADCAST_MODE_SYNC,
			TxBytes: txBytes, // Proto-binary of the signed transaction, see previous step.
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast tx: %w", err)
	}

	if grpcRes.TxResponse.Code != 0 {
		return nil, fmt.Errorf("tx failed with code %d: %s", grpcRes.TxResponse.Code, grpcRes.TxResponse.RawLog)
	}

	return grpcRes.TxResponse, nil
}
