//go:build system_test

package accountabstraction

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/require"

	suite "github.com/cosmos/evm/tests/systemtests/suite"
)

// RunEIP7702SameBlock tests an EIP-7702 SetCode tx (relayer-sent, user as
// authority at nonce N) and a user-signed Cosmos bank send at sequence N
// landing in the same block. Both block-internal orderings are exercised:
//
//   - SetCode-first: delegation installed; bank included-as-failed.
//   - Bank-first: bank commits; SetCode's auth silently skipped; no delegation.
func RunEIP7702SameBlock(t *testing.T, base *suite.BaseTestSuite) {
	gomega.RegisterTestingT(t)

	s := NewTestSuite(base)
	s.SetupTestWithTimeoutCommit(t, 8*time.Second)

	relayer := s.BaseTestSuite.Acc(0)
	user := s.BaseTestSuite.Acc(1)
	s.SetPrimaryAccount(relayer)
	relayerID := relayer.ID
	userID := user.ID

	fee, err := s.GetLatestBaseFee("node0")
	require.NoError(t, err)
	s.SetBaseFee(fee)

	cleanupDelegation := func(t *testing.T) {
		ctx := context.Background()
		code, err := s.EthClient.Clients["node0"].CodeAt(ctx, s.GetAddr(userID), nil)
		require.NoError(t, err)
		if len(code) == 0 {
			return
		}
		auth := createSetCodeAuthorization(s.GetChainID(), s.GetNonce(userID)+1, common.Address{})
		signed, err := signSetCodeAuthorization(s.GetPrivKey(userID), auth)
		require.NoError(t, err)
		txHash, err := s.SendSetCodeTx(userID, signed)
		require.NoError(t, err, "cleanup SetCode failed")
		s.WaitForCommit(txHash)
	}

	// SetCode-first: bias the SetCode tx's tip so the proposer iterator
	// emits it before the bank tx inside the same block.
	t.Run("SetCode-first execution: delegation installed, bank included-as-failed", func(t *testing.T) {
		t.Cleanup(func() { cleanupDelegation(t) })

		counterAddr := s.GetCounterAddr()
		baseFee := s.GasPriceMultiplier(1)
		setCodeTip := uint256.MustFromBig(new(big.Int).Mul(baseFee, big.NewInt(100)))
		setCodeFeeCap := new(uint256.Int).Mul(setCodeTip, uint256.NewInt(2))
		bankPrice := new(big.Int).Mul(baseFee, big.NewInt(2))

		receipt, bankRes := submitSameBlock(t, s, relayerID, userID, counterAddr, setCodeTip, setCodeFeeCap, bankPrice)

		require.Equal(t, uint64(1), receipt.Status, "outer SetCode tx must be accepted")

		ctx := context.Background()
		code, err := s.EthClient.Clients["node0"].CodeAt(ctx, s.GetAddr(userID), nil)
		require.NoError(t, err)
		require.Equal(t, 23, len(code), "delegation must be installed; got code length %d", len(code))
		resolved, ok := ethtypes.ParseDelegation(code)
		require.True(t, ok)
		require.Equal(t, counterAddr, resolved)

		require.NotZero(t, bankRes.code,
			"bank must be included-as-failed; got code=%d log=%q", bankRes.code, bankRes.log)
		require.True(t,
			strings.Contains(strings.ToLower(bankRes.log), "sequence") ||
				strings.Contains(strings.ToLower(bankRes.log), "nonce"),
			"expected sequence/nonce error in bank log, got: %s", bankRes.log)

		t.Logf("SetCode-first: block=%d delegation=installed bankCode=%d log=%q",
			receipt.BlockNumber.Uint64(), bankRes.code, bankRes.log)
	})

	// Bank-first: bias the bank tx's tip so the proposer iterator emits
	// it before the SetCode tx inside the same block.
	t.Run("Bank-first execution: bank commits, auth silently skipped, no delegation", func(t *testing.T) {
		t.Cleanup(func() { cleanupDelegation(t) })

		counterAddr := s.GetCounterAddr()
		baseFee := s.GasPriceMultiplier(1)
		setCodeTip := uint256.MustFromBig(new(big.Int).Mul(baseFee, big.NewInt(1)))
		// 10× base fee absorbs any drift between GetLatestBaseFee and inclusion
		// so the SetCode tx still passes its fee cap check when the iterator
		// emits it after the bank tx.
		setCodeFeeCap := uint256.MustFromBig(new(big.Int).Mul(baseFee, big.NewInt(10)))
		bankPrice := new(big.Int).Mul(baseFee, big.NewInt(100))

		receipt, bankRes := submitSameBlock(t, s, relayerID, userID, counterAddr, setCodeTip, setCodeFeeCap, bankPrice)

		require.Equal(t, uint64(1), receipt.Status, "outer SetCode tx must be accepted")

		ctx := context.Background()
		code, err := s.EthClient.Clients["node0"].CodeAt(ctx, s.GetAddr(userID), nil)
		require.NoError(t, err)
		require.Equal(t, 0, len(code), "no delegation should be installed when auth was silently skipped")

		require.Zero(t, bankRes.code,
			"bank must commit normally; got code=%d log=%q", bankRes.code, bankRes.log)

		t.Logf("Bank-first: block=%d delegation=absent bankCode=%d", receipt.BlockNumber.Uint64(), bankRes.code)
	})
}

