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
		actions []func(s suite.TestSuite)
		bypass  bool
	}{
		{
			name: "single pending tx %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					lowFeeEVMTxHash, err := s.SendTx(t, s.GetNodeID(0), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					highGasEVMTxHash, err := s.SendTx(t, s.GetNodeID(1), "acc0", 0, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					s.SetExpQueuedTxs(highGasEVMTxHash, lowFeeEVMTxHash)
				},
			},
			bypass: true,
		},
		{
			name: "multiple pending txs %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					_, err := s.SendTx(t, s.GetNodeID(0), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendTx(t, s.GetNodeID(0), "acc0", 0, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					_, err = s.SendTx(t, s.GetNodeID(1), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					tx4, err := s.SendTx(t, s.GetNodeID(1), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					_, err = s.SendTx(t, s.GetNodeID(2), "acc0", 2, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					tx6, err := s.SendTx(t, s.GetNodeID(2), "acc0", 2, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx2, tx4, tx6)
				},
			},
			bypass: true,
		},
		{
			name: "single queued tx %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					_, err := s.SendTx(t, s.GetNodeID(0), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendTx(t, s.GetNodeID(0), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					s.SetExpQueuedTxs(tx2)
				},
				func(s suite.TestSuite) {
					txHash, err := s.SendTx(t, s.GetNodeID(1), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(txHash)
					s.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "multiple queued txs %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					_, err := s.SendTx(t, s.GetNodeID(0), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendTx(t, s.GetNodeID(0), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					_, err = s.SendTx(t, s.GetNodeID(1), "acc0", 2, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					tx4, err := s.SendTx(t, s.GetNodeID(1), "acc0", 2, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					_, err = s.SendTx(t, s.GetNodeID(2), "acc0", 3, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					tx6, err := s.SendTx(t, s.GetNodeID(2), "acc0", 3, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					s.SetExpQueuedTxs(tx2, tx4, tx6)
				},
				func(s suite.TestSuite) {
					tx, err := s.SendTx(t, s.GetNodeID(3), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx)
					s.PromoteExpTxs(3)
				},
			},
		},
	}

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, to := range s.DefaultTestOption() {
		for _, tc := range testCases {
			s.TestOption = to
			tc.name = fmt.Sprintf(tc.name, to.TestType)
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					return
				}

				s.BeforeEach(t)
				for _, action := range tc.actions {
					action(s)
					s.JustAfterEach(t)
				}
				s.AfterEach(t)
			})
		}
	}
}

func TestMixedTxsReplacement(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s suite.TestSuite)
		bypass  bool
	}{
		{
			name: "single pending tx (low prio evm tx first) %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					_, err := s.SendEthTx(t, s.GetNodeID(0), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendCosmosTx(t, s.GetNodeID(1), "acc0", 0, s.BaseFeeX2(), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx2)
				},
			},
			bypass: true,
		},
		{
			name: "single pending tx (high prio evm tx first) %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					tx1, err := s.SendEthTx(t, s.GetNodeID(0), "acc0", 0, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendCosmosTx(t, s.GetNodeID(1), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx1)
				},
			},
			bypass: true,
		},
		{
			name: "single pending tx (low prio cosmos tx first) %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					_, err := s.SendCosmosTx(t, s.GetNodeID(0), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendEthTx(t, s.GetNodeID(1), "acc0", 0, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx2)
				},
			},
			bypass: true,
		},
		{
			name: "single pending tx (high prio cosmos tx first) %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					tx1, err := s.SendCosmosTx(t, s.GetNodeID(0), "acc0", 0, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendEthTx(t, s.GetNodeID(1), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx1)
				},
			},
			bypass: true,
		},
		{
			name: "single queued tx (low prio evm tx first) %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					tx1, err := s.SendEthTx(t, s.GetNodeID(0), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendCosmosTx(t, s.GetNodeID(0), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					s.SetExpQueuedTxs(tx1)
				},
				func(s suite.TestSuite) {
					tx3, err := s.SendEthTx(t, s.GetNodeID(1), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx3)
					s.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "single queued tx (high prio evm tx first) %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					tx1, err := s.SendEthTx(t, s.GetNodeID(0), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")
					_, err = s.SendCosmosTx(t, s.GetNodeID(0), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					s.SetExpQueuedTxs(tx1)
				},
				func(s suite.TestSuite) {
					tx3, err := s.SendEthTx(t, s.GetNodeID(1), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx3)
					s.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "single queued tx (low prio cosmos tx first) %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					_, err := s.SendCosmosTx(t, s.GetNodeID(0), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendEthTx(t, s.GetNodeID(0), "acc0", 1, new(big.Int).Add(s.BaseFeeX2(), big.NewInt(100)), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					s.SetExpQueuedTxs(tx2)
				},
				func(s suite.TestSuite) {
					tx3, err := s.SendEthTx(t, s.GetNodeID(1), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx3)
					s.PromoteExpTxs(1)
				},
			},
		},
		{
			name: "single queued tx (high prio cosmos tx first) %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					_, err := s.SendCosmosTx(t, s.GetNodeID(0), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendEthTx(t, s.GetNodeID(0), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")

					// CosmosTx is not queued in local mempool
					s.SetExpQueuedTxs(tx2)
				},
				func(s suite.TestSuite) {
					tx3, err := s.SendEthTx(t, s.GetNodeID(1), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(tx3)
					s.PromoteExpTxs(1)
				},
			},
		},
	}

	testOptions := []suite.TestOption{
		{
			TestType:          "EVM LegacyTx & Cosmos LegacyTx",
			ApplyDynamicFeeTx: false,
			NodeEntries:       []string{"node0", "node0", "node0", "node0", "node0", "node0", "node0", "node0"},
		},
		{
			TestType:          "EVM DynamicTx & Cosmos LegacyTx",
			ApplyDynamicFeeTx: false,
			NodeEntries:       []string{"node0", "node0", "node0", "node0", "node0", "node0", "node0", "node0"},
		},
	}

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, to := range testOptions {
		for _, tc := range testCases {
			s.TestOption = to
			tc.name = fmt.Sprintf(tc.name, to.TestType)
			t.Run(tc.name, func(t *testing.T) {
				if tc.bypass {
					return
				}

				s.BeforeEach(t)
				for _, action := range tc.actions {
					action(s)
					s.JustAfterEach(t)
				}
				s.AfterEach(t)
			})
		}
	}
}
