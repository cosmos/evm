package mempool

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/stretchr/testify/require"
)

func TestNonceGappedTxs(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s TestSuite)
		bypass  bool
	}{
		{
			name: "Single nonce gap fill %s",
			actions: []func(s TestSuite){
				func(s TestSuite) {
					lowFeeEVMTxHash, err := s.SendTx(s.GetNode(), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err)

					highGasEVMTxHash, err := s.SendTx(s.GetNode(), "acc0", 1, s.BaseFeeX2(), big.NewInt(1))
					require.NoError(t, err)

					if s.OnlyEthTxs() {
						s.SetExpQueuedTxs(highGasEVMTxHash, lowFeeEVMTxHash)
					}
				},
				func(s TestSuite) {
					txHash, err := s.SendTx(s.GetNode(), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err)

					s.SetExpPendingTxs(txHash)
					if s.OnlyEthTxs() {
						s.PromoteExpTxs(0)
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
