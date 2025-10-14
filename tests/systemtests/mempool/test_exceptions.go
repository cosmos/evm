package mempool

import (
	"fmt"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/test-go/testify/require"
)

func TestTxRebroadcasting(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s TestSuite)
	}{
		{
			name: "ordering of pending txs %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					signer := s.AcquireAcc()
					defer s.ReleaseAcc(signer)

					tx1, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					tx2, err := s.SendTx(t, s.Node(1), signer.ID, 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					tx3, err := s.SendTx(t, s.Node(2), signer.ID, 2, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					// Skip tx4 with nonce 3

					tx5, err := s.SendTx(t, s.Node(3), signer.ID, 4, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					tx6, err := s.SendTx(t, s.Node(0), signer.ID, 5, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					// At AfterEachAction hook, we will check expected queued txs are not broadcasted.
					s.SetExpPendingTxs(tx1, tx2, tx3)
					s.SetExpQueuedTxs(tx5, tx6)
				},
				func(s TestSuite) {
					// Wait for 3 blocks.
					// It is because tx1, tx2, tx3 are sent to different nodes, tx3 needs maximum 3 blocks to be committed.
					// e.g. node3 is 1st proposer -> tx3 will tale 1 block to be committed.
					// e.g. node3 is 3rd proposer -> tx3 will take 3 blocks to be committed.
					s.AwaitNBlocks(t, 3)

					// current nonce is 3.
					// so, we should set nonce idx to 0.
					nonce3Idx := uint64(0)

					signer := s.AcquireAcc()
					defer s.ReleaseAcc(signer)

					tx4, err := s.SendTx(t, s.Node(2), signer.ID, nonce3Idx, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					// At AfterEachAction hook, we will check expected pending txs are broadcasted.
					s.SetExpPendingTxs(tx4)
					s.PromoteExpTxs(2)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description:    "EVM LegacyTx",
			TxType:         suite.TxTypeEVM,
			IsDynamicFeeTx: false,
		},
	}

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, to := range testOptions {
		s.SetOptions(to)
		for _, tc := range testCases {
			testName := fmt.Sprintf(tc.name, to.Description)
			t.Run(testName, func(t *testing.T) {
				s.BeforeEachCase(t)
				for _, action := range tc.actions {
					action(s)
					s.AfterEachAction(t)
				}
				s.AfterEachCase(t)
			})
		}
	}
}

func TestMinimumGasPricesZero(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s TestSuite)
	}{
		{
			name: "sequencial pending txs %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					signer := s.AcquireAcc()
					defer s.ReleaseAcc(signer)

					tx1, err := s.SendTx(t, s.Node(0), signer.ID, 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					tx2, err := s.SendTx(t, s.Node(1), signer.ID, 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					tx3, err := s.SendTx(t, s.Node(2), signer.ID, 2, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx1, tx2, tx3)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description:    "EVM LegacyTx",
			TxType:         suite.TxTypeEVM,
			IsDynamicFeeTx: false,
		},
		{
			Description:    "EVM DynamicFeeTx",
			TxType:         suite.TxTypeEVM,
			IsDynamicFeeTx: true,
		},
	}

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t, suite.MinimumGasPriceZeroArgs()...)

	for _, to := range testOptions {
		s.SetOptions(to)
		for _, tc := range testCases {
			testName := fmt.Sprintf(tc.name, to.Description)
			t.Run(testName, func(t *testing.T) {
				s.BeforeEachCase(t)
				for _, action := range tc.actions {
					action(s)
					s.AfterEachAction(t)
				}
				s.AfterEachCase(t)
			})
		}
	}
}
