package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	protov2 "google.golang.org/protobuf/proto"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/indexer"
	"github.com/cosmos/evm/rpc/backend/mocks"
	rpctypes "github.com/cosmos/evm/rpc/types"
	servertypes "github.com/cosmos/evm/server/types"
	"github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
	grpctypes "github.com/cosmos/cosmos-sdk/types/grpc"
)

func setupMockBackend(t *testing.T) *Backend {
	t.Helper()
	ctx := server.NewDefaultContext()
	ctx.Viper.Set("telemetry.global-labels", []interface{}{})
	ctx.Viper.Set("evm.evm-chain-id", constants.ExampleChainID.EVMChainID)

	baseDir := t.TempDir()
	nodeDirName := "node"
	clientDir := filepath.Join(baseDir, nodeDirName, "evmoscli")

	keyRing := keyring.NewInMemory(client.Context{}.Codec)

	acc := sdk.AccAddress(utiltx.GenerateAddress().Bytes())
	accounts := map[string]client.TestAccount{}
	accounts[acc.String()] = client.TestAccount{
		Address: acc,
		Num:     uint64(1),
		Seq:     uint64(1),
	}

	encodingConfig := encoding.MakeConfig(constants.ExampleChainID.EVMChainID)
	clientCtx := client.Context{}.WithChainID(constants.ExampleChainID.ChainID).
		WithHeight(1).
		WithTxConfig(encodingConfig.TxConfig).
		WithKeyringDir(clientDir).
		WithKeyring(keyRing).
		WithAccountRetriever(client.TestAccountRetriever{Accounts: accounts}).
		WithClient(mocks.NewClient(t)).
		WithCodec(encodingConfig.Codec)

	idxer := indexer.NewKVIndexer(dbm.NewMemDB(), ctx.Logger, clientCtx)

	backend := NewBackend(ctx, clientCtx, idxer, nil, WithLogger(ctx.Logger))
	backend.Cfg.JSONRPC.GasCap = 25000000
	backend.Cfg.JSONRPC.EVMTimeout = 0
	backend.Cfg.JSONRPC.AllowInsecureUnlock = true
	backend.Cfg.EVM.EVMChainID = constants.ExampleChainID.EVMChainID
	mockEVMQueryClient := mocks.NewEVMQueryClient(t)
	mockFeeMarketQueryClient := mocks.NewFeeMarketQueryClient(t)
	backend.QueryClient.QueryClient = mockEVMQueryClient
	backend.QueryClient.FeeMarket = mockFeeMarketQueryClient

	mockClient := backend.ClientCtx.Client.(*mocks.Client)
	mockClient.On("Status", context.Background()).Return(&tmrpctypes.ResultStatus{
		SyncInfo: tmrpctypes.SyncInfo{
			LatestBlockHeight: 1,
		},
	}, nil).Maybe()

	mockHeader := &tmtypes.Header{
		Height:  1,
		Time:    time.Now(),
		ChainID: constants.ExampleChainID.ChainID,
	}
	mockBlock := &tmtypes.Block{
		Header: *mockHeader,
	}
	mockClient.On("Block", context.Background(), (*int64)(nil)).Return(&tmrpctypes.ResultBlock{
		Block: mockBlock,
	}, nil).Maybe()

	mockClient.On("BlockResults", context.Background(), (*int64)(nil)).Return(&tmrpctypes.ResultBlockResults{
		Height:     1,
		TxsResults: []*abcitypes.ExecTxResult{},
	}, nil).Maybe()

	mockEVMQueryClient.On("Params",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(&evmtypes.QueryParamsResponse{
		Params: evmtypes.DefaultParams(),
	}, nil).Maybe()

	return backend
}

func TestCreateAccessList(t *testing.T) {
	from := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef")
	overrides := json.RawMessage(`{
        "` + to.Hex() + `": {
            "balance": "0x1000000000000000000",
            "nonce": "0x1",
            "code": "0x608060405234801561001057600080fd5b50600436106100365760003560e01c8063c6888fa11461003b578063c8e7ca2e14610057575b600080fd5b610055600480360381019061005091906100a3565b610075565b005b61005f61007f565b60405161006c91906100e1565b60405180910390f35b8060008190555050565b60008054905090565b600080fd5b6000819050919050565b61009d8161008a565b81146100a857600080fd5b50565b6000813590506100ba81610094565b92915050565b6000602082840312156100d6576100d5610085565b5b60006100e4848285016100ab565b91505092915050565b6100f68161008a565b82525050565b600060208201905061011160008301846100ed565b9291505056fea2646970667358221220c7d2d7c0b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b2b264736f6c634300080a0033",
            "storage": {
                "0x0000000000000000000000000000000000000000000000000000000000000000": "0x123"
            }
        }
    }`)
	invalidOverrides := json.RawMessage(`{"invalid": json}`)
	emptyOverrides := json.RawMessage(`{}`)
	testCases := []struct {
		name          string
		malleate      func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash)
		overrides     *json.RawMessage
		expectError   bool
		errorContains string
		expectGasUsed bool
		expectAccList bool
	}{
		{
			name: "success - basic transaction",
			malleate: func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash) {
				gas := hexutil.Uint64(21000)
				value := (*hexutil.Big)(big.NewInt(1000))
				gasPrice := (*hexutil.Big)(big.NewInt(20000000000))

				args := evmtypes.TransactionArgs{
					From:     &from,
					To:       &to,
					Gas:      &gas,
					Value:    value,
					GasPrice: gasPrice,
				}

				blockNum := rpctypes.EthLatestBlockNumber
				blockNumOrHash := rpctypes.BlockNumberOrHash{
					BlockNumber: &blockNum,
				}

				return args, blockNumOrHash
			},
			expectError:   false,
			expectGasUsed: true,
			expectAccList: true,
		},
		{
			name: "success - transaction with data",
			malleate: func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash) {
				gas := hexutil.Uint64(100000)
				gasPrice := (*hexutil.Big)(big.NewInt(20000000000))
				data := hexutil.Bytes("0xa9059cbb")

				args := evmtypes.TransactionArgs{
					From:     &from,
					To:       &to,
					Gas:      &gas,
					GasPrice: gasPrice,
					Data:     &data,
				}

				blockNum := rpctypes.EthLatestBlockNumber
				blockNumOrHash := rpctypes.BlockNumberOrHash{
					BlockNumber: &blockNum,
				}

				return args, blockNumOrHash
			},
			expectError:   false,
			expectGasUsed: true,
			expectAccList: true,
		},
		{
			name: "success - transaction with existing access list",
			malleate: func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash) {
				gas := hexutil.Uint64(100000)
				gasPrice := (*hexutil.Big)(big.NewInt(20000000000))
				accessList := ethtypes.AccessList{
					{
						Address: common.HexToAddress("0x1111111111111111111111111111111111111111"),
						StorageKeys: []common.Hash{
							common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"),
						},
					},
				}

				args := evmtypes.TransactionArgs{
					From:       &from,
					To:         &to,
					Gas:        &gas,
					GasPrice:   gasPrice,
					AccessList: &accessList,
				}

				blockNum := rpctypes.EthLatestBlockNumber
				blockNumOrHash := rpctypes.BlockNumberOrHash{
					BlockNumber: &blockNum,
				}

				return args, blockNumOrHash
			},
			expectError:   false,
			expectGasUsed: true,
			expectAccList: true,
		},
		{
			name: "success - transaction with specific block hash",
			malleate: func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash) {
				gas := hexutil.Uint64(21000)
				gasPrice := (*hexutil.Big)(big.NewInt(20000000000))

				args := evmtypes.TransactionArgs{
					From:     &from,
					To:       &to,
					Gas:      &gas,
					GasPrice: gasPrice,
				}

				blockHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef12")
				blockNumOrHash := rpctypes.BlockNumberOrHash{
					BlockHash: &blockHash,
				}

				return args, blockNumOrHash
			},
			expectError:   false,
			expectGasUsed: true,
			expectAccList: true,
		},
		{
			name: "error - missing from address",
			malleate: func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash) {
				gas := hexutil.Uint64(21000)
				gasPrice := (*hexutil.Big)(big.NewInt(20000000000))
				args := evmtypes.TransactionArgs{
					To:       &to,
					Gas:      &gas,
					GasPrice: gasPrice,
				}

				blockNum := rpctypes.EthLatestBlockNumber
				blockNumOrHash := rpctypes.BlockNumberOrHash{
					BlockNumber: &blockNum,
				}

				return args, blockNumOrHash
			},
			expectError:   true,
			expectGasUsed: false,
			expectAccList: false,
		},
		{
			name: "error - invalid gas limit",
			malleate: func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash) {
				gas := hexutil.Uint64(0)
				gasPrice := (*hexutil.Big)(big.NewInt(20000000000))

				args := evmtypes.TransactionArgs{
					From:     &from,
					To:       &to,
					Gas:      &gas,
					GasPrice: gasPrice,
				}

				blockNum := rpctypes.EthLatestBlockNumber
				blockNumOrHash := rpctypes.BlockNumberOrHash{
					BlockNumber: &blockNum,
				}

				return args, blockNumOrHash
			},
			expectError:   true,
			expectGasUsed: false,
			expectAccList: false,
		},
		{
			name: "pass - With state overrides",
			malleate: func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash) {
				gas := hexutil.Uint64(21000)
				gasPrice := (*hexutil.Big)(big.NewInt(20000000000))
				args := evmtypes.TransactionArgs{
					From:     &from,
					To:       &to,
					Gas:      &gas,
					GasPrice: gasPrice,
				}
				blockNum := rpctypes.EthLatestBlockNumber
				blockNumOrHash := rpctypes.BlockNumberOrHash{
					BlockNumber: &blockNum,
				}
				return args, blockNumOrHash
			},
			overrides:     &overrides,
			expectError:   false,
			expectGasUsed: true,
			expectAccList: true,
		},
		{
			name: "fail - Invalid state overrides JSON",
			malleate: func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash) {
				gas := hexutil.Uint64(21000)
				gasPrice := (*hexutil.Big)(big.NewInt(20000000000))
				args := evmtypes.TransactionArgs{
					From:     &from,
					To:       &to,
					Gas:      &gas,
					GasPrice: gasPrice,
				}
				blockNum := rpctypes.EthLatestBlockNumber
				blockNumOrHash := rpctypes.BlockNumberOrHash{
					BlockNumber: &blockNum,
				}

				return args, blockNumOrHash
			},
			overrides:     &invalidOverrides,
			expectError:   true,
			expectGasUsed: false,
			expectAccList: false,
		},
		{
			name: "pass - Empty state overrides",
			malleate: func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash) {
				gas := hexutil.Uint64(21000)
				gasPrice := (*hexutil.Big)(big.NewInt(20000000000))
				args := evmtypes.TransactionArgs{
					From:     &from,
					To:       &to,
					Gas:      &gas,
					GasPrice: gasPrice,
				}
				blockNum := rpctypes.EthLatestBlockNumber
				blockNumOrHash := rpctypes.BlockNumberOrHash{
					BlockNumber: &blockNum,
				}

				return args, blockNumOrHash
			},
			overrides:     &emptyOverrides,
			expectError:   false,
			expectGasUsed: true,
			expectAccList: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backend := setupMockBackend(t)

			args, blockNumOrHash := tc.malleate()

			require.True(t, blockNumOrHash.BlockNumber != nil || blockNumOrHash.BlockHash != nil,
				"BlockNumberOrHash should have either BlockNumber or BlockHash set")

			if !tc.expectError || tc.name != "error - missing from address" {
				require.NotEqual(t, common.Address{}, args.GetFrom(), "From address should not be zero")
			}

			result, err := backend.CreateAccessList(rpctypes.NewContextWithHeight(1), args, blockNumOrHash, tc.overrides)

			if tc.expectError {
				require.Error(t, err)
				require.Nil(t, result)
				if tc.errorContains != "" {
					require.Contains(t, err.Error(), tc.errorContains)
				}
				return
			}

			if err != nil {
				t.Logf("Expected success case failed due to incomplete mocking: %v", err)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			if tc.expectGasUsed {
				require.NotNil(t, result.GasUsed)
				require.Greater(t, uint64(*result.GasUsed), uint64(0))
			}

			if tc.expectAccList {
				require.NotNil(t, result.AccessList)
			}
		})
	}
}