type bankCommitResult struct {
	height int64
	code   uint32
	log    string
}

// submitSameBlock submits a relayer-sent SetCode tx (user as authority at the
// user's current nonce N), waits for it to reach every validator's legacypool
// pending list, then submits a user-signed bank tx at the same sequence N.
// Asserts both land in the same block.
func submitSameBlock(
	t *testing.T,
	s *TestSuite,
	relayerID, userID string,
	counterAddr common.Address,
	setCodeTip, setCodeFeeCap *uint256.Int,
	bankPrice *big.Int,
) (*ethtypes.Receipt, bankCommitResult) {
	t.Helper()

	contested := s.GetNonce(userID)

	auth := createSetCodeAuthorization(s.GetChainID(), contested, counterAddr)
	signedAuth, err := signSetCodeAuthorization(s.GetPrivKey(userID), auth)
	require.NoError(t, err)

	s.AwaitNBlocks(t, 1)

	setCodeHash, err := s.SendSetCodeTxWithFees(relayerID, setCodeTip, setCodeFeeCap, signedAuth)
	require.NoError(t, err, "failed to broadcast SetCode tx")

	// Wait for SetCode to be in pending on every validator before submitting bank.
	require.Eventually(t, func() bool {
		for i := 0; i < 4; i++ {
			pending, _, err := s.TxPoolContent(s.Node(i), suite.TxTypeEVM, 5*time.Second)
			if err != nil {
				return false
			}
			found := false
			for _, h := range pending {
				if h == setCodeHash.Hex() {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	}, 30*time.Second, 500*time.Millisecond,
		"SetCode tx failed to gossip into every validator's legacypool pending")

	// Bank at the same sequence as the auth.
	userCosmosAcc := s.CosmosAccount(userID)
	toAcc := s.CosmosAccount(s.AccID(2))
	bankResp, err := s.CosmosClient.BankSend(
		"node0",
		userCosmosAcc,
		userCosmosAcc.AccAddress,
		toAcc.AccAddress,
		sdkmath.NewInt(1000),
		contested,
		bankPrice,
	)
	require.NoError(t, err, "BankSend RPC call should not error")
	require.NotNil(t, bankResp)
	require.Zero(t, bankResp.Code,
		"bank CheckTx must accept; got code=%d raw_log=%q", bankResp.Code, bankResp.RawLog)

	receipt, err := s.EthClient.WaitForCommit("node0", setCodeHash.Hex(), 60*time.Second)
	require.NoError(t, err, "SetCode never committed")
	bankRes, err := s.CosmosClient.WaitForCommit("node0", bankResp.TxHash, 60*time.Second)
	require.NoError(t, err, "bank never committed")

	require.Equal(t, int64(receipt.BlockNumber.Uint64()), bankRes.Height,
		"SetCode and bank must land in the same block; got SetCode=%d bank=%d",
		receipt.BlockNumber.Uint64(), bankRes.Height)

	return receipt, bankCommitResult{
		height: bankRes.Height,
		code:   bankRes.TxResult.Code,
		log:    bankRes.TxResult.Log,
	}
}
