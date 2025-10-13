package mempool

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/test-go/testify/require"
)

func TestTxsReplacement(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s TestSuite)
	}{
		{
			name: "single pending tx submitted to same nodes %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx2)
				},
			},
		},
		{
			name: "multiple pending txs submitted to same nodes %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					_, err = s.SendTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					tx4, err := s.SendTx(t, s.Node(1), "acc0", 1, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					_, err = s.SendTx(t, s.Node(0), "acc0", 2, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					tx6, err := s.SendTx(t, s.Node(1), "acc0", 2, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx2, tx4, tx6)
				},
			},
		},
		{
			name: "single queued tx %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					s.SetExpQueuedTxs(tx2)
				},
				func(s TestSuite) {
					txHash, err := s.SendTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(txHash)
					s.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "multiple queued txs %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					_, err = s.SendTx(t, s.Node(1), "acc0", 2, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					tx4, err := s.SendTx(t, s.Node(1), "acc0", 2, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					_, err = s.SendTx(t, s.Node(2), "acc0", 3, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					tx6, err := s.SendTx(t, s.Node(2), "acc0", 3, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					s.SetExpQueuedTxs(tx2, tx4, tx6)
				},
				func(s TestSuite) {
					tx, err := s.SendTx(t, s.Node(3), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx)
					s.PromoteExpTxs(3)
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

func TestTxsReplacementWithCosmosTx(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s TestSuite)
	}{
		{
			name: "single pending tx submitted to same nodes %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					// NOTE: Currently EVMD cannot handle tx reordering correctly when cosmos tx is used.
					// It is because of CheckTxHandler cannot handle errors from SigVerificationDecorator properly.
					// After modifying CheckTxHandler, we can also modify this test case
					// : high prio cosmos tx should replace low prio evm tx.
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					//_, err = s.SendTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					//require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx1)
				},
			},
		},
		{
			name: "multiple pending txs submitted to same nodes %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					// NOTE: Currently EVMD cannot handle tx reordering correctly when cosmos tx is used.
					// It is because of CheckTxHandler cannot handle errors from SigVerificationDecorator properly.
					// After modifying CheckTxHandler, we can also modify this test case
					// : high prio cosmos tx should replace low prio evm tx.
					tx1, err := s.SendTx(t, s.Node(0), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					//_, err = s.SendTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					//require.NoError(t, err, "failed to send tx")

					tx3, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					//_, err = s.SendTx(t, s.Node(1), "acc0", 1, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					//require.NoError(t, err, "failed to send tx")

					tx5, err := s.SendTx(t, s.Node(0), "acc0", 2, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					//_, err = s.SendTx(t, s.Node(1), "acc0", 2, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					//require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx1, tx3, tx5)
				},
			},
		},
		{
			name: "single queued tx %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					// Cosmos txs are not queued in local mempool
					_, err := s.SendTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")
				},
				func(s TestSuite) {
					txHash, err := s.SendTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(txHash)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description: "Cosmos LegacyTx",
			TxType:      suite.TxTypeCosmos,
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

func TestMixedTxsReplacementEVMAndCosmos(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s TestSuite)
	}{
		{
			name: "single pending tx (low prio evm tx first) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendEthTx(t, s.Node(0), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), s.GetTxGasPrice(s.BaseFee()))
					require.NoError(t, err, "failed to send tx")

					baseFeeX20 := new(big.Int).Mul(s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1000000000000000000))
					_, err = s.SendCosmosTx(t, s.Node(1), "acc0", 0, baseFeeX20, nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx1)
				},
			},
		},
		{
			name: "single pending tx (high prio evm tx first) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendEthTx(t, s.Node(0), "acc0", 0, s.GetTxGasPrice(s.BaseFeeX2()), s.GetTxGasPrice(s.BaseFeeX2()))
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendCosmosTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx1)
				},
			},
		},
		{
			name: "single pending tx (low prio cosmos tx first) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendCosmosTx(t, s.Node(0), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendEthTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFeeX2()), s.GetTxGasPrice(s.BaseFeeX2()))
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx2)
				},
			},
		},
		{
			name: "single pending tx (high prio cosmos tx first) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					baseFeeX20 := new(big.Int).Mul(s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(10))
					tx1, err := s.SendCosmosTx(t, s.Node(0), "acc0", 0, baseFeeX20, nil)
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendEthTx(t, s.Node(0), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx1)
				},
			},
		},
		{
			name: "single queued tx (low prio evm tx first) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendEthTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendCosmosTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					s.SetExpQueuedTxs(tx1)
				},
				func(s TestSuite) {
					tx3, err := s.SendEthTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx3)
					s.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "single queued tx (high prio evm tx first) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendEthTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendCosmosTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					s.SetExpQueuedTxs(tx1)
				},
				func(s TestSuite) {
					tx3, err := s.SendEthTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx3)
					s.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "single queued tx (low prio cosmos tx first) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendCosmosTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendEthTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFeeX2()), s.GetTxGasPrice(s.BaseFeeX2()))
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					s.SetExpQueuedTxs(tx2)
				},
				func(s TestSuite) {
					tx3, err := s.SendEthTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx3)
					s.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "single queued tx (high prio cosmos tx first) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendCosmosTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendEthTx(t, s.Node(0), "acc0", 1, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					s.SetExpQueuedTxs(tx2)
				},
				func(s TestSuite) {
					tx3, err := s.SendEthTx(t, s.Node(1), "acc0", 0, s.GetTxGasPrice(s.BaseFee()), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx3)
					s.PromoteExpTxs(1)
				},
			},
		},
	}

	testOptions := []*suite.TestOptions{
		{
			Description: "EVM LegacyTx & Cosmos LegacyTx",
		},
		{
			Description:    "EVM DynamicTx & Cosmos LegacyTx",
			IsDynamicFeeTx: true,
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

func TestMixedTxsReplacementLegacyAndDynamicFee(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s TestSuite)
	}{
		{
			name: "dynamic fee tx should not replace legacy tx",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendEthLegacyTx(t, s.Node(0), s.Acc(0), 1, s.GetTxGasPrice(s.BaseFee()))
					require.NoError(t, err, "failed to send eth legacy tx")

					_, err = s.SendEthDynamicFeeTx(t, s.Node(0), s.Acc(0), 1, s.GetTxGasPrice(s.BaseFeeX2()), big.NewInt(1))
					require.Error(t, err)
					require.Contains(t, err.Error(), "replacement transaction underpriced")

					s.SetExpQueuedTxs(tx1)
				},
				func(s TestSuite) {
					txHash, err := s.SendEthLegacyTx(t, s.Node(0), s.Acc(0), 0, s.GetTxGasPrice(s.BaseFee()))
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(txHash)
					s.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "dynamic fee tx should replace legacy tx",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendEthLegacyTx(t, s.Node(0), s.Acc(0), 1, s.GetTxGasPrice(s.BaseFee()))
					require.NoError(t, err, "failed to send eth legacy tx")

					tx2, err := s.SendEthDynamicFeeTx(t, s.Node(0), s.Acc(0), 1,
						s.GetTxGasPrice(s.BaseFeeX2()),
						s.GetTxGasPrice(s.BaseFeeX2()),
					)
					require.NoError(t, err)

					s.SetExpQueuedTxs(tx2)
				},
				func(s TestSuite) {
					txHash, err := s.SendEthLegacyTx(t, s.Node(0), s.Acc(0), 0, s.GetTxGasPrice(s.BaseFee()))
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(txHash)
					s.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "legacy should never replace dynamic fee tx",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendEthDynamicFeeTx(t, s.Node(0), s.Acc(0), 1, s.GetTxGasPrice(s.BaseFeeX2()),
						new(big.Int).Sub(s.GetTxGasPrice(s.BaseFee()), big.NewInt(1)))
					require.NoError(t, err)

					_, err = s.SendEthLegacyTx(t, s.Node(0), s.Acc(0), 1, s.GetTxGasPrice(s.BaseFee()))
					require.Error(t, err, "failed to send eth legacy tx")
					require.Contains(t, err.Error(), "replacement transaction underpriced")

					// Legacy tx cannot replace dynamic fee tx.
					s.SetExpQueuedTxs(tx1)
				},
				func(s TestSuite) {
					txHash, err := s.SendEthLegacyTx(t, s.Node(0), s.Acc(0), 0, s.GetTxGasPrice(s.BaseFee()))
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(txHash)
					s.PromoteExpTxs(1)
				},
			},
		},
	}

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			s.BeforeEachCase(t)
			for _, action := range tc.actions {
				action(s)
				s.AfterEachAction(t)
			}
			s.AfterEachCase(t)
		})
	}
}