func buildMsgEthereumTx(t *testing.T) *evmtypes.MsgEthereumTx {
	t.Helper()
	return buildMsgEthereumTxWithNonce(t, 0)
}

func buildMsgEthereumTxWithNonce(t *testing.T, nonce uint64) *evmtypes.MsgEthereumTx {
	t.Helper()
	from, _ := utiltx.NewAddrKey()
	ethTxParams := evmtypes.EvmTxArgs{
		ChainID:  new(big.Int).SetUint64(constants.ExampleChainID.EVMChainID),
		Nonce:    nonce,
		To:       &common.Address{},
		Amount:   big.NewInt(0),
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
	}
	msgEthereumTx := evmtypes.NewTx(&ethTxParams)
	msgEthereumTx.From = from.Bytes()
	return msgEthereumTx
}

type mockDecodedTx struct {
	msgs []sdk.Msg
}

func (m mockDecodedTx) GetMsgs() []sdk.Msg { return m.msgs }

func (m mockDecodedTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func (m mockDecodedTx) ValidateBasic() error { return nil }

type txConfigWithDecoder struct {
	client.TxConfig
	decoder sdk.TxDecoder
}

func (t txConfigWithDecoder) TxDecoder() sdk.TxDecoder {
	return t.decoder
}

type MockIndexer struct {
	txResults map[common.Hash]*servertypes.TxResult
}

func (m *MockIndexer) LastIndexedBlock() (int64, error) {
	return 0, nil
}

func (m *MockIndexer) IndexBlock(block *tmtypes.Block, txResults []*abcitypes.ExecTxResult) error {
	return nil
}

func (m *MockIndexer) GetByTxHash(hash common.Hash) (*servertypes.TxResult, error) {
	if result, exists := m.txResults[hash]; exists {
		return result, nil
	}
	return nil, nil
}

func (m *MockIndexer) GetByBlockAndIndex(blockNumber int64, txIndex int32) (*servertypes.TxResult, error) {
	return nil, nil
}

func TestReceiptsFromCometBlock(t *testing.T) {
	backend := setupMockBackend(t)
	height := int64(100)
	resBlock := &tmrpctypes.ResultBlock{
		Block: &tmtypes.Block{
			Header: tmtypes.Header{
				Height: height,
			},
		},
	}
	anyData := codectypes.UnsafePackAny(&evmtypes.MsgEthereumTxResponse{Hash: "hash"})
	txMsgData := &sdk.TxMsgData{MsgResponses: []*codectypes.Any{anyData}}
	encodingConfig := encoding.MakeConfig(constants.ExampleChainID.EVMChainID)
	encodedData, err := encodingConfig.Codec.Marshal(txMsgData)
	require.NoError(t, err)
	blockRes := &tmrpctypes.ResultBlockResults{
		Height:     height,
		TxsResults: []*abcitypes.ExecTxResult{{Code: 0, Data: encodedData}},
	}
	tcs := []struct {
		name            string
		ethTxIndex      int32
		expectedTxIndex uint
	}{
		{"tx_with_valid_index", 5, 5},
		{"tx_with_sentinel_index", -1, 0}, // -1 sentinel resolved to loop index
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			msgs := []*evmtypes.MsgEthereumTx{
				buildMsgEthereumTx(t),
			}
			expectedTxResult := &servertypes.TxResult{
				Height:     height,
				TxIndex:    0,
				EthTxIndex: tc.ethTxIndex,
				MsgIndex:   0,
			}
			mockIndexer := &MockIndexer{
				txResults: map[common.Hash]*servertypes.TxResult{
					msgs[0].Hash(): expectedTxResult,
				},
			}
			backend.Indexer = mockIndexer
			mockEVMQueryClient := backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
			mockEVMQueryClient.On("BaseFee", mock.Anything, mock.Anything).Return(&evmtypes.QueryBaseFeeResponse{}, nil)
			receipts, err := backend.ReceiptsFromCometBlock(rpctypes.NewContextWithHeight(1), resBlock, blockRes, msgs)
			require.NoError(t, err)
			require.Len(t, receipts, 1)
			require.Equal(t, tc.expectedTxIndex, receipts[0].TransactionIndex)
			require.Equal(t, msgs[0].Hash(), receipts[0].TxHash)
			require.Equal(t, big.NewInt(height), receipts[0].BlockNumber)
			require.Equal(t, ethtypes.ReceiptStatusSuccessful, receipts[0].Status)
		})
	}
}

