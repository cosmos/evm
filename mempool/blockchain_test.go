package mempool_test

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/mocks"
	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/vm/statedb"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// createMockContext creates a basic mock context for testing with multistore
func createMockContext() sdk.Context {
	// crate ctx with multistore so that CacheContext() can be used
	storeKey := storetypes.NewKVStoreKey("test")
	transientKey := storetypes.NewTransientStoreKey("transient_test")
	ctx := testutil.DefaultContext(storeKey, transientKey)

	return ctx.
		WithBlockTime(time.Now()).
		WithBlockHeader(cmtproto.Header{AppHash: []byte("00000000000000000000000000000000")}).
		WithBlockHeight(1)
}

// TestBlockchainRaceCondition tests concurrent access to NotifyNewBlock and StateAt
// to ensure there are no race conditions between these operations.
func TestBlockchainRaceCondition(t *testing.T) {
	logger := log.NewNopLogger()

	// Create mock keepers using generated mocks
	mockVMKeeper := mocks.NewVMKeeper(t)
	mockFeeMarketKeeper := mocks.NewFeeMarketKeeper(t)

	ethCfg := vmtypes.DefaultChainConfig(constants.EighteenDecimalsChainID)
	if err := vmtypes.SetChainConfig(ethCfg); err != nil {
		panic(err)
	}

	// Set up mock expectations for methods that will be called
	mockVMKeeper.On("GetBaseFee", mock.Anything).Return(big.NewInt(1000000000)).Maybe()         // 1 gwei
	mockFeeMarketKeeper.On("GetBlockGasWanted", mock.Anything).Return(uint64(10000000)).Maybe() // 10M gas
	mockVMKeeper.On("GetParams", mock.Anything).Return(vmtypes.DefaultParams()).Maybe()
	mockVMKeeper.On("GetAccount", mock.Anything, common.Address{}).Return(&statedb.Account{}).Maybe()
	mockVMKeeper.On("GetState", mock.Anything, common.Address{}, common.Hash{}).Return(common.Hash{}).Maybe()
	mockVMKeeper.On("GetCode", mock.Anything, common.Hash{}).Return([]byte{}).Maybe()
	mockVMKeeper.On("GetCodeHash", mock.Anything, common.Address{}).Return(common.Hash{}).Maybe()
	mockVMKeeper.On("ForEachStorage", mock.Anything, common.Address{}, mock.AnythingOfType("func(common.Hash, common.Hash) bool")).Maybe()
	mockVMKeeper.On("KVStoreKeys").Return(make(map[string]*storetypes.KVStoreKey)).Maybe()

	err := vmtypes.NewEVMConfigurator().WithEVMCoinInfo(constants.ChainsCoinInfo[constants.EighteenDecimalsChainID]).Configure()
	require.NoError(t, err)

	// Mock context callback that returns a valid context
	getCtxCallback := func(height int64, prove bool) (sdk.Context, error) {
		return createMockContext(), nil
	}

	blockchain := mempool.NewBlockchain(
		getCtxCallback,
		logger,
		mockVMKeeper,
		mockFeeMarketKeeper,
		21000000, // block gas limit
	)

	const numIterations = 100
	var wg sync.WaitGroup

	// Channel to collect any errors from goroutines
	errChan := make(chan error, numIterations*2)

	// Start goroutine that calls NotifyNewBlock repeatedly
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numIterations; i++ {
			blockchain.NotifyNewBlock()
			// Small delay to allow interleaving
			time.Sleep(time.Microsecond)
		}
	}()

	// Start goroutine that calls StateAt repeatedly
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numIterations; i++ {
			hash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
			_, err := blockchain.StateAt(hash)
			if err != nil {
				errChan <- err
				return
			}
			// Small delay to allow interleaving
			time.Sleep(time.Microsecond)
		}
	}()

	// Wait for both goroutines to complete
	wg.Wait()
	close(errChan)

	// Check for any errors
	for err := range errChan {
		require.NoError(t, err)
	}

	// Basic validation - ensure blockchain still functions correctly after concurrent access
	header := blockchain.CurrentBlock()
	require.NotNil(t, header)
	require.Equal(t, int64(1), header.Number.Int64())

	// Ensure StateAt still works after concurrent access
	hash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	stateDB, err := blockchain.StateAt(hash)
	require.NoError(t, err)
	require.NotNil(t, stateDB)
}
