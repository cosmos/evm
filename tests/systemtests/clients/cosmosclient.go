package clients

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"
	"time"

	coretypes "github.com/cometbft/cometbft/v2/rpc/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/tests/systemtests/config"

	rpchttp "github.com/cometbft/cometbft/v2/rpc/client/http"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type CosmosClient struct {
	ChainID    string
	ClientCtx  client.Context
	RpcClients map[string]*rpchttp.HTTP
	Accs       map[string]*CosmosAccount
}

func NewCosmosClient(t *testing.T, config *config.Config) (*CosmosClient, error) {
	clientCtx, err := newClientContext(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create client context: %v", err)
	}

	rpcClients := make(map[string]*rpchttp.HTTP, 0)
	for i, nodeUrl := range config.NodeRPCUrls {
		rpcClient, err := client.NewClientFromNode(nodeUrl)
		if err != nil {
			return nil, fmt.Errorf("failed to connect rpc server: %v", err)
		}

		rpcClients[fmt.Sprintf("node%v", i)] = rpcClient
	}

	accs := make(map[string]*CosmosAccount, 0)
	for i, privKeyHex := range config.PrivKeys {
		priv, err := crypto.HexToECDSA(privKeyHex)

		privKey := &ethsecp256k1.PrivKey{Key: crypto.FromECDSA(priv)}
		addr := sdk.AccAddress(privKey.PubKey().Address().Bytes())

		if err != nil {
			return nil, err
		}
		acc := &CosmosAccount{
			AccAddress:    addr,
			AccountNumber: uint64(i + 1),
			PrivKey:       privKey,
		}
		accs[fmt.Sprintf("acc%v", i)] = acc
	}

	return &CosmosClient{
		ChainID:    config.ChainID,
		ClientCtx:  *clientCtx,
		RpcClients: rpcClients,
		Accs:       accs,
	}, nil
}

func (c *CosmosClient) BankSend(nodeID, accID string, from, to sdk.AccAddress, amount sdkmath.Int, nonce uint64, gasPrice *big.Int) (*sdk.TxResponse, error) {
	c.ClientCtx = c.ClientCtx.WithClient(c.RpcClients[nodeID])

	privKey := c.Accs[accID].PrivKey
	accountNumber := c.Accs[accID].AccountNumber

	msg := banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewCoin("atest", amount)))

	txBytes, err := c.signMsgsV2(privKey, accountNumber, nonce, gasPrice, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to sign tx msg: %v", err)
	}

	resp, err := c.ClientCtx.BroadcastTx(txBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast tx: %v", err)
	}

	return resp, err
}

func (c *CosmosClient) WaitForCommit(
	nodeID string,
	txHash string,
	timeout time.Duration,
) (*coretypes.ResultTx, error) {
	c.ClientCtx = c.ClientCtx.WithClient(c.RpcClients[nodeID])

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	hashBytes, err := hex.DecodeString(txHash)
	if err != nil {
		return nil, fmt.Errorf("invalid tx hash format: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction %s", txHash)
		case <-ticker.C:
			result, err := c.ClientCtx.Client.Tx(ctx, hashBytes, false)
			if err != nil {
				continue
			}

			return result, nil
		}
	}
}

func newClientContext(config *config.Config) (*client.Context, error) {
	// Create codec and tx config
	interfaceRegistry := types.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	marshaler := codec.NewProtoCodec(interfaceRegistry)
	txConfig := tx.NewTxConfig(marshaler, tx.DefaultSignModes)

	// Create client context
	clientCtx := client.Context{
		BroadcastMode:     flags.BroadcastSync,
		TxConfig:          txConfig,
		Codec:             marshaler,
		InterfaceRegistry: interfaceRegistry,
		ChainID:           config.ChainID,
		AccountRetriever:  authtypes.AccountRetriever{},
	}

	return &clientCtx, nil
}

func (c *CosmosClient) signMsgsV2(privKey cryptotypes.PrivKey, accountNumber, sequence uint64, gasPrice *big.Int, msg sdk.Msg) ([]byte, error) {
	senderAddr := sdk.AccAddress(privKey.PubKey().Address().Bytes())
	signMode := signing.SignMode_SIGN_MODE_DIRECT

	txBuilder := c.ClientCtx.TxConfig.NewTxBuilder()
	txBuilder.SetMsgs(msg)
	txBuilder.SetFeePayer(senderAddr)

	signerData := xauthsigning.SignerData{
		Address:       senderAddr.String(),
		ChainID:       c.ChainID,
		AccountNumber: accountNumber,
		Sequence:      sequence,
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

	err := txBuilder.SetSignatures(sigsV2)
	if err != nil {
		return nil, fmt.Errorf("failed to set empty signatures: %v", err)
	}

	txBuilder.SetGasLimit(1_000_000)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin("atest", sdkmath.NewIntFromBigInt(gasPrice).MulRaw(1_000_001))))

	sigV2, err := clienttx.SignWithPrivKey(
		context.Background(),
		signMode,
		signerData,
		txBuilder,
		privKey,
		c.ClientCtx.TxConfig,
		sequence,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign with private key: %v", err)
	}

	err = txBuilder.SetSignatures(sigV2)
	if err != nil {
		return nil, fmt.Errorf("failed to set signatures: %v", err)
	}

	txBytes, err := c.ClientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	if err != nil {
		return nil, fmt.Errorf("failed to encode tx: %v", err)
	}

	return txBytes, nil
}