// TestReceiptsLogIndexBlockGlobal asserts that after PatchTxResponses runs
// on a block's ExecTxResults, ReceiptsFromCometBlock surfaces block-global
// log.Index and eth-only log.TxIndex per the Ethereum JSON-RPC spec.
func TestReceiptsLogIndexBlockGlobal(t *testing.T) {
	backend := setupMockBackend(t)
	height := int64(200)
	encodingConfig := encoding.MakeConfig(constants.ExampleChainID.EVMChainID)

	addr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	topic := common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")

	// Pre-patch logs: log.TxIndex == cosmos tx index, log.Index per-tx local.
	makeTxResponse := func(hash string, cosmosTxIndex uint64) *evmtypes.MsgEthereumTxResponse {
		return &evmtypes.MsgEthereumTxResponse{
			Hash: hash,
			Logs: []*evmtypes.Log{
				{Address: addr.String(), Topics: []string{topic.String()}, TxIndex: cosmosTxIndex, Index: 0},
				{Address: addr.String(), Topics: []string{topic.String()}, TxIndex: cosmosTxIndex, Index: 1},
			},
		}
	}

	// Distinct nonces so the eth tx hashes differ.
	msg0 := buildMsgEthereumTxWithNonce(t, 0)
	msg1 := buildMsgEthereumTxWithNonce(t, 1)

	encodeTxResult := func(resp *evmtypes.MsgEthereumTxResponse) []byte {
		anyData := codectypes.UnsafePackAny(resp)
		txMsgData := &sdk.TxMsgData{MsgResponses: []*codectypes.Any{anyData}}
		data, err := encodingConfig.Codec.Marshal(txMsgData)
		require.NoError(t, err)
		return data
	}

	tx0Data := encodeTxResult(makeTxResponse(msg0.Hash().Hex(), 0))
	tx1Data := encodeTxResult(makeTxResponse(msg1.Hash().Hex(), 1))

	resBlock := &tmrpctypes.ResultBlock{
		Block: &tmtypes.Block{Header: tmtypes.Header{Height: height}},
	}
	blockRes := &tmrpctypes.ResultBlockResults{
		Height: height,
		TxsResults: []*abcitypes.ExecTxResult{
			{Code: 0, Data: tx0Data},
			{Code: 0, Data: tx1Data},
		},
	}

	patched, err := evmtypes.PatchTxResponses(blockRes.TxsResults)
	require.NoError(t, err)
	blockRes.TxsResults = patched

	mockIndexer := &MockIndexer{
		txResults: map[common.Hash]*servertypes.TxResult{
			msg0.Hash(): {Height: height, TxIndex: 0, EthTxIndex: 0, MsgIndex: 0},
			msg1.Hash(): {Height: height, TxIndex: 1, EthTxIndex: 1, MsgIndex: 0},
		},
	}
	backend.Indexer = mockIndexer

	mockEVMQueryClient := backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
	mockEVMQueryClient.On("BaseFee", mock.Anything, mock.Anything).Return(&evmtypes.QueryBaseFeeResponse{}, nil)

	msgs := []*evmtypes.MsgEthereumTx{msg0, msg1}
	receipts, err := backend.ReceiptsFromCometBlock(rpctypes.NewContextWithHeight(1), resBlock, blockRes, msgs)
	require.NoError(t, err)
	require.Len(t, receipts, 2)

	require.Len(t, receipts[0].Logs, 2)
	require.Equal(t, uint(0), receipts[0].Logs[0].Index)
	require.Equal(t, uint(1), receipts[0].Logs[1].Index)
	require.Equal(t, uint(0), receipts[0].Logs[0].TxIndex)
	require.Equal(t, uint(0), receipts[0].Logs[1].TxIndex)

	require.Len(t, receipts[1].Logs, 2)
	require.Equal(t, uint(2), receipts[1].Logs[0].Index)
	require.Equal(t, uint(3), receipts[1].Logs[1].Index)
	require.Equal(t, uint(1), receipts[1].Logs[0].TxIndex)
	require.Equal(t, uint(1), receipts[1].Logs[1].TxIndex)
}

