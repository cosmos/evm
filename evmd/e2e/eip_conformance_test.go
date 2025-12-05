package e2e

import (
	"context"
	"math/big"
	"testing"

	"github.com/cosmos/evm/evmd/e2e/testharness"
	"github.com/cosmos/evm/evmd/e2e/utils"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// TestEIP1559FeeSemantics validates EIP-1559 behavior:
// - A tx with maxFeePerGas below baseFee is rejected or not included.
// - A valid tx is mined with effectiveGasPrice >= block baseFee.
func TestEIP1559FeeSemantics(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	// Preconditions: baseFee present and > 0
	hdr, err := chain.EthClient.HeaderByNumber(harness.Ctx, nil)
	req.NoError(err)
	req.NotNil(hdr)
	if hdr.BaseFee == nil || hdr.BaseFee.Sign() == 0 {
		t.Skip("feemarket disabled or baseFee is zero")
	}

	// Under-baseFee tx: reject or not included
	lowKey, err := crypto.GenerateKey()
	req.NoError(err)
	lowAddr := crypto.PubkeyToAddress(lowKey.PublicKey)

	// Fund the ephemeral sender
	fundAmt := big.NewInt(1_000_000_000_000_000) // 1e15 wei
	txHashFund, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &lowAddr, fundAmt, nil, 21000)
	req.NoError(err)
	recFund, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHashFund, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(recFund)
	req.Equal(uint64(1), recFund.Status)

	chainID, err := chain.EthClient.ChainID(harness.Ctx)
	req.NoError(err)
	nonce, err := chain.EthClient.PendingNonceAt(harness.Ctx, lowAddr)
	req.NoError(err)

	to := harness.SenderAddr
	gas := uint64(21000)
	feeCap := big.NewInt(1) // deliberately far below baseFee
	tipCap := big.NewInt(1)

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &to,
		Value:     big.NewInt(0),
		Gas:       gas,
		GasFeeCap: feeCap,
		GasTipCap: tipCap,
	})
	signer := types.LatestSignerForChainID(chainID)
	signed, err := types.SignTx(tx, signer, lowKey)
	req.NoError(err)

	sendErr := chain.EthClient.SendTransaction(harness.Ctx, signed)
	if sendErr == nil {
		// Not immediately rejected: assert it is not included quickly
		_, rerr := utils.WaitReceipt(harness.Ctx, chain.EthClient, signed.Hash(), utils.ShortReceiptTimeout)
		req.Error(rerr, "underpriced tx unexpectedly got mined")
		req.ErrorIs(rerr, context.DeadlineExceeded, "underpriced tx should not be included")
	} else {
		t.Logf("low-fee tx rejected by RPC as expected: %v", sendErr)
	}

	// Valid tx: status=1 and effectiveGasPrice >= baseFee
	recvKey, err := crypto.GenerateKey()
	req.NoError(err)
	recipient := crypto.PubkeyToAddress(recvKey.PublicKey)
	value := big.NewInt(1) // 1 wei

	txHashGood, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &recipient, value, nil, 21000)
	req.NoError(err)

	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHashGood, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)

	hdr2, err := chain.EthClient.HeaderByNumber(harness.Ctx, rec.BlockNumber)
	req.NoError(err)
	req.NotNil(hdr2)
	req.NotNil(hdr2.BaseFee)
	req.GreaterOrEqual(rec.EffectiveGasPrice.Cmp(hdr2.BaseFee), 0,
		"effectiveGasPrice %s < baseFee %s", rec.EffectiveGasPrice.String(), hdr2.BaseFee.String())

	t.Logf("valid tx=%s baseFee=%s effectiveGasPrice=%s", txHashGood.Hex(), hdr2.BaseFee.String(), rec.EffectiveGasPrice.String())
}
