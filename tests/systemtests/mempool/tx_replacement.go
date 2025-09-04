package mempool

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/stretchr/testify/require"
)

func TestTransactionReplacement(t *testing.T) {
	testCases := []struct {
		name       string
		malleate   func(s TestSuite)
		postAction func(s TestSuite)
		bypass     bool
	}{
		{
			name: "Replacement of single pending tx %s",
			malleate: func(s TestSuite) {
				lowFeeEVMTxHash, err := s.SendTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
				require.NoError(t, err)

				highGasEVMTxHash, err := s.SendTx(s.GetNode(), "acc0", 0, s.BaseFeeX2(), big.NewInt(1))
				require.NoError(t, err)

				if s.OnlyEthTxs() {
					s.SetExpQueuedTxs(highGasEVMTxHash, lowFeeEVMTxHash)
				}
			},
			postAction: func(s TestSuite) {},
			bypass:     true,
		},
		{
			name: "Replacement of multiple pending txs %s",
			malleate: func(s TestSuite) {
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
			postAction: func(s TestSuite) {},
			bypass:     true,
		},
		{
			name: "Replacement of single queued tx %s",
			malleate: func(s TestSuite) {
				_, err := s.SendTx(s.GetNode(), "acc0", 1, s.BaseFee(), nil)
				require.NoError(t, err)

				tx2, err := s.SendTx(s.GetNode(), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
				require.NoError(t, err)

				if s.OnlyEthTxs() {
					s.SetExpQueuedTxs(tx2)
				}
			},
			postAction: func(s TestSuite) {
				txHash, err := s.SendTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
				require.NoError(t, err)

				s.SetExpPendingTxs(txHash)
				if s.OnlyEthTxs() {
					s.PromoteExpTxs(1)
				}
			},
		},
		{
			name: "Replacement of multiple queued txs %s",
			malleate: func(s TestSuite) {
				_, err := s.SendTx(s.GetNode(), "acc0", 1, s.BaseFee(), nil)
				require.NoError(t, err)

				tx2, err := s.SendTx(s.GetNode(), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
				require.NoError(t, err)

				_, err = s.SendTx(s.GetNode(), "acc0", 2, s.BaseFee(), nil)
				require.NoError(t, err)

				tx4, err := s.SendTx(s.GetNode(), "acc0", 2, s.BaseFeeX2(), big.NewInt(1))
				require.NoError(t, err)

				_, err = s.SendTx(s.GetNode(), "acc0", 2, s.BaseFee(), nil)
				require.NoError(t, err)

				tx6, err := s.SendTx(s.GetNode(), "acc0", 2, s.BaseFeeX2(), big.NewInt(1))
				require.NoError(t, err)

				if s.OnlyEthTxs() {
					s.SetExpQueuedTxs(tx2, tx4, tx6)
				}
			},
			postAction: func(s TestSuite) {
				txHash, err := s.SendTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
				require.NoError(t, err)

				s.SetExpPendingTxs(txHash)
				if s.OnlyEthTxs() {
					s.PromoteExpTxs(2)
				}
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

				tc.malleate(s)
				tc.postAction(s)

				s.AfterEach(t)
			})
		}
	}
}