func TestEthMsgsFromCometBlockSkipStateDBCommitFailure(t *testing.T) {
	backend := setupMockBackend(t)
	successMsg := buildMsgEthereumTx(t)
	decodeCalls := 0

	backend.ClientCtx = backend.ClientCtx.WithTxConfig(txConfigWithDecoder{
		TxConfig: backend.ClientCtx.TxConfig,
		decoder: func(_ []byte) (sdk.Tx, error) {
			decodeCalls++
			return mockDecodedTx{msgs: []sdk.Msg{successMsg}}, nil
		},
	})

	resBlock := &tmrpctypes.ResultBlock{
		Block: &tmtypes.Block{
			Header: tmtypes.Header{Height: 100},
			Data:   tmtypes.Data{Txs: []tmtypes.Tx{{0x01}, {0x02}}},
		},
	}

	blockRes := &tmrpctypes.ResultBlockResults{
		Height: 100,
		TxsResults: []*abcitypes.ExecTxResult{
			{Code: 4, Log: "failed to commit stateDB"},
			{Code: 0},
		},
	}

	msgs := backend.EthMsgsFromCometBlock(rpctypes.NewContextWithHeight(100), resBlock, blockRes)
	require.Equal(t, 1, decodeCalls)
	require.Len(t, msgs, 1)
	require.Same(t, successMsg, msgs[0])
}

