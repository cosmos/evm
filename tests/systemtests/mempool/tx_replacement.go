package mempool

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/stretchr/testify/require"
)

func TestTxsReplacement(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s TestSuite)
		bypass  bool
	}{
		{
			name: "single pending tx %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					lowFeeEVMTxHash, err := s.SendTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err)

					highGasEVMTxHash, err := s.SendTx(s.GetNode(), "acc0", 0, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err)

					if s.OnlyEthTxs() {
						s.SetExpQueuedTxs(highGasEVMTxHash, lowFeeEVMTxHash)
					}
				},
			},
			bypass: true,
		},
		{
			name: "multiple pending txs %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err)

					tx2, err := s.SendTx(s.GetNode(), "acc0", 0, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err)

					_, err = s.SendTx(s.GetNode(), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err)

					tx4, err := s.SendTx(s.GetNode(), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err)

					_, err = s.SendTx(s.GetNode(), "acc0", 2, s.BaseFee(), nil)
					require.NoError(t, err)

					tx6, err := s.SendTx(s.GetNode(), "acc0", 2, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err)

					s.SetExpPendingTxs(tx2, tx4, tx6)
				},
			},
			bypass: true,
		},
		{
			name: "single queued tx %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendTx(s.GetNode(), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err)

					tx2, err := s.SendTx(s.GetNode(), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err)

					if s.OnlyEthTxs() {
						s.SetExpQueuedTxs(tx2)
					}
				},
				func(s TestSuite) {
					txHash, err := s.SendTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err)

					s.SetExpPendingTxs(txHash)
					if s.OnlyEthTxs() {
						s.PromoteExpTxs(1)
					}
				},
			},
		},
		{
			name: "multiple queued txs %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendTx(s.GetNode(), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err)

					tx2, err := s.SendTx(s.GetNode(), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err)

					_, err = s.SendTx(s.GetNode(), "acc0", 2, s.BaseFee(), nil)
					require.NoError(t, err)

					tx4, err := s.SendTx(s.GetNode(), "acc0", 2, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err)

					_, err = s.SendTx(s.GetNode(), "acc0", 3, s.BaseFee(), nil)
					require.NoError(t, err)

					tx6, err := s.SendTx(s.GetNode(), "acc0", 3, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err)

					if s.OnlyEthTxs() {
						s.SetExpQueuedTxs(tx2, tx4, tx6)
					}
				},
				func(s TestSuite) {
					txHash, err := s.SendTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err)

					s.SetExpPendingTxs(txHash)
					if s.OnlyEthTxs() {
						s.PromoteExpTxs(2)
					}
				},
			},
		},
	}

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, to := range s.DefaultTestOption() {
		for _, tc := range testCases {
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
		actions []func(s TestSuite)
		bypass  bool
	}{
		{
			name: "single pending tx (low prio evm tx first) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendEthTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err)

					_, err = s.SendCosmosTx(s.GetNode(), "acc0", 0, s.BaseFeeX2(), nil)
					require.NoError(t, err)

					s.SetExpQueuedTxs(tx1)
				},
			},
		},
		{
			name: "single pending tx (high prio evm tx first) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendEthTx(s.GetNode(), "acc0", 0, s.BaseFeeX2(), nil)
					require.NoError(t, err)

					_, err = s.SendCosmosTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err)

					s.SetExpQueuedTxs(tx1)
				},
			},
		},
		{
			name: "single pending tx (low prio cosmos tx) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					_, err := s.SendCosmosTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err)

					tx2, err := s.SendEthTx(s.GetNode(), "acc0", 0, s.BaseFeeX2(), nil)
					require.NoError(t, err)

					s.SetExpQueuedTxs(tx2)
				},
			},
		},
		{
			name: "single pending tx (high prio cosmos tx) %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					tx1, err := s.SendCosmosTx(s.GetNode(), "acc0", 0, s.BaseFeeX2(), nil)
					require.NoError(t, err)

					_, err = s.SendEthTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err)

					s.SetExpQueuedTxs(tx1)
				},
			},
		},
	}

	testOptions := []suite.TestOption{
		{
			TestType:          "EVM LegacyTx & Cosmos LegacyTx",
			ApplyDynamicFeeTx: false,
		},
		{
			TestType:          "EVM Dynamic & Cosmos LegacyTx",
			ApplyDynamicFeeTx: false,
		},
	}

	s := suite.NewSystemTestSuite(t)
	s.SetupTest(t)

	for _, to := range testOptions {
		for _, tc := range testCases {
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
