package backend

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/rpc/backend/mocks"
	ethrpc "github.com/cosmos/evm/rpc/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func (s *TestSuite) TestGetLogs() {
	_, bz := s.buildEthereumTx()
	block := cmttypes.MakeBlock(1, []cmttypes.Tx{bz}, nil, nil)
	logs := []*evmtypes.Log{
		{
			Data:        []byte("data"),
			BlockNumber: 1,
		},
	}

	testCases := []struct {
		name         string
		registerMock func(hash common.Hash)
		blockHash    common.Hash
		expLogs      [][]*ethtypes.Log
		expPass      bool
	}{
		{
			"fail - no block with that hash",
			func(hash common.Hash) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHashNotFound(client, hash, bz)
			},
			common.Hash{},
			nil,
			false,
		},
		{
			"fail - error fetching block by hash",
			func(hash common.Hash) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHashError(client, hash, bz)
			},
			common.Hash{},
			nil,
			false,
		},
		{
			"fail - error getting block results",
			func(hash common.Hash) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHash(client, hash, bz)
				RegisterBlockResultsError(client, 1)
			},
			common.Hash{},
			nil,
			false,
		},
		{
			"success - getting logs with block hash",
			func(hash common.Hash) {
				client := s.backend.ClientCtx.Client.(*mocks.Client)
				RegisterBlockByHash(client, hash, bz)
				_, err := RegisterBlockResultsWithEventLog(client, ethrpc.BlockNumber(1).Int64())
				s.Require().NoError(err)
			},
			common.BytesToHash(block.Hash()),
			[][]*ethtypes.Log{evmtypes.LogsToEthereum(logs)},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			tc.registerMock(tc.blockHash)
			logs, err := s.backend.GetLogs(tc.blockHash)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(tc.expLogs, logs)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *TestSuite) TestBloomStatus() {
	testCases := []struct {
		name         string
		registerMock func()
		expResult    uint64
		expPass      bool
	}{
		{
			"pass - returns the BloomBitsBlocks and the number of processed sections maintained",
			func() {},
			4096,
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			tc.registerMock()
			bloom, _ := s.backend.BloomStatus()

			if tc.expPass {
				s.Require().Equal(tc.expResult, bloom)
			}
		})
	}
}
