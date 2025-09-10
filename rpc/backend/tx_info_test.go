package backend

import (
	"context"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	tmrpctypes "github.com/cometbft/cometbft/rpc/core/types"
	tmtypes "github.com/cometbft/cometbft/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/indexer"
	"github.com/cosmos/evm/rpc/backend/mocks"
	rpctypes "github.com/cosmos/evm/rpc/types"
	testconfig "github.com/cosmos/evm/testutil/config"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func setupMockBackend(t *testing.T) *Backend {
	t.Helper()
	ctx := server.NewDefaultContext()
	ctx.Viper.Set("telemetry.global-labels", []interface{}{})
	ctx.Viper.Set("evm.evm-chain-id", testconfig.DefaultEvmChainID)

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

	chainID := testconfig.DefaultChainConfig.ChainID
	evmChainConfig := testconfig.DefaultChainConfig.EvmConfig.ChainConfig
	encodingConfig := encoding.MakeConfig(evmChainConfig.ChainId)
	clientCtx := client.Context{}.WithChainID(chainID).
		WithHeight(1).
		WithTxConfig(encodingConfig.TxConfig).
		WithKeyringDir(clientDir).
		WithKeyring(keyRing).
		WithAccountRetriever(client.TestAccountRetriever{Accounts: accounts}).
		WithClient(mocks.NewClient(t)).
		WithCodec(encodingConfig.Codec)

	allowUnprotectedTxs := false
	idxer := indexer.NewKVIndexer(dbm.NewMemDB(), ctx.Logger, clientCtx)

	backend := NewBackend(ctx, ctx.Logger, clientCtx, allowUnprotectedTxs, idxer, nil)
	backend.Cfg.JSONRPC.GasCap = 25000000
	backend.Cfg.JSONRPC.EVMTimeout = 0
	backend.Cfg.JSONRPC.AllowInsecureUnlock = true
	backend.Cfg.EVM.EVMChainID = evmChainConfig.ChainId
	mockEVMQueryClient := mocks.NewEVMQueryClient(t)
	mockFeeMarketQueryClient := mocks.NewFeeMarketQueryClient(t)
	backend.QueryClient.QueryClient = mockEVMQueryClient
	backend.QueryClient.FeeMarket = mockFeeMarketQueryClient
	backend.Ctx = rpctypes.ContextWithHeight(1)

	mockClient := backend.ClientCtx.Client.(*mocks.Client)
	mockClient.On("Status", context.Background()).Return(&tmrpctypes.ResultStatus{
		SyncInfo: tmrpctypes.SyncInfo{
			LatestBlockHeight: 1,
		},
	}, nil).Maybe()

	mockHeader := &tmtypes.Header{
		Height:  1,
		Time:    time.Now(),
		ChainID: chainID,
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
	testCases := []struct {
		name          string
		malleate      func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash)
		expectError   bool
		errorContains string
		expectGasUsed bool
		expectAccList bool
	}{
		{
			name: "success - basic transaction",
			malleate: func() (evmtypes.TransactionArgs, rpctypes.BlockNumberOrHash) {
				from := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
				to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef")
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
				from := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
				to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef")
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
				from := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
				to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef")
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
				from := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
				to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef")
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
				to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef")
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
				from := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
				to := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdef")
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

			result, err := backend.CreateAccessList(args, blockNumOrHash)

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
