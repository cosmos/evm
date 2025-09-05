package mempool

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/test-go/testify/require"
)

func TestNonceGappedTxs(t *testing.T) {
	testCases := []struct {
		name    string
		actions []func(s suite.TestSuite)
		bypass  bool
	}{
		{
			name: "Single nonce gap fill %s",
			actions: []func(s suite.TestSuite){
				func(s suite.TestSuite) {
					tx1, err := s.SendTx(t, s.GetNode(), "acc0", 1, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")
					tx2, err := s.SendTx(t, s.GetNode(), "acc0", 1, s.BaseFeeX10(), big.NewInt(10000))
					require.NoError(t, err, "failed to send tx")

					s.SetExpDiscardedTxs(tx1)
					s.SetExpQueuedTxs(tx2)
				},
				func(s suite.TestSuite) {
					txHash, err := s.SendTx(t, s.GetNode(), "acc0", 0, s.BaseFee(), nil)
					require.NoError(t, err, "failed to send tx")

					s.SetExpPendingTxs(txHash)
					s.PromoteExpTxs(1)
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
					// s.JustAfterEach(t)
				}
				s.AfterEach(t)
			})
		}
	}
}
