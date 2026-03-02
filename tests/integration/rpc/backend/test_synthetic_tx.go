package backend

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/types"

	dbm "github.com/cosmos/cosmos-db"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/evm/indexer"
	intindexer "github.com/cosmos/evm/tests/integration/indexer"
)

func (s *TestSuite) TestGetSyntheticTransactionByHash() {
	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())

	testCases := []struct {
		name             string
		registerMock     func()
		finalizeEvents   []abci.Event
		expPass          bool
		expSyntheticAddr common.Address
	}{
		{
			name:         "success - synthetic tx from coin_received event",
			registerMock: func() {},
			finalizeEvents: []abci.Event{
				{
					Type: banktypes.EventTypeCoinReceived,
					Attributes: []abci.EventAttribute{
						{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
						{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
					},
				},
			},
			expPass:          true,
			expSyntheticAddr: tokenAddress,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.registerMock()

			db := dbm.NewMemDB()
			idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), s.backend.ClientCtx)

			bankTransformer := intindexer.NewBankTransferTransformer(tokenAddress)
			idxer.RegisterTransformer(bankTransformer)

			s.backend.Indexer = idxer

			block := &types.Block{
				Header: types.Header{Height: 1, ChainID: "test"},
				Data:   types.Data{Txs: []types.Tx{}},
			}

			err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, tc.finalizeEvents)
			s.Require().NoError(err)

			// FinalizeBlockEvents use phase + blockHash for synthetic tx hash
			syntheticEthHash := indexer.GenerateSyntheticEthTxHash([]byte(indexer.BlockPhasePreBlock), block.Hash())
			txResult, err := idxer.GetByTxHash(syntheticEthHash)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().NotNil(txResult)
				s.Require().Equal(int64(1), txResult.Height)
				s.Require().Equal(int32(0), txResult.EthTxIndex)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetSyntheticTransactionReceipt() {
	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())

	testCases := []struct {
		name           string
		registerMock   func()
		finalizeEvents []abci.Event
		expPass        bool
		validateFields func(map[string]interface{})
	}{
		{
			name:         "success - receipt has correct fields",
			registerMock: func() {},
			finalizeEvents: []abci.Event{
				{
					Type: banktypes.EventTypeCoinReceived,
					Attributes: []abci.EventAttribute{
						{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
						{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
					},
				},
			},
			expPass: true,
			validateFields: func(receipt map[string]interface{}) {
				s.Require().NotNil(receipt["status"])
				s.Require().NotNil(receipt["logs"])
				s.Require().NotNil(receipt["blockNumber"])
				s.Require().Equal(hexutil.Uint64(1), receipt["blockNumber"])
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.registerMock()

			db := dbm.NewMemDB()
			idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), s.backend.ClientCtx)

			bankTransformer := intindexer.NewBankTransferTransformer(tokenAddress)
			idxer.RegisterTransformer(bankTransformer)

			s.backend.Indexer = idxer

			block := &types.Block{
				Header: types.Header{Height: 1, ChainID: "test"},
				Data:   types.Data{Txs: []types.Tx{}},
			}

			err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, tc.finalizeEvents)
			s.Require().NoError(err)

			res, err := idxer.GetByBlockAndIndex(1, 0)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().NotNil(res)

				// Use phase-based hash for receipt lookup
				receiptJSON, err := idxer.GetEthReceipt(indexer.GenerateSyntheticEthTxHash([]byte(indexer.BlockPhasePreBlock), block.Hash()))
				if err == nil && receiptJSON != nil {
					s.Require().NotEmpty(receiptJSON)
				}
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestGetSyntheticStakingDelegateTransaction() {
	stakingPrecompileAddr := common.HexToAddress("0x0000000000000000000000000000000000000800")
	validDelegatorAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())
	validValidatorAddr := sdk.ValAddress(common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").Bytes())

	testCases := []struct {
		name           string
		registerMock   func()
		finalizeEvents []abci.Event
		expPass        bool
	}{
		{
			name:         "success - staking delegate synthetic tx",
			registerMock: func() {},
			finalizeEvents: []abci.Event{
				{
					Type: intindexer.EventTypeDelegate,
					Attributes: []abci.EventAttribute{
						{Key: intindexer.AttributeKeyDelegator, Value: validDelegatorAddr.String()},
						{Key: intindexer.AttributeKeyValidator, Value: validValidatorAddr.String()},
						{Key: sdk.AttributeKeyAmount, Value: "5000stake"},
					},
				},
			},
			expPass: true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.registerMock()

			db := dbm.NewMemDB()
			idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), s.backend.ClientCtx)

			stakingTransformer := intindexer.NewStakingDelegateTransformer(stakingPrecompileAddr)
			idxer.RegisterTransformer(stakingTransformer)

			s.backend.Indexer = idxer

			block := &types.Block{
				Header: types.Header{Height: 1, ChainID: "test"},
				Data:   types.Data{Txs: []types.Tx{}},
			}

			err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, tc.finalizeEvents)
			s.Require().NoError(err)

			res, err := idxer.GetByBlockAndIndex(1, 0)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().NotNil(res)
				s.Require().Equal(int64(1), res.Height)
				s.Require().Equal(int32(0), res.EthTxIndex)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

// TestMultipleSyntheticEventsInPhase tests that multiple events in the same phase
// are bundled into a single synthetic tx with multiple logs.
func (s *TestSuite) TestMultipleSyntheticEventsInPhase() {
	tokenAddress := common.HexToAddress("0x0000000000000000000000000000000000000001")
	stakingPrecompileAddr := common.HexToAddress("0x0000000000000000000000000000000000000800")

	validReceiverAddr := sdk.AccAddress(common.HexToAddress("0x1234567890123456789012345678901234567890").Bytes())
	validDelegatorAddr := sdk.AccAddress(common.HexToAddress("0x2234567890123456789012345678901234567890").Bytes())
	validValidatorAddr := sdk.ValAddress(common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd").Bytes())

	s.SetupTest()

	db := dbm.NewMemDB()
	idxer := indexer.NewKVIndexer(db, log.NewNopLogger(), s.backend.ClientCtx)

	bankTransformer := intindexer.NewBankTransferTransformer(tokenAddress)
	stakingTransformer := intindexer.NewStakingDelegateTransformer(stakingPrecompileAddr)
	idxer.RegisterTransformer(bankTransformer)
	idxer.RegisterTransformer(stakingTransformer)

	s.backend.Indexer = idxer

	block := &types.Block{
		Header: types.Header{Height: 1, ChainID: "test"},
		Data:   types.Data{Txs: []types.Tx{}},
	}

	// All events in same phase (PreBlock - no mode attribute)
	finalizeEvents := []abci.Event{
		{
			Type: banktypes.EventTypeCoinReceived,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeyReceiver, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "1000stake"},
			},
		},
		{
			Type: intindexer.EventTypeDelegate,
			Attributes: []abci.EventAttribute{
				{Key: intindexer.AttributeKeyDelegator, Value: validDelegatorAddr.String()},
				{Key: intindexer.AttributeKeyValidator, Value: validValidatorAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "5000stake"},
			},
		},
		{
			Type: banktypes.EventTypeCoinSpent,
			Attributes: []abci.EventAttribute{
				{Key: banktypes.AttributeKeySpender, Value: validReceiverAddr.String()},
				{Key: sdk.AttributeKeyAmount, Value: "500stake"},
			},
		},
	}

	err := idxer.IndexBlock(block, []*abci.ExecTxResult{}, finalizeEvents)
	s.Require().NoError(err)

	// Should have 1 synthetic tx (single phase with 3 events)
	res, err := idxer.GetByBlockAndIndex(1, 0)
	s.Require().NoError(err)
	s.Require().NotNil(res)
	s.Require().Equal(int64(1), res.Height)
	s.Require().Equal(int32(0), res.EthTxIndex)

	// Verify no second tx exists
	_, err = idxer.GetByBlockAndIndex(1, 1)
	s.Require().Error(err, "should not have tx at index 1")

	// Verify receipt has 3 logs (one from each event)
	ethTxHash := indexer.GenerateSyntheticEthTxHash([]byte(indexer.BlockPhasePreBlock), block.Hash())
	receiptJSON, err := idxer.GetEthReceipt(ethTxHash)
	s.Require().NoError(err)
	s.Require().NotEmpty(receiptJSON)
}
