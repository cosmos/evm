//go:build system_test

package eip712

import (
	"math/big"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/stretchr/testify/require"
)

// RunEIP712BankSend tests that a bank send transaction can be signed and broadcast using EIP-712.
func RunEIP712BankSend(t *testing.T, base *suite.BaseTestSuite) {
	s := NewTestSuite(base)
	s.SetupTest(t)

	// Get initial nonce for acc0
	acc0 := s.Acc(0)

	gasPrice := big.NewInt(1000000000000)

	// Send a bank send transaction from acc0 to acc1 using EIP-712 signing
	from := acc0.Cosmos.AccAddress
	to := s.Acc(1).Cosmos.AccAddress
	amount := big.NewInt(1000000)
	curNonceIdx := uint64(0)

	txHash, err := s.SendBankSendWithEIP712(
		t,
		s.Node(0),
		acc0.ID,
		to,
		amount,
		curNonceIdx,
		gasPrice,
	)
	require.NoError(t, err, "Failed to send bank send with EIP-712")
	require.NotEmpty(t, txHash, "Transaction hash should not be empty")

	// Wait for the transaction to be committed
	err = s.WaitForCommit(s.Node(0), txHash)
	require.NoError(t, err, "Transaction should be committed successfully")

	t.Logf("Successfully sent bank send transaction with EIP-712 signing: %s", txHash)
	t.Logf("From: %s", from.String())
	t.Logf("To: %s", to.String())
	t.Logf("Amount: %s", amount.String())
}

// RunEIP712BankSendWithBalanceCheck tests that a bank send transaction using EIP-712
// correctly updates the balances of the sender and receiver.
func RunEIP712BankSendWithBalanceCheck(t *testing.T, base *suite.BaseTestSuite) {
	s := NewTestSuite(base)
	s.SetupTest(t)

	signer := s.Acc(0)

	denom := "atest"

	// Get accounts
	fromAddr := signer.Cosmos.AccAddress
	toAddr := s.Acc(1).Cosmos.AccAddress

	// Get initial balances
	initialFromBalance, err := s.GetBalance(t, s.Node(0), fromAddr, denom)
	require.NoError(t, err, "Failed to get initial balance for sender")
	t.Logf("Initial sender balance: %s", initialFromBalance.String())

	initialToBalance, err := s.GetBalance(t, s.Node(0), toAddr, denom)
	require.NoError(t, err, "Failed to get initial balance for receiver")
	t.Logf("Initial receiver balance: %s", initialToBalance.String())

	// Send a bank send transaction using EIP-712
	curNonceIdx := uint64(0)
	gasPrice := big.NewInt(1000000000000)
	amount := big.NewInt(5000000)

	txHash, err := s.SendBankSendWithEIP712(
		t,
		s.Node(0),
		signer.ID,
		toAddr,
		amount,
		curNonceIdx,
		gasPrice,
	)
	require.NoError(t, err, "Failed to send bank send with EIP-712")
	require.NotEmpty(t, txHash, "Transaction hash should not be empty")

	// Wait for the transaction to be committed
	err = s.WaitForCommit(s.Node(0), txHash)
	require.NoError(t, err, "Transaction should be committed successfully")

	// Wait for one more block to ensure balance updates are finalized
	s.AwaitNBlocks(t, 1)

	// Get final balances
	finalFromBalance, err := s.GetBalance(t, s.Node(0), fromAddr, denom)
	require.NoError(t, err, "Failed to get final balance for sender")
	t.Logf("Final sender balance: %s", finalFromBalance.String())

	finalToBalance, err := s.GetBalance(t, s.Node(0), toAddr, denom)
	require.NoError(t, err, "Failed to get final balance for receiver")
	t.Logf("Final receiver balance: %s", finalToBalance.String())

	// Verify receiver balance increased by the amount sent
	expectedToBalance := new(big.Int).Add(initialToBalance, amount)
	require.Equal(t, expectedToBalance, finalToBalance,
		"Receiver balance should increase by the amount sent")

	// Verify sender balance decreased (by amount + fees)
	// The sender's balance should be less than initial - amount
	maxExpectedFromBalance := new(big.Int).Sub(initialFromBalance, amount)
	require.True(t, finalFromBalance.Cmp(maxExpectedFromBalance) < 0,
		"Sender balance should decrease by at least the amount sent (plus fees)")

	t.Logf("Transaction hash: %s", txHash)
	t.Logf("Amount transferred: %s", amount.String())
	t.Logf("Sender balance change: %s", new(big.Int).Sub(initialFromBalance, finalFromBalance).String())
	t.Logf("Receiver balance change: %s", new(big.Int).Sub(finalToBalance, initialToBalance).String())
}

// RunEIP712MultipleBankSends tests that multiple bank send transactions can be sent
// sequentially using EIP-712 signing with correct nonce management.
func RunEIP712MultipleBankSends(t *testing.T, base *suite.BaseTestSuite) {
	s := NewTestSuite(base)
	s.SetupTest(t)

	signer := s.Acc(0)

	denom := "atest"
	toAddr := s.Acc(1).Cosmos.AccAddress

	// Get initial balance
	initialBalance, err := s.GetBalance(t, s.Node(0), toAddr, denom)
	require.NoError(t, err, "Failed to get initial balance")
	t.Logf("Initial receiver balance: %s", initialBalance.String())

	curNonceIdx := uint64(0)
	gasPrice := big.NewInt(1000000000000)
	amount := big.NewInt(1000000)
	numTxs := 3

	totalAmount := new(big.Int).Mul(amount, big.NewInt(int64(numTxs)))

	// Send multiple transactions with sequential nonces
	for i := 0; i < numTxs; i++ {
		txHash, err := s.SendBankSendWithEIP712(
			t,
			s.Node(0),
			signer.ID,
			toAddr,
			amount,
			curNonceIdx,
			gasPrice,
		)
		require.NoError(t, err, "Failed to send transaction %d", i)
		require.NotEmpty(t, txHash, "Transaction hash should not be empty for tx %d", i)

		t.Logf("Sent transaction %d with hash: %s", i, txHash)

		// Wait for the transaction to be committed
		err = s.WaitForCommit(s.Node(0), txHash)
		require.NoError(t, err, "Transaction %d should be committed successfully", i)
	}

	// Wait for one more block to ensure all balance updates are finalized
	s.AwaitNBlocks(t, 1)

	// Get final balance
	finalBalance, err := s.GetBalance(t, s.Node(0), toAddr, denom)
	require.NoError(t, err, "Failed to get final balance")
	t.Logf("Final receiver balance: %s", finalBalance.String())

	// Verify the balance increased by the total amount sent
	expectedBalance := new(big.Int).Add(initialBalance, totalAmount)
	require.Equal(t, expectedBalance, finalBalance,
		"Receiver balance should increase by the total amount sent across all transactions")

	t.Logf("Successfully sent %d transactions", numTxs)
	t.Logf("Total amount transferred: %s", totalAmount.String())
	t.Logf("Balance change: %s", new(big.Int).Sub(finalBalance, initialBalance).String())
}
