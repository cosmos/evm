package utils

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"strings"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	defaultGasLimit   = 300000
	gasPaddingPercent = 20 // 20% padding for gas estimation
	gasPaddingFixed   = 10000
)

// NewClient dials the JSON-RPC endpoint.
func NewClient(ctx context.Context, rpcURL string) (*ethclient.Client, error) {
	return ethclient.DialContext(ctx, rpcURL)
}

// GetBalance returns the latest balance (wei) for the given address.
func GetBalance(ctx context.Context, c *ethclient.Client, addr common.Address) (*big.Int, error) {
	return c.BalanceAt(ctx, addr, nil)
}

// WaitReceipt polls for a transaction receipt until the deadline.
func WaitReceipt(ctx context.Context, c *ethclient.Client, txHash common.Hash, deadline time.Duration) (*types.Receipt, error) {
	var receipt *types.Receipt
	var receiptErr error

	err := WaitForCondition(ctx, func() (bool, string, error) {
		// Try to get the receipt
		rec, err := c.TransactionReceipt(ctx, txHash)
		if err == nil && rec != nil {
			receipt = rec
			receiptErr = nil
			return true, "receipt found", nil
		}
		// Check if this is a genuine error (not just "not found"). Some RPC backends may intermittently time out
		// individual requests even while the node is healthy. Treat such timeouts as transient and keep polling
		// until the overall deadline expires.
		if err != nil && !isTransientReceiptError(err) {
			receiptErr = err
			return true, "", err // Return true to stop polling on genuine errors
		}
		return false, "receipt not found yet", nil
	}, WaitReceiptPollInterval, deadline)
	if err != nil {
		if receiptErr != nil {
			return nil, receiptErr
		}
		return nil, err
	}

	return receipt, nil
}

func isTransientReceiptError(err error) bool {
	if errors.Is(err, ethereum.NotFound) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "request timed out") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "Client.Timeout exceeded")
}

// BuildSignedTx constructs and signs an EIP-1559 transaction.
// - If explicitNonce == nil, the account's pending nonce is used.
// - If gas == 0, gas is estimated and padded with a safety margin.
// - If to == nil, the tx is treated as contract creation using data as init code.
func BuildSignedTx(
	ctx context.Context,
	c *ethclient.Client,
	priv *ecdsa.PrivateKey,
	to *common.Address,
	value *big.Int,
	data []byte,
	gas uint64,
	explicitNonce *uint64,
) (*types.Transaction, error) {
	from := crypto.PubkeyToAddress(priv.PublicKey)

	chainID, err := c.ChainID(ctx)
	if err != nil {
		return nil, err
	}

	// Resolve nonce
	var nonce uint64
	if explicitNonce != nil {
		nonce = *explicitNonce
	} else {
		n, err := c.PendingNonceAt(ctx, from)
		if err != nil {
			return nil, err
		}
		nonce = n
	}

	// Resolve gas limit
	if gas == 0 {
		msg := ethereum.CallMsg{From: from, To: to, Value: value, Data: data}
		g, err := c.EstimateGas(ctx, msg)
		if err != nil || g == 0 {
			gas = defaultGasLimit
		} else {
			gas = g + g*gasPaddingPercent/100 + gasPaddingFixed
		}
	}

	// Tip cap with fallback
	tipCap, err := c.SuggestGasTipCap(ctx)
	if err != nil || tipCap == nil || tipCap.Sign() <= 0 {
		tipCap = big.NewInt(1_000_000_000) // 1 gwei fallback
	}

	// Base fee (tolerate missing header/basefee by treating as zero)
	hdr, err := c.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, err
	}
	baseFee := new(big.Int)
	if hdr != nil && hdr.BaseFee != nil {
		baseFee.Set(hdr.BaseFee)
	}

	val := big.NewInt(0)
	if value != nil {
		val = new(big.Int).Set(value)
	}

	// feeCap = 2*baseFee + tipCap (or tipCap if baseFee==0)
	feeCap := new(big.Int).Set(tipCap)
	if baseFee.Sign() > 0 {
		feeCap = new(big.Int).Add(new(big.Int).Mul(baseFee, big.NewInt(2)), tipCap)
	}

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        to,
		Value:     val,
		Gas:       gas,
		GasFeeCap: new(big.Int).Set(feeCap),
		GasTipCap: new(big.Int).Set(tipCap),
		Data:      append([]byte(nil), data...),
	})

	signer := types.LatestSignerForChainID(chainID)
	return types.SignTx(tx, signer, priv)
}

// SendTx sends a transaction using EIP-1559.
// - If to == nil, this performs contract creation using data as init code.
// - If gas == 0, gas is estimated and buffered; otherwise, the provided gas limit is used.
func SendTx(
	ctx context.Context,
	c *ethclient.Client,
	priv *ecdsa.PrivateKey,
	to *common.Address,
	value *big.Int,
	data []byte,
	gas uint64,
) (common.Hash, error) {
	signed, err := BuildSignedTx(ctx, c, priv, to, value, data, gas, nil)
	if err != nil {
		return common.Hash{}, err
	}
	if err := c.SendTransaction(ctx, signed); err != nil {
		return common.Hash{}, err
	}
	return signed.Hash(), nil
}
