package mempool

import (
	"math/big"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"

	evmmempool "github.com/cosmos/evm/mempool"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestPerfRecheckAndProposal logs recheck-pass and steady-state
// PrepareProposal timings over 200 cosmos txs.
func (s *IntegrationTestSuite) TestPerfRecheckAndProposal() {
	const (
		signers         = 20
		noncesPerSigner = 10
		iters           = 10
		gasLimit        = 200000
	)

	kMp, ok := s.network.App.GetMempool().(*evmmempool.Mempool)
	if !ok {
		s.T().Skip("EVM mempool not configured")
	}

	txs := make([]sdk.Tx, 0, signers*noncesPerSigner)
	for nonce := range noncesPerSigner {
		for i := range signers {
			txs = append(txs, s.createCosmosSendTxWithNonceAndGas(
				s.keyring.GetKey(i), uint64(nonce), big.NewInt(1000), gasLimit, big.NewInt(1000000000),
			))
		}
	}
	s.Require().NoError(s.insertTxs(txs))

	bench := func(fn func()) time.Duration {
		fn() // warm
		start := time.Now()
		for range iters {
			fn()
		}
		return time.Since(start) / iters
	}

	head := kMp.GetBlockchain().CurrentBlock()
	perPass := bench(func() { kMp.RecheckCosmosTxs(head) })

	_, err := s.network.FinalizeBlock()
	s.Require().NoError(err)

	height := s.network.GetContext().BlockHeight() + 1
	perProposal := bench(func() {
		res, err := s.network.App.PrepareProposal(&abci.RequestPrepareProposal{
			MaxTxBytes: 10_000_000,
			Height:     height,
		})
		s.Require().NoError(err)
		s.Require().Len(res.Txs, len(txs))
	})

	perTx := time.Duration(len(txs))
	s.T().Logf("PERF txs=%d recheck_pass=%v (%v/tx) proposal=%v (%v/tx)",
		len(txs), perPass, perPass/perTx, perProposal, perProposal/perTx)
}