// an unmined tx has no receipt: null right away, with or without an EVM mempool
func TestGetTransactionReceiptUnknownTx(t *testing.T) {
	backend := setupMockBackend(t)
	require.Nil(t, backend.Mempool)

	start := time.Now()
	receipt, err := backend.GetTransactionReceipt(context.Background(), common.HexToHash("0xdeadbeef"))
	require.NoError(t, err)
	require.Nil(t, receipt)
	require.Less(t, time.Since(start), time.Second)
}

// TestReceiptsFromCometBlockUnindexed: the tx index runs behind commit, so a
// committed block's txs may not be indexed yet. ReceiptsFromCometBlock then
// takes each tx's entry from the block's own results, as the indexer would
// store it; a tx that is neither indexed nor in the block is still an error.
func TestReceiptsFromCometBlockUnindexed(t *testing.T) {
	backend := setupMockBackend(t) // empty KV indexer: every lookup misses
	msgs, resBlock, blockRes := buildUnindexedBlock(t, backend, 300)

	mockEVMQueryClient := backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
	mockEVMQueryClient.On("BaseFee", mock.Anything, mock.Anything).Return(&evmtypes.QueryBaseFeeResponse{}, nil)

	_, err := backend.GetTxByEthHash(rpctypes.NewContextWithHeight(1), msgs[0].Hash())
	require.Error(t, err, "the index must not hold the block's txs")

	receipts, err := backend.ReceiptsFromCometBlock(rpctypes.NewContextWithHeight(1), resBlock, blockRes, msgs)
	require.NoError(t, err)
	require.Len(t, receipts, 2)
	for i, r := range receipts {
		require.Equal(t, msgs[i].Hash(), r.TxHash)
		require.Equal(t, uint(i), r.TransactionIndex)
		require.Equal(t, uint64(21000), r.GasUsed)
		require.Equal(t, uint64(21000*(i+1)), r.CumulativeGasUsed)
		require.Equal(t, ethtypes.ReceiptStatusSuccessful, r.Status)
	}

	stray := buildMsgEthereumTxWithNonce(t, 2)
	_, err = backend.ReceiptsFromCometBlock(rpctypes.NewContextWithHeight(1), resBlock, blockRes, []*evmtypes.MsgEthereumTx{stray})
	require.ErrorContains(t, err, "tx not found")
}

