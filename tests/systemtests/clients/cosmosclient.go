package clients

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"slices"
	"time"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm/crypto/ethsecp256k1"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	evmencoding "github.com/cosmos/evm/encoding"
)

// CosmosClient is a client for interacting with Cosmos SDK-based nodes.
type CosmosClient struct {
	ChainID    string
	ClientCtx  client.Context
	RpcClients map[string]*rpchttp.HTTP
}

const defaultGenesisAccountCount = 4

// NewCosmosClient creates a new CosmosClient instance and returns it together with the loaded accounts.
func NewCosmosClient() (*CosmosClient, map[string]*CosmosAccount, error) {
	config, err := NewConfig()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load config")
	}

	clientCtx, err := newClientContext(config)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create client context: %v", err)
	}

	rpcClients := make(map[string]*rpchttp.HTTP, 0)
	for i, nodeUrl := range config.NodeRPCUrls {
		rpcClient, err := client.NewClientFromNode(nodeUrl)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to connect rpc server: %v", err)
		}

		rpcClients[fmt.Sprintf("node%v", i)] = rpcClient
	}

	accs := make(map[string]*CosmosAccount, 0)
	for i, privKeyHex := range config.PrivKeys {
		priv, err := crypto.HexToECDSA(privKeyHex)

		privKey := &ethsecp256k1.PrivKey{Key: crypto.FromECDSA(priv)}
		addr := sdk.AccAddress(privKey.PubKey().Address().Bytes())

		if err != nil {
			return nil, nil, err
		}
		// Only the default genesis accounts have deterministic account numbers.
		// Additional test accounts are created on demand by the suite; they start
		// with an account number of zero and will query the chain for their actual
		// value once they exist on-chain.
		accountNumber := uint64(0)
		if i < defaultGenesisAccountCount {
			accountNumber = uint64(i + 1)
		}
		acc := &CosmosAccount{
			AccAddress:    addr,
			AccountNumber: accountNumber,
			PrivKey:       privKey,
		}
		accs[fmt.Sprintf("acc%v", i)] = acc
	}

	return &CosmosClient{
		ChainID:    config.ChainID,
		ClientCtx:  *clientCtx,
		RpcClients: rpcClients,
	}, accs, nil
}

// BankSend sends a bank send transaction from one account to another.
func (c *CosmosClient) BankSend(nodeID string, account *CosmosAccount, from, to sdk.AccAddress, amount sdkmath.Int, nonce uint64, gasPrice *big.Int) (*sdk.TxResponse, error) {
	c.ClientCtx = c.ClientCtx.WithClient(c.RpcClients[nodeID])

	privKey := account.PrivKey
	accountNumber := account.AccountNumber

	msg := banktypes.NewMsgSend(from, to, sdk.NewCoins(sdk.NewCoin("atest", amount)))

	txBytes, err := c.signMsgsV2(privKey, accountNumber, nonce, gasPrice, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to sign tx msg: %v", err)
	}

	resp, err := c.ClientCtx.BroadcastTx(txBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to broadcast tx: %v", err)
	}

	// This debug string is useful for transactions that don't yield an error until after they're broadcasted to the chain
	fmt.Printf("DEBUG: CosmosClient BankSend: %s\n", resp.String())

	return resp, nil
}

// WaitForCommit waits for a transaction to be committed in a block.
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

// CheckTxsPending checks if a transaction is either pending in the mempool or already committed.
func (c *CosmosClient) CheckTxsPending(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for transaction %s", txHash)
		case <-ticker.C:
			result, err := c.UnconfirmedTxs(ctx, nodeID)
			if err == nil {
				pendingTxHashes := make([]string, 0, len(result.Txs))
				for _, tx := range result.Txs {
					pendingTxHashes = append(pendingTxHashes, string(tx.Hash()))
				}

				if slices.Contains(pendingTxHashes, txHash) {
					return nil
				}
			}
		}
	}
}

// UnconfirmedTxs retrieves the list of unconfirmed transactions from the node's mempool.
func (c *CosmosClient) UnconfirmedTxs(ctx context.Context, nodeID string) (*coretypes.ResultUnconfirmedTxs, error) {
	return c.RpcClients[nodeID].UnconfirmedTxs(ctx, nil)
}

// GetBalance retrieves the balance of a given address for a specific denomination.
func (c *CosmosClient) GetBalance(nodeID string, address sdk.AccAddress, denom string) (*big.Int, error) {
	c.ClientCtx = c.ClientCtx.WithClient(c.RpcClients[nodeID])

	queryClient := banktypes.NewQueryClient(c.ClientCtx)

	res, err := queryClient.Balance(context.Background(), &banktypes.QueryBalanceRequest{
		Address: address.String(),
		Denom:   denom,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query balance: %w", err)
	}

	return res.Balance.Amount.BigInt(), nil
}

// newClientContext creates a new client context for the Cosmos SDK.
func newClientContext(config *Config) (*client.Context, error) {
	// Use the encoding config setup which properly initializes EIP-712
	encodingConfig := evmencoding.MakeConfig(config.EVMChainID.Uint64())

	// Register auth module types for account queries
	authtypes.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	// Register bank module types for EIP-712 signing
	// Note: The MakeConfig only registers base SDK and EVM types,
	// but we need bank types for MsgSend transactions
	banktypes.RegisterLegacyAminoCodec(encodingConfig.Amino)
	banktypes.RegisterInterfaces(encodingConfig.InterfaceRegistry)

	// Create client context
	clientCtx := client.Context{
		BroadcastMode:     flags.BroadcastSync,
		TxConfig:          encodingConfig.TxConfig,
		Codec:             encodingConfig.Codec,
		InterfaceRegistry: encodingConfig.InterfaceRegistry,
		ChainID:           config.ChainID,
		AccountRetriever:  authtypes.AccountRetriever{},
	}

	return &clientCtx, nil
}

// signMsgsV2 signs the provided messages using the given private key and returns the signed transaction bytes.
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

	txBuilder.SetGasLimit(150_000)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin("atest", sdkmath.NewIntFromBigInt(gasPrice).MulRaw(150_001))))

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
