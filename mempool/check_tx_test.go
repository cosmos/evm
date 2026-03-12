package mempool_test

import (
	"math/big"
	"testing"
	"time"

	ethcore "github.com/ethereum/go-ethereum/core"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/mempool"
	evmtxpool "github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type CheckTxHandlerTestSuite struct {
	suite.Suite
}

func TestCheckTxHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(CheckTxHandlerTestSuite))
}

func (s *CheckTxHandlerTestSuite) submitCheckTx(handler sdk.CheckTxHandler, txBytes []byte, txType abci.CheckTxType) *abci.ResponseCheckTx {
	res, err := handler(nil, &abci.RequestCheckTx{
		Tx:   txBytes,
		Type: txType,
	})
	s.Require().NoError(err)
	return res
}

func (s *CheckTxHandlerTestSuite) cosmosSelectContext() sdk.Context {
	storeKey := storetypes.NewKVStoreKey("test")
	transientKey := storetypes.NewTransientStoreKey("transient_test")
	return testutil.DefaultContext(storeKey, transientKey).WithBlockHeight(2)
}

func (s *CheckTxHandlerTestSuite) TestEVMCheckTx() {
	testCases := []struct {
		name   string
		setup  func() testMempool
		assert func(mp testMempool, handler sdk.CheckTxHandler)
	}{
		{
			name: "inserts new tx",
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				tx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 0, nil)
				txBytes, err := mp.txConfig.TxEncoder()(tx)
				s.Require().NoError(err)

				res := s.submitCheckTx(handler, txBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code)
				s.Require().NoError(mp.mp.GetTxPool().Sync())
				s.Equal(1, mp.mp.CountTx())
			},
		},
		{
			name: "returns insert error for duplicate tx",
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				tx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 0, nil)
				txBytes, err := mp.txConfig.TxEncoder()(tx)
				s.Require().NoError(err)

				firstRes := s.submitCheckTx(handler, txBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, firstRes.Code)
				s.Require().NoError(mp.mp.GetTxPool().Sync())

				secondRes := s.submitCheckTx(handler, txBytes, abci.CheckTxType_New)
				s.NotEqual(abci.CodeTypeOK, secondRes.Code)
				s.Contains(secondRes.Log, evmtxpool.ErrAlreadyKnown.Error())
				s.Require().NoError(mp.mp.GetTxPool().Sync())
				s.Equal(1, mp.mp.CountTx())
			},
		},
		{
			name: "replaces with higher fee tx",
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				lowFeeTx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 0, big.NewInt(1000))
				highFeeTx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 0, big.NewInt(2000))

				lowFeeTxBytes, err := mp.txConfig.TxEncoder()(lowFeeTx)
				s.Require().NoError(err)
				highFeeTxBytes, err := mp.txConfig.TxEncoder()(highFeeTx)
				s.Require().NoError(err)

				res := s.submitCheckTx(handler, lowFeeTxBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code)
				s.Require().NoError(mp.mp.GetTxPool().Sync())

				res = s.submitCheckTx(handler, highFeeTxBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code)
				s.Require().NoError(mp.mp.GetTxPool().Sync())

				legacyPool := mp.mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
				pending, queued := legacyPool.ContentFrom(mp.accounts[0].address)
				s.Len(pending, 1)
				s.Len(queued, 0)
				s.Equal(big.NewInt(2000), pending[0].GasPrice())
				s.Equal(1, mp.mp.CountTx())
			},
		},
		{
			name: "rejects underpriced replacement",
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				originalTx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 0, big.NewInt(1000))
				underpricedReplacementTx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 0, big.NewInt(1050))

				originalTxBytes, err := mp.txConfig.TxEncoder()(originalTx)
				s.Require().NoError(err)
				underpricedReplacementTxBytes, err := mp.txConfig.TxEncoder()(underpricedReplacementTx)
				s.Require().NoError(err)

				res := s.submitCheckTx(handler, originalTxBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code)
				s.Require().NoError(mp.mp.GetTxPool().Sync())

				res = s.submitCheckTx(handler, underpricedReplacementTxBytes, abci.CheckTxType_New)
				s.NotEqual(abci.CodeTypeOK, res.Code)
				s.Contains(res.Log, evmtxpool.ErrReplaceUnderpriced.Error())
				s.Require().NoError(mp.mp.GetTxPool().Sync())

				legacyPool := mp.mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
				pending, queued := legacyPool.ContentFrom(mp.accounts[0].address)
				s.Len(pending, 1)
				s.Len(queued, 0)
				s.Equal(big.NewInt(1000), pending[0].GasPrice())
				s.Equal(1, mp.mp.CountTx())
			},
		},
		{
			name: "accepts nonce gapped txs and promotes them when gap is filled",
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				queuedTx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 1, big.NewInt(1000))
				fillGapTx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 0, big.NewInt(1000))

				queuedTxBytes, err := mp.txConfig.TxEncoder()(queuedTx)
				s.Require().NoError(err)
				fillGapTxBytes, err := mp.txConfig.TxEncoder()(fillGapTx)
				s.Require().NoError(err)

				res := s.submitCheckTx(handler, queuedTxBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code)
				s.Require().NoError(mp.mp.GetTxPool().Sync())

				legacyPool := mp.mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
				pending, queued := legacyPool.ContentFrom(mp.accounts[0].address)
				s.Len(pending, 0)
				s.Len(queued, 1)
				s.Equal(uint64(1), queued[0].Nonce())
				s.Equal(0, mp.mp.CountTx())

				res = s.submitCheckTx(handler, fillGapTxBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code)
				s.Require().NoError(mp.mp.GetTxPool().Sync())

				pending, queued = legacyPool.ContentFrom(mp.accounts[0].address)
				s.Len(pending, 2)
				s.Len(queued, 0)
				s.Equal(uint64(0), pending[0].Nonce())
				s.Equal(uint64(1), pending[1].Nonce())
				s.Equal(2, mp.mp.CountTx())
			},
		},
		{
			name: "replaces queued tx with higher fee",
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				queuedTx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 1, big.NewInt(1000))
				replacementTx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 1, big.NewInt(2000))

				queuedTxBytes, err := mp.txConfig.TxEncoder()(queuedTx)
				s.Require().NoError(err)
				replacementTxBytes, err := mp.txConfig.TxEncoder()(replacementTx)
				s.Require().NoError(err)

				res := s.submitCheckTx(handler, queuedTxBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code)
				s.Require().NoError(mp.mp.GetTxPool().Sync())

				res = s.submitCheckTx(handler, replacementTxBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code)
				s.Require().NoError(mp.mp.GetTxPool().Sync())

				legacyPool := mp.mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
				pending, queued := legacyPool.ContentFrom(mp.accounts[0].address)
				s.Len(pending, 0)
				s.Len(queued, 1)
				s.Equal(big.NewInt(2000), queued[0].GasPrice())
				s.Equal(0, mp.mp.CountTx())
			},
		},
		{
			name:  "rejects lower nonce against advanced state",
			setup: func() testMempool { return setupMempoolWithAccountNonces(s.T(), []uint64{1}) },
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				tx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 0, big.NewInt(1000))
				txBytes, err := mp.txConfig.TxEncoder()(tx)
				s.Require().NoError(err)

				res := s.submitCheckTx(handler, txBytes, abci.CheckTxType_New)
				s.NotEqual(abci.CodeTypeOK, res.Code)
				s.Contains(res.Log, ethcore.ErrNonceTooLow.Error())
				s.Equal(0, mp.mp.CountTx())
			},
		},
		{
			name:  "returns queue full when insert queue is saturated",
			setup: func() testMempool { return setupMempoolWithInsertQueueSize(s.T(), 1, 0) },
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				tx := createMsgEthereumTx(s.T(), mp.txConfig, mp.accounts[0].key, 0, big.NewInt(1000))
				txBytes, err := mp.txConfig.TxEncoder()(tx)
				s.Require().NoError(err)

				res := s.submitCheckTx(handler, txBytes, abci.CheckTxType_New)
				s.NotEqual(abci.CodeTypeOK, res.Code)
				s.Contains(res.Log, mempool.ErrQueueFull.Error())
				s.Equal(0, mp.mp.CountTx())
			},
		},
		{
			name: "rejects malformed tx bytes",
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				res := s.submitCheckTx(handler, []byte("not-a-real-tx"), abci.CheckTxType_New)
				s.NotEqual(abci.CodeTypeOK, res.Code)
				s.NotEmpty(res.Log)
				s.Equal(0, mp.mp.CountTx())
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			mp := setupMempoolWithAccounts(s.T(), 1)
			if tc.setup != nil {
				mp = tc.setup()
			}
			handler := mempool.NewCheckTxHandler(mp.mp, false, time.Minute)
			tc.assert(mp, handler)
		})
	}
}