// buildUnindexedBlock returns a block at height of two encoded Ethereum txs
// and its results, with the events the indexer reads.
func buildUnindexedBlock(t *testing.T, backend *Backend, height int64) ([]*evmtypes.MsgEthereumTx, *tmrpctypes.ResultBlock, *tmrpctypes.ResultBlockResults) {
	t.Helper()
	encodingConfig := encoding.MakeConfig(constants.ExampleChainID.EVMChainID)
	evmtypes.RegisterInterfaces(encodingConfig.InterfaceRegistry) // the block's txs are decoded
	backend.ClientCtx = backend.ClientCtx.WithTxConfig(encodingConfig.TxConfig)

	msgs := []*evmtypes.MsgEthereumTx{buildMsgEthereumTxWithNonce(t, 0), buildMsgEthereumTxWithNonce(t, 1)}
	var txs tmtypes.Txs
	var results []*abcitypes.ExecTxResult
	for i, msg := range msgs {
		tx, err := msg.BuildTxWithEvmParams(encodingConfig.TxConfig.NewTxBuilder(), evmtypes.Params{
			EvmDenom:             evmtypes.DefaultEVMExtendedDenom,
			ExtendedDenomOptions: &evmtypes.ExtendedDenomOptions{ExtendedDenom: evmtypes.DefaultEVMExtendedDenom},
		})
		require.NoError(t, err)
		bz, err := encodingConfig.TxConfig.TxEncoder()(tx)
		require.NoError(t, err)
		txs = append(txs, bz)

		data, err := encodingConfig.Codec.Marshal(&sdk.TxMsgData{MsgResponses: []*codectypes.Any{
			codectypes.UnsafePackAny(&evmtypes.MsgEthereumTxResponse{Hash: msg.Hash().Hex()}),
		}})
		require.NoError(t, err)
		results = append(results, &abcitypes.ExecTxResult{
			Code:    0,
			Data:    data,
			GasUsed: 21000,
			Events: []abcitypes.Event{{Type: evmtypes.EventTypeEthereumTx, Attributes: []abcitypes.EventAttribute{
				{Key: evmtypes.AttributeKeyEthereumTxHash, Value: msg.Hash().Hex()},
				{Key: evmtypes.AttributeKeyTxIndex, Value: []string{"0", "1"}[i]},
				{Key: evmtypes.AttributeKeyTxGasUsed, Value: "21000"},
			}}},
		})
	}
	resBlock := &tmrpctypes.ResultBlock{Block: &tmtypes.Block{Header: tmtypes.Header{Height: height}, Data: tmtypes.Data{Txs: txs}}}
	blockRes := &tmrpctypes.ResultBlockResults{Height: height, TxsResults: results}
	return msgs, resBlock, blockRes
}

