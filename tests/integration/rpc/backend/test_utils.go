package backend

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cometbft/cometbft/proto/tendermint/crypto"
	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	"github.com/cometbft/cometbft/types"

	backend2 "github.com/cosmos/evm/rpc/backend"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func mookProofs(num int, withData bool) *crypto.ProofOps {
	var proofOps *crypto.ProofOps
	if num > 0 {
		proofOps = new(crypto.ProofOps)
		for i := 0; i < num; i++ {
			proof := crypto.ProofOp{}
			if withData {
				proof.Data = []byte("\n\031\n\003KEY\022\005VALUE\032\013\010\001\030\001 \001*\003\000\002\002")
			}
			proofOps.Ops = append(proofOps.Ops, proof)
		}
	}
	return proofOps
}

func (s *TestSuite) TestGetHexProofs() {
	defaultRes := []string{""}
	testCases := []struct {
		name  string
		proof *crypto.ProofOps
		exp   []string
	}{
		{
			"no proof provided",
			mookProofs(0, false),
			defaultRes,
		},
		{
			"no proof data provided",
			mookProofs(1, false),
			defaultRes,
		},
		{
			"valid proof provided",
			mookProofs(1, true),
			[]string{"0x0a190a034b4559120556414c55451a0b0801180120012a03000202"},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.Require().Equal(tc.exp, backend2.GetHexProofs(tc.proof))
		})
	}
}

