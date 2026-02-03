package e2e

import (
	"math/big"
	"strings"
	"testing"

	"github.com/cosmos/evm/evmd/e2e/contracts"
	"github.com/cosmos/evm/evmd/e2e/testharness"
	"github.com/cosmos/evm/evmd/e2e/utils"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// TestEVMTransfer verifies EVM value transfer under EIP-1559 (type-2).
// Steps: send 1e18 wei A→B; assert receipt.status=1; balances reflect amount and gas.
func TestEVMTransfer(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	// Generate a recipient address (B)
	recvKey, err := crypto.GenerateKey()
	req.NoError(err)
	recipient := crypto.PubkeyToAddress(recvKey.PublicKey)
	recipientBech32, err := utils.AddressToBech32(recipient)
	req.NoError(err)

	// Value to transfer: 1e18 wei
	value := big.NewInt(1_000_000_000_000_000_000)

	// Pre-transfer balances
	preSender, err := utils.GetBalance(harness.Ctx, chain.EthClient, harness.SenderAddr)
	req.NoError(err)
	preRecipient, err := utils.GetBalance(harness.Ctx, chain.EthClient, recipient)
	req.NoError(err)

	// Send EIP-1559 dynamic fee transaction A -> B
	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &recipient, value, nil, utils.BasicTransferGas)
	req.NoError(err)

	// Wait for inclusion and fetch receipt
	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)
	req.NotZero(rec.GasUsed)

	// Post-transfer balances
	postSender, err := utils.GetBalance(harness.Ctx, chain.EthClient, harness.SenderAddr)
	req.NoError(err)
	postSenderBankBalance, err := utils.GetBankBalance(harness.Ctx, chain.GrpcClient, harness.SenderBech32, utils.TestDenom)
	req.NoError(err)
	postRecipient, err := utils.GetBalance(harness.Ctx, chain.EthClient, recipient)
	req.NoError(err)
	postRecipientBankBalance, err := utils.GetBankBalance(harness.Ctx, chain.GrpcClient, recipientBech32, utils.TestDenom)
	req.NoError(err)

	// Ethereum balance should be equal to bank balance
	req.Equal(postSender, postSenderBankBalance.Balance.Amount.BigInt())
	req.Equal(postRecipient, postRecipientBankBalance.Balance.Amount.BigInt())

	// Recipient delta must equal value exactly
	recipientDelta := new(big.Int).Sub(postRecipient, preRecipient)
	req.Equalf(0, recipientDelta.Cmp(value), "recipient delta %s != value %s", recipientDelta.String(), value.String())

	// Sender decrease should be >= value + gasUsed*effectiveGasPrice
	gasCost := new(big.Int).Mul(new(big.Int).SetUint64(rec.GasUsed), rec.EffectiveGasPrice)
	minDecrease := new(big.Int).Add(new(big.Int).Set(value), gasCost)
	senderDelta := new(big.Int).Sub(preSender, postSender)
	req.GreaterOrEqual(senderDelta.Cmp(minDecrease), 0, "sender decrease %s < expected minimum %s", senderDelta.String(), minDecrease.String())

	t.Logf("tx=%s gasUsed=%d effectiveGasPrice=%s", txHash.Hex(), rec.GasUsed, rec.EffectiveGasPrice.String())
}

// TestEVMNonceGap verifies that out-of-order EVM transactions (with nonce gaps)
// are queued and only become includable once gaps are filled.
// Flow:
//  1. Send tx0 at nonce N and wait for success
//  2. Send tx2 at nonce N+2 and assert it is NOT included before gap is filled
//  3. Send tx1 at nonce N+1, wait for success
//  4. Verify tx2 is eventually included after the gap is filled
func TestEVMNonceGap(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain
	ctx := harness.Ctx

	// Recipient: ephemeral EOA; use value=0 to avoid changing balances
	toKey, err := crypto.GenerateKey()
	req.NoError(err)
	to := crypto.PubkeyToAddress(toKey.PublicKey)

	// Determine starting nonce for the funded sender
	baseNonce, err := chain.EthClient.PendingNonceAt(ctx, harness.SenderAddr)
	req.NoError(err)

	// Helper to build a signed tx with explicit nonce using shared utility
	buildSignedTx := func(nonce uint64) (*types.Transaction, error) {
		n := nonce
		return utils.BuildSignedTx(ctx, chain.EthClient, harness.SenderKey, &to, big.NewInt(0), nil, 21000, &n)
	}

	// 1) Send tx0 (nonce N) and wait for inclusion
	signed0, err := buildSignedTx(baseNonce)
	req.NoError(err)
	req.NoError(chain.EthClient.SendTransaction(ctx, signed0))
	rec0, err := utils.WaitReceipt(ctx, chain.EthClient, signed0.Hash(), utils.StakingReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec0)
	req.Equal(uint64(1), rec0.Status)

	// 2) Send tx2 (nonce N+2) — should be queued due to nonce gap
	signed2, err := buildSignedTx(baseNonce + 2)
	req.NoError(err)
	req.NoError(chain.EthClient.SendTransaction(ctx, signed2))
	// Ensure it is NOT included before we fill the gap
	_, err = utils.WaitReceipt(ctx, chain.EthClient, signed2.Hash(), utils.FailureReceiptTimeout)
	req.Error(err)

	// 3) Send tx1 (nonce N+1) and wait for inclusion
	signed1, err := buildSignedTx(baseNonce + 1)
	req.NoError(err)
	req.NoError(chain.EthClient.SendTransaction(ctx, signed1))
	rec1, err := utils.WaitReceipt(ctx, chain.EthClient, signed1.Hash(), utils.StakingReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec1)
	req.Equal(uint64(1), rec1.Status)

	// 4) Now tx2 should become includable and get mined
	rec2, err := utils.WaitReceipt(ctx, chain.EthClient, signed2.Hash(), utils.StakingReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec2)
	req.Equal(uint64(1), rec2.Status)

	// Sanity: verify the mined transactions carry the intended nonces
	mined0, _, err := chain.EthClient.TransactionByHash(ctx, signed0.Hash())
	req.NoError(err)
	mined1, _, err := chain.EthClient.TransactionByHash(ctx, signed1.Hash())
	req.NoError(err)
	mined2, _, err := chain.EthClient.TransactionByHash(ctx, signed2.Hash())
	req.NoError(err)

	req.Equal(baseNonce, mined0.Nonce())
	req.Equal(baseNonce+1, mined1.Nonce())
	req.Equal(baseNonce+2, mined2.Nonce())
}