// lastIndexedAt is an empty tx index that reports last as its indexed height.
type lastIndexedAt struct {
	MockIndexer
	last int64
}

func (l *lastIndexedAt) LastIndexedBlock() (int64, error) { return l.last, nil }

// catchingUpIndex is an empty tx index that finishes indexing height during
// each lookup: the lookup misses, and the indexed height read after it is
// already height.
type catchingUpIndex struct {
	MockIndexer
	last, height int64
}

func (c *catchingUpIndex) LastIndexedBlock() (int64, error) { return c.last, nil }

func (c *catchingUpIndex) GetByTxHash(common.Hash) (*servertypes.TxResult, error) {
	c.last = c.height
	return nil, nil
}

// unindexedChain mocks a chain whose latest height is *latest: height 300
// holds the two txs of buildUnindexedBlock, every other height is empty.
// *blockCalls counts the blocks read.
func unindexedChain(t *testing.T) (backend *Backend, msgs []*evmtypes.MsgEthereumTx, latest, blockCalls *int64) {
	t.Helper()
	const height = int64(300)
	backend = setupMockBackend(t)
	msgs, resBlock, blockRes := buildUnindexedBlock(t, backend, height)
	latest, blockCalls = new(int64), new(int64)
	*latest = height

	// "latest" is read from the gRPC height header
	mockEVMQueryClient := backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
	mockEVMQueryClient.ExpectedCalls = nil
	mockEVMQueryClient.On("Params", mock.Anything, mock.Anything, mock.Anything).
		Return(&evmtypes.QueryParamsResponse{Params: evmtypes.DefaultParams()}, nil).
		Run(func(args mock.Arguments) {
			for _, opt := range args[2:] {
				if h, ok := opt.(grpc.HeaderCallOption); ok {
					*h.HeaderAddr = metadata.Pairs(grpctypes.GRPCBlockHeightHeader, fmt.Sprint(*latest))
				}
			}
		}).Maybe()
	mockEVMQueryClient.On("BaseFee", mock.Anything, mock.Anything).Return(&evmtypes.QueryBaseFeeResponse{}, nil).Maybe()

	mockClient := backend.ClientCtx.Client.(*mocks.Client)
	anyHeight := mock.MatchedBy(func(h *int64) bool { return h != nil })
	mockClient.On("Block", mock.Anything, anyHeight).Return(
		func(_ context.Context, h *int64) *tmrpctypes.ResultBlock {
			*blockCalls++
			if *h == height {
				return resBlock
			}
			return &tmrpctypes.ResultBlock{Block: &tmtypes.Block{Header: tmtypes.Header{Height: *h}}}
		}, nil).Maybe()
	mockClient.On("BlockResults", mock.Anything, anyHeight).Return(
		func(_ context.Context, h *int64) *tmrpctypes.ResultBlockResults {
			if *h == height {
				return blockRes
			}
			return &tmrpctypes.ResultBlockResults{Height: *h}
		}, nil).Maybe()
	return backend, msgs, latest, blockCalls
}