func (s *TestSuite) TestFindEthTxIndexByHash() {
	txHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	tx := evmtypes.NewTx(&evmtypes.EvmTxArgs{
		ChainID:  s.backend.EvmChainID,
		Nonce:    0,
		GasLimit: 100000,
	})
	tx.From = s.from.Bytes()
	txEncoder := s.backend.ClientCtx.TxConfig.TxEncoder()
	builder := s.backend.ClientCtx.TxConfig.NewTxBuilder()
	err := builder.SetMsgs(tx)
	s.Require().NoError(err)
	txBz, err := txEncoder(builder.GetTx())
	s.Require().NoError(err)

	testCases := []struct {
		name        string
		setupMock   func() (*cmtrpctypes.ResultBlock, *cmtrpctypes.ResultBlockResults)
		txHash      common.Hash
		expectError bool
		errorMsg    string
		expectedIdx int32
	}{
		{
			name: "tx found at index 0",
			setupMock: func() (*cmtrpctypes.ResultBlock, *cmtrpctypes.ResultBlockResults) {
				block := &cmtrpctypes.ResultBlock{
					Block: &types.Block{
						Header: types.Header{Height: 1},
						Data: types.Data{
							Txs: []types.Tx{txBz},
						},
					},
				}
				blockRes := &cmtrpctypes.ResultBlockResults{
					TxsResults: []*abci.ExecTxResult{
						{Code: 0, GasUsed: 21000},
					},
				}
				return block, blockRes
			},
			txHash:      tx.Hash(),
			expectError: false,
			expectedIdx: 0,
		},
		{
			name: "tx not found",
			setupMock: func() (*cmtrpctypes.ResultBlock, *cmtrpctypes.ResultBlockResults) {
				otherTx := evmtypes.NewTx(&evmtypes.EvmTxArgs{
					ChainID:  s.backend.EvmChainID,
					Nonce:    1,
					GasLimit: 100000,
				})
				otherTx.From = s.from.Bytes()

				builder := s.backend.ClientCtx.TxConfig.NewTxBuilder()
				_ = builder.SetMsgs(otherTx)
				otherTxBz, _ := txEncoder(builder.GetTx())

				block := &cmtrpctypes.ResultBlock{
					Block: &types.Block{
						Header: types.Header{Height: 1},
						Data: types.Data{
							Txs: []types.Tx{otherTxBz},
						},
					},
				}
				blockRes := &cmtrpctypes.ResultBlockResults{
					TxsResults: []*abci.ExecTxResult{
						{Code: 0, GasUsed: 21000},
					},
				}
				return block, blockRes
			},
			txHash:      txHash, // Different hash
			expectError: true,
			errorMsg:    "can't find index of ethereum tx",
		},
		{
			name: "empty block",
			setupMock: func() (*cmtrpctypes.ResultBlock, *cmtrpctypes.ResultBlockResults) {
				block := &cmtrpctypes.ResultBlock{
					Block: &types.Block{
						Header: types.Header{Height: 1},
						Data:   types.Data{Txs: []types.Tx{}},
					},
				}
				blockRes := &cmtrpctypes.ResultBlockResults{
					TxsResults: []*abci.ExecTxResult{},
				}
				return block, blockRes
			},
			txHash:      txHash,
			expectError: true,
			errorMsg:    "can't find index of ethereum tx",
		},
		{
			name: "tx with failed result code",
			setupMock: func() (*cmtrpctypes.ResultBlock, *cmtrpctypes.ResultBlockResults) {
				block := &cmtrpctypes.ResultBlock{
					Block: &types.Block{
						Header: types.Header{Height: 1},
						Data: types.Data{
							Txs: []types.Tx{txBz},
						},
					},
				}
				blockRes := &cmtrpctypes.ResultBlockResults{
					TxsResults: []*abci.ExecTxResult{
						{Code: 1, GasUsed: 21000, Log: "execution reverted"}, // Failed tx
					},
				}
				return block, blockRes
			},
			txHash:      tx.Hash(),
			expectError: true,
			errorMsg:    "can't find index of ethereum tx", // Will be filtered out by EthMsgsFromCometBlock
		},
		{
			name: "multiple txs, target at index 1",
			setupMock: func() (*cmtrpctypes.ResultBlock, *cmtrpctypes.ResultBlockResults) {
				tx1 := evmtypes.NewTx(&evmtypes.EvmTxArgs{
					ChainID:  s.backend.EvmChainID,
					Nonce:    0,
					GasLimit: 100000,
				})
				tx1.From = s.from.Bytes()

				builder1 := s.backend.ClientCtx.TxConfig.NewTxBuilder()
				_ = builder1.SetMsgs(tx1)
				tx1Bz, _ := txEncoder(builder1.GetTx())

				tx2 := evmtypes.NewTx(&evmtypes.EvmTxArgs{
					ChainID:  s.backend.EvmChainID,
					Nonce:    1,
					GasLimit: 100000,
				})
				tx2.From = s.from.Bytes()

				builder2 := s.backend.ClientCtx.TxConfig.NewTxBuilder()
				_ = builder2.SetMsgs(tx2)
				tx2Bz, _ := txEncoder(builder2.GetTx())

				block := &cmtrpctypes.ResultBlock{
					Block: &types.Block{
						Header: types.Header{Height: 1},
						Data: types.Data{
							Txs: []types.Tx{tx1Bz, tx2Bz},
						},
					},
				}
				blockRes := &cmtrpctypes.ResultBlockResults{
					TxsResults: []*abci.ExecTxResult{
						{Code: 0, GasUsed: 21000},
						{Code: 0, GasUsed: 21000},
					},
				}
				return block, blockRes
			},
			txHash: func() common.Hash {
				tx2 := evmtypes.NewTx(&evmtypes.EvmTxArgs{
					ChainID:  s.backend.EvmChainID,
					Nonce:    1,
					GasLimit: 100000,
				})
				tx2.From = s.from.Bytes()
				return tx2.Hash()
			}(),
			expectError: false,
			expectedIdx: 1,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			block, blockRes := tc.setupMock()

			idx, err := backend2.FindEthTxIndexByHash(tc.txHash, block, blockRes, s.backend)

			if tc.expectError {
				s.Require().Error(err)
				s.Require().Contains(err.Error(), tc.errorMsg)
				s.Require().Equal(int32(-1), idx)
			} else {
				s.Require().NoError(err)
				s.Require().Equal(tc.expectedIdx, idx)
			}
		})
	}
}