// TestGasRevertSemantics covers:
// - Out-of-gas on a contract call yields status=0 and no state change
// - Revert-with-reason is surfaced via eth_call and trace for a mined tx
func TestGasRevertSemantics(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	// OOG: no state change
	// Deploy Counter
	parsedCounter, err := abi.JSON(strings.NewReader(contracts.CounterABIJSON()))
	req.NoError(err)
	creation := common.FromHex(contracts.CounterBinHex())

	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, nil, big.NewInt(0), creation, 0)
	req.NoError(err)

	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)
	counter := rec.ContractAddress

	// Pre-state: slot0 should be zero
	slot0 := common.Hash{}
	preRaw, err := chain.EthClient.StorageAt(harness.Ctx, counter, slot0, nil)
	req.NoError(err)
	req.Len(preRaw, 32)
	req.Zero(new(big.Int).SetBytes(preRaw).Cmp(big.NewInt(0)))

	// Prepare set(42) call data
	setData, err := parsedCounter.Pack("set", big.NewInt(42))
	req.NoError(err)

	// Estimate and undercut gas to force OOG while still clearing intrinsic gas
	from := harness.SenderAddr
	est, err := chain.EthClient.EstimateGas(harness.Ctx, ethereum.CallMsg{From: from, To: &counter, Data: setData})
	req.NoError(err)
	req.NotZero(est, "eth_estimateGas returned 0 for set(42) — unexpected; investigate RPC/node behavior")

	const outOfGasFraction = 0.66 // Use 66% of estimated gas to force OOG
	gas := uint64(float64(est) * outOfGasFraction)
	if gas < 30000 {
		gas = 30000 // ensure above intrinsic
	}

	// Build and send a dynamic-fee tx with limited gas
	txHashLimited, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &counter, big.NewInt(0), setData, gas)
	req.NoError(err)

	rec2, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHashLimited, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec2)
	req.Equal(uint64(0), rec2.Status)
	t.Logf("OOG tx=%s gasUsed=%d gasLimit=%d", txHashLimited.Hex(), rec2.GasUsed, gas)

	// Post-state: still zero
	postRaw, err := chain.EthClient.StorageAt(harness.Ctx, counter, slot0, nil)
	req.NoError(err)
	req.Len(postRaw, 32)
	req.Zero(new(big.Int).SetBytes(postRaw).Cmp(big.NewInt(0)))

	// Revert with reason visibility
	parsedRev, err := abi.JSON(strings.NewReader(contracts.ReverterABIJSON()))
	req.NoError(err)
	creation2 := common.FromHex(contracts.ReverterBinHex())

	txHash3, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, nil, big.NewInt(0), creation2, 0)
	req.NoError(err)
	rec3, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash3, utils.ReceiptTimeout)
	req.NoError(err)
	req.Equal(uint64(1), rec3.Status)
	revAddr := rec3.ContractAddress

	// eth_call should return revert data encoding the reason; decode via rpc error.data
	reason := "NOPE"
	callData, err := parsedRev.Pack("revertWithReason", reason)
	req.NoError(err)

	// Send a tx that reverts with reason
	txHashRevert, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &revAddr, big.NewInt(0), callData, 100000)
	req.NoError(err)

	rec4, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHashRevert, utils.ReceiptTimeout)
	req.NoError(err)
	req.Equal(uint64(0), rec4.Status)

	// Assert revert reason strictly by decoding revert data via eth_call at the block of inclusion
	got, err := utils.GetRevertReasonViaEthCall(harness.Ctx, chain.RPC, revAddr, callData, rec4.BlockNumber)
	req.NoError(err)
	req.Equal(reason, got)
}