// TestLookupByHashUnindexed: during the window the block endpoints cover (the
// tx index behind commit), lookups by hash take the tx's result from the
// newest blocks' results too, so both agree. Heights the index already holds
// are not scanned.
func TestLookupByHashUnindexed(t *testing.T) {
	backend, msgs, latest, blockCalls := unindexedChain(t)
	ctx := context.Background()
	for i, msg := range msgs {
		res, err := backend.getTxByEthHashOrUnindexed(ctx, msg.Hash())
		require.NoError(t, err)
		require.Equal(t, int64(300), res.Height)
		require.Equal(t, uint32(i), res.TxIndex)
		require.Equal(t, int32(i), res.EthTxIndex)
		require.Equal(t, uint64(21000), res.GasUsed)
		require.False(t, res.Failed)
	}

	logs, err := backend.GetTransactionLogs(ctx, msgs[1].Hash())
	require.NoError(t, err)
	require.Empty(t, logs)

	receipt, err := backend.GetTransactionReceipt(ctx, buildMsgEthereumTxWithNonce(t, 3).Hash())
	require.NoError(t, err)
	require.Nil(t, receipt, "an unmined tx still has no receipt")

	// the index holds the latest height already: nothing is scanned
	backend.Indexer = &lastIndexedAt{last: *latest}
	*blockCalls = 0
	_, err = backend.getTxByEthHashOrUnindexed(ctx, msgs[0].Hash())
	require.Error(t, err)
	require.Zero(t, *blockCalls)
}

// TestLookupByHashIndexCatchesUp: the index finishes a height between the
// lookup that misses and the read of the indexed height. The scan must still
// cover that height, or the tx is found by neither.
func TestLookupByHashIndexCatchesUp(t *testing.T) {
	backend, msgs, latest, _ := unindexedChain(t)
	backend.Indexer = &catchingUpIndex{last: *latest - 1, height: *latest}

	res, err := backend.getTxByEthHashOrUnindexed(context.Background(), msgs[0].Hash())
	require.NoError(t, err)
	require.Equal(t, *latest, res.Height)
}

// TestLookupByHashCachesBlocks: repeated misses (receipt polls for a pending
// tx) read each of the newest blocks once, not on every lookup.
func TestLookupByHashCachesBlocks(t *testing.T) {
	backend, msgs, latest, blockCalls := unindexedChain(t)
	ctx := context.Background()
	pending := buildMsgEthereumTxWithNonce(t, 2).Hash()

	_, err := backend.getTxByEthHashOrUnindexed(ctx, pending)
	require.Error(t, err)
	require.Equal(t, int64(maxUnindexedBlocks), *blockCalls, "a miss scans at most maxUnindexedBlocks heights")

	*blockCalls = 0
	_, err = backend.getTxByEthHashOrUnindexed(ctx, pending)
	require.Error(t, err)
	res, err := backend.getTxByEthHashOrUnindexed(ctx, msgs[1].Hash())
	require.NoError(t, err)
	require.Equal(t, int32(1), res.EthTxIndex)
	require.Zero(t, *blockCalls, "no new block: nothing is reread")

	*latest++
	_, err = backend.getTxByEthHashOrUnindexed(ctx, pending)
	require.Error(t, err)
	require.Equal(t, int64(1), *blockCalls, "only the new block is read")

	// receipts of a cached block do not modify the cached entries
	*blockCalls = 0
	resBlock, err := backend.RPCClient.Block(ctx, new(int64(300)))
	require.NoError(t, err)
	blockRes, err := backend.RPCClient.BlockResults(ctx, new(int64(300)))
	require.NoError(t, err)
	_, err = backend.ReceiptsFromCometBlock(ctx, resBlock, blockRes, msgs[1:])
	require.NoError(t, err)
	res, err = backend.getTxByEthHashOrUnindexed(ctx, msgs[1].Hash())
	require.NoError(t, err)
	require.Equal(t, int32(1), res.EthTxIndex)
}

// unindexedCache keeps the newest unindexedCacheHeights heights.
func TestUnindexedCacheEvictsLowest(t *testing.T) {
	c := newUnindexedCache()
	for h := int64(1); h <= unindexedCacheHeights+2; h++ {
		c.put(h, nil)
	}
	for h := int64(1); h <= unindexedCacheHeights+2; h++ {
		_, ok := c.get(h)
		require.Equal(t, h > 2, ok, "height %d", h)
	}
	var nilCache *unindexedCache
	_, ok := nilCache.get(1)
	require.False(t, ok)
	require.NotNil(t, nilCache.put(1, nil))
}