func (s *CheckTxHandlerTestSuite) TestCosmosCheckTx() {
	testCases := []struct {
		name        string
		numAccounts int
		assert      func(mp testMempool, handler sdk.CheckTxHandler)
	}{
		{
			name:        "inserts cosmos tx",
			numAccounts: 1,
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				tx := createTestCosmosTx(s.T(), mp.txConfig, mp.accounts[0].key, 0)
				txBytes, err := mp.txConfig.TxEncoder()(tx)
				s.Require().NoError(err)

				res := s.submitCheckTx(handler, txBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code, res.Log)
				s.Equal(1, mp.mp.CountTx())
			},
		},
		{
			name:        "replaces cosmos tx with higher fee",
			numAccounts: 1,
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				lowFeeTx := createTestCosmosTxWithFee(s.T(), mp.txConfig, mp.accounts[0].key, 0, 1000000)
				highFeeTx := createTestCosmosTxWithFee(s.T(), mp.txConfig, mp.accounts[0].key, 0, 2000000)

				lowFeeTxBytes, err := mp.txConfig.TxEncoder()(lowFeeTx)
				s.Require().NoError(err)
				highFeeTxBytes, err := mp.txConfig.TxEncoder()(highFeeTx)
				s.Require().NoError(err)

				res := s.submitCheckTx(handler, lowFeeTxBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code, res.Log)

				res = s.submitCheckTx(handler, highFeeTxBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code, res.Log)
				s.Equal(1, mp.mp.CountTx())

				iter := mp.mp.Select(s.cosmosSelectContext(), nil)
				s.Require().NotNil(iter)

				feeTx, ok := iter.Tx().(sdk.FeeTx)
				s.True(ok)
				s.EqualValues(2000000, feeTx.GetFee()[0].Amount.Int64())
				s.Nil(iter.Next())
			},
		},
		{
			name:        "replaces multi signer cosmos tx with higher fee",
			numAccounts: 2,
			assert: func(mp testMempool, handler sdk.CheckTxHandler) {
				originalTx := createTestMultiSignerCosmosTxWithFee(s.T(), mp.txConfig, 1000000, mp.accounts[0].key, mp.accounts[1].key)
				replacementTx := createTestMultiSignerCosmosTxWithFee(s.T(), mp.txConfig, 2000000, mp.accounts[0].key, mp.accounts[1].key)

				originalTxBytes, err := mp.txConfig.TxEncoder()(originalTx)
				s.Require().NoError(err)
				replacementTxBytes, err := mp.txConfig.TxEncoder()(replacementTx)
				s.Require().NoError(err)

				res := s.submitCheckTx(handler, originalTxBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code, res.Log)

				res = s.submitCheckTx(handler, replacementTxBytes, abci.CheckTxType_New)
				s.Equal(abci.CodeTypeOK, res.Code, res.Log)
				s.Equal(1, mp.mp.CountTx())

				iter := mp.mp.Select(s.cosmosSelectContext(), nil)
				s.Require().NotNil(iter)

				feeTx, ok := iter.Tx().(sdk.FeeTx)
				s.True(ok)
				s.EqualValues(2000000, feeTx.GetFee()[0].Amount.Int64())
				s.Nil(iter.Next())
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			mp := setupMempoolWithAccounts(s.T(), tc.numAccounts)
			handler := mempool.NewCheckTxHandler(mp.mp, false, time.Minute)
			tc.assert(mp, handler)
		})
	}
}

func (s *CheckTxHandlerTestSuite) TestRecheckIsNoOp() {
	mp := setupMempoolWithAccounts(s.T(), 1)
	handler := mempool.NewCheckTxHandler(mp.mp, false, time.Minute)

	require.Panics(s.T(), func() {
		s.submitCheckTx(handler, []byte("not-a-real-tx"), abci.CheckTxType_Recheck)
	})
}
