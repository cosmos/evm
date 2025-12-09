package mempool_test

import (
	"crypto/ecdsa"
	"errors"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/mocks"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/vm/statedb"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txmodule "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
)

const (
	txValue    = 100
	txGasLimit = 50000
)

// Ensures txs are not reaped multiple times when promoting and demoting the
// same tx
func TestMempool_ReapPromoteDemotePromote(t *testing.T) {
	mp, _, txConfig, rechecker, bus, accounts := setupMempoolWithAccounts(t, 3)

	err := bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
		Header: cmttypes.Header{
			Height:  1,
			Time:    time.Now(),
			ChainID: strconv.Itoa(constants.EighteenDecimalsChainID),
		},
	})
	require.NoError(t, err)

	// for a reset to happen for block 1 and wait for it
	require.NoError(t, mp.GetTxPool().Sync())

	// Account 0: Insert 3 sequential transactions (nonce 0, 1, 2) - should all go to pending
	for nonce := uint64(0); nonce < 3; nonce++ {
		tx := createMsgEthereumTx(t, txConfig, accounts[0].key, nonce, big.NewInt(1e8))
		err := mp.Insert(sdk.Context{}, tx)
		require.NoError(t, err, "failed to insert pending tx for account 0, nonce %d", nonce)
	}

	// wait for another reset to make sure the pool processes the above txns into pending
	require.NoError(t, mp.GetTxPool().Sync())
	require.Equal(t, 3, mp.CountTx())

	// reap txs now and we should get back all txs since they were all validated
	txs, err := mp.ReapNewValidTxs(0, 0)
	require.NoError(t, err)
	require.Len(t, txs, 3)

	// setup tx with nonce 1 to fail recheck. it will get kicked out of the
	// pool and tx with nonce 2 will be demoted to queued (when tx 1 is
	// resubmitted, it will be returned from reap again).
	rechecker.RecheckFn = func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		if tx.Nonce() == 1 {
			return sdk.Context{}, errors.New("recheck failed on tx with nonce 1")
		}
		return sdk.Context{}, nil
	}

	// sync the pool to make sure the above happens
	require.NoError(t, mp.GetTxPool().Sync())
	legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
	pending, queued := legacyPool.ContentFrom(accounts[0].address)
	require.Len(t, pending, 1)
	require.Len(t, queued, 1)

	// reap should now return no txs, since no new txs have been validated
	txs, err = mp.ReapNewValidTxs(0, 0)
	require.NoError(t, err)
	require.Len(t, txs, 0)

	// setup recheck to not fail any txs again, tx 2 will not fail this but
	// it wont be promoted since it is nonce gapped from tx 1
	rechecker.RecheckFn = func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		return sdk.Context{}, nil
	}

	// sync the pool to make sure the above happens
	require.NoError(t, mp.GetTxPool().Sync())
	pending, queued = legacyPool.ContentFrom(accounts[0].address)
	require.Len(t, pending, 1)
	require.Len(t, queued, 1)

	// reap should still not return any new valid txs, since even though tx
	// with nonce 2 was validated again (but not promoted), we have already
	// returned it from reap
	txs, err = mp.ReapNewValidTxs(0, 0)
	require.NoError(t, err)
	require.Len(t, txs, 0)

	// re submit tx 1 to the mempool to fill the nonce gap, since this is
	// now a new valid txn, it should be returned by reap again
	tx := createMsgEthereumTx(t, txConfig, accounts[0].key, 1, big.NewInt(1e8))
	err = mp.Insert(sdk.Context{}, tx)
	require.NoError(t, err, "failed to insert pending tx for account 0, nonce %d", 1)

	// sync the pool tx 1 and 2 should now be promoted to pending
	require.NoError(t, mp.GetTxPool().Sync())
	pending, queued = legacyPool.ContentFrom(accounts[0].address)
	require.Len(t, pending, 3)
	require.Len(t, queued, 0)

	// finally ensure reap still is not returning these txs since they have
	// already been reaped, even though they were newly validated
	txs, err = mp.ReapNewValidTxs(0, 0)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Equal(t, uint64(1), getTxNonce(t, txConfig, txs[0]))
}

func TestMempool_QueueInvalidWhenUsingPendingState(t *testing.T) {
	mp, _, txConfig, rechecker, bus, accounts := setupMempoolWithAccounts(t, 3)
	err := bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
		Header: cmttypes.Header{
			Height:  1,
			Time:    time.Now(),
			ChainID: strconv.Itoa(constants.EighteenDecimalsChainID),
		},
	})
	require.NoError(t, err)

	// for a reset to happen for block 1 and wait for it
	require.NoError(t, mp.GetTxPool().Sync())

	legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
	rechecker.RecheckFn = func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		return sdk.Context{}, nil
	}

	// insert a tx that will make it into the pending pool and use up the
	// accounts entire balance
	account := accounts[0]
	gasPrice := (account.initialBalance - txValue) / txGasLimit // assuming they divide evenly
	pendingTx := createMsgEthereumTx(t, txConfig, accounts[0].key, 0, new(big.Int).SetUint64(gasPrice))
	require.NoError(t, mp.Insert(sdk.Context{}, pendingTx))

	pending, queued := legacyPool.ContentFrom(account.address)
	require.Len(t, pending, 1)
	require.Len(t, queued, 0)

	// we shoudl write if we are not resetting from promote
	// promoate shoudl write if it is being called out side of the context of a
	// new block (reset) but if it is in the context of a new blcok and we know
	// we are about to run demote executables again on it, then we should not
	// write

	// insert a tx that will be placed in queued due to a nonce gap. the above
	// tx is using the entire balance though so this tx is not technically
	// valid taking into account the contents of the pending pool. we need to
	// ensure this tx does not make it into the pending pool, because it could
	// then be selected for a proposal if a new block does not come in an cause
	// it to be rechecked again and dropped.

	queuedTx := createMsgEthereumTx(t, txConfig, accounts[0].key, 2, new(big.Int).SetUint64(100))
	require.Error(t, mp.Insert(sdk.Context{}, queuedTx))

	pending, queued = legacyPool.ContentFrom(account.address)
	require.Len(t, pending, 1)
	var expectedNonce uint64 = 0
	require.Equal(t, expectedNonce, pending[0].Nonce())
	require.Len(t, queued, 0)
}

func TestMempool_ReapPromoteDemoteReap(t *testing.T) {
	mp, _, txConfig, rechecker, bus, accounts := setupMempoolWithAccounts(t, 3)
	err := bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
		Header: cmttypes.Header{
			Height:  1,
			Time:    time.Now(),
			ChainID: strconv.Itoa(constants.EighteenDecimalsChainID),
		},
	})
	require.NoError(t, err)

	// for a reset to happen for block 1 and wait for it
	require.NoError(t, mp.GetTxPool().Sync())

	// insert a single tx for an account at nonce 0
	tx := createMsgEthereumTx(t, txConfig, accounts[0].key, 0, big.NewInt(1e8))
	require.NoError(t, mp.Insert(sdk.Context{}, tx))

	// wait for another reset to make sure the pool processes the above
	// txn into pending
	require.NoError(t, mp.GetTxPool().Sync())
	require.Equal(t, 1, mp.CountTx())

	// setup tx with nonce 0 to fail recheck.
	legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
	rechecker.RecheckFn = func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		if tx.Nonce() == 0 {
			return sdk.Context{}, errors.New("recheck failed on tx with nonce 0")
		}
		return sdk.Context{}, nil
	}

	// sync the pool to make sure the above happens
	require.NoError(t, mp.GetTxPool().Sync())
	pending, queued := legacyPool.ContentFrom(accounts[0].address)
	require.Len(t, pending, 0)
	require.Len(t, queued, 0)

	// reap should now return no txs, since even though a new tx was
	// validated since the last reap call, it was then invalidated and
	// dropped before this reap call
	txs, err := mp.ReapNewValidTxs(0, 0)
	require.NoError(t, err)
	require.Len(t, txs, 0)

	// recheck will pass for all txns again
	rechecker.RecheckFn = func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		return sdk.Context{}, nil
	}

	// insert the same tx again and make sure the tx can still be returned
	// from the next call to reap
	tx = createMsgEthereumTx(t, txConfig, accounts[0].key, 0, big.NewInt(1e8))
	require.NoError(t, mp.Insert(sdk.Context{}, tx))

	// sync the pool to make sure its promoted to pending
	require.NoError(t, mp.GetTxPool().Sync())
	pending, queued = legacyPool.ContentFrom(accounts[0].address)
	require.Len(t, pending, 1)
	require.Len(t, queued, 0)

	// reap should now return our tx again
	txs, err = mp.ReapNewValidTxs(0, 0)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	require.Equal(t, uint64(0), getTxNonce(t, txConfig, txs[0]))
}

func TestMempool_ReapNewBlock(t *testing.T) {
	mp, vmKeeper, txConfig, _, bus, accounts := setupMempoolWithAccounts(t, 3)
	err := bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
		Header: cmttypes.Header{
			Height:  1,
			Time:    time.Now(),
			ChainID: strconv.Itoa(constants.EighteenDecimalsChainID),
		},
	})
	require.NoError(t, err)

	// for a reset to happen for block 1 and wait for it
	require.NoError(t, mp.GetTxPool().Sync())

	tx0 := createMsgEthereumTx(t, txConfig, accounts[0].key, 0, big.NewInt(1e8))
	require.NoError(t, mp.Insert(sdk.Context{}, tx0))
	tx1 := createMsgEthereumTx(t, txConfig, accounts[0].key, 1, big.NewInt(1e8))
	require.NoError(t, mp.Insert(sdk.Context{}, tx1))
	tx2 := createMsgEthereumTx(t, txConfig, accounts[0].key, 2, big.NewInt(1e8))
	require.NoError(t, mp.Insert(sdk.Context{}, tx2))

	// wait for another reset to make sure the pool processes the above
	// txns into pending
	require.NoError(t, mp.GetTxPool().Sync())
	require.Equal(t, 3, mp.CountTx())

	// simulate comet calling removeTx, a new height being published, and
	// our accounts nonce increments to 1, so tx 0 will be invalidated
	// after the next reset
	vmKeeper.On("GetAccount", mock.Anything, accounts[0].address).Unset()
	vmKeeper.On("GetAccount", mock.Anything, accounts[0].address).Return(&statedb.Account{
		Nonce:   1,
		Balance: uint256.NewInt(1e18),
	})
	err = bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
		Header: cmttypes.Header{
			Height:  2,
			Time:    time.Now(),
			ChainID: strconv.Itoa(constants.EighteenDecimalsChainID),
		},
	})
	require.NoError(t, err)

	// sync the pool to make sure the above happens, tx0 should be dropped
	// from the pool and the reap list
	require.NoError(t, mp.GetTxPool().Sync())

	legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
	pending, queued := legacyPool.ContentFrom(accounts[0].address)
	require.Len(t, pending, 2)
	require.Len(t, queued, 0)

	// reap should return txs 1 and 2
	txs, err := mp.ReapNewValidTxs(0, 0)
	require.NoError(t, err)
	require.Len(t, txs, 2)
	require.GreaterOrEqual(t, getTxNonce(t, txConfig, txs[0]), uint64(1)) // 1 or 2
	require.GreaterOrEqual(t, getTxNonce(t, txConfig, txs[1]), uint64(1)) // 1 or 2
}

// Helper types and functions

type testAccount struct {
	key            *ecdsa.PrivateKey
	address        common.Address
	nonce          uint64
	initialBalance uint64
}

func setupMempoolWithAccounts(t *testing.T, numAccounts int) (*mempool.ExperimentalEVMMempool, *mocks.VMKeeper, client.TxConfig, *MockRechecker, *cmttypes.EventBus, []testAccount) {
	t.Helper()

	// Create accounts
	accounts := make([]testAccount, numAccounts)
	for i := 0; i < numAccounts; i++ {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		accounts[i] = testAccount{
			key:            key,
			address:        crypto.PubkeyToAddress(key.PublicKey),
			nonce:          0,
			initialBalance: 100000000000100,
		}
	}

	// Setup EVM chain config
	vmtypes.NewEVMConfigurator().ResetTestConfig()
	ethCfg := vmtypes.DefaultChainConfig(constants.EighteenDecimalsChainID)
	require.NoError(t, vmtypes.SetChainConfig(ethCfg))

	err := vmtypes.NewEVMConfigurator().
		WithEVMCoinInfo(constants.ChainsCoinInfo[constants.EighteenDecimalsChainID]).
		Configure()
	require.NoError(t, err)

	// Create mocks
	mockVMKeeper := mocks.NewVMKeeper(t)
	mockFeeMarketKeeper := mocks.NewFeeMarketKeeper(t)
	mockRechecker := &MockRechecker{}

	// Setup mock expectations
	mockVMKeeper.On("GetBaseFee", mock.Anything).Return(big.NewInt(1e9)).Maybe()
	mockVMKeeper.On("GetParams", mock.Anything).Return(vmtypes.DefaultParams()).Maybe()
	mockFeeMarketKeeper.On("GetBlockGasWanted", mock.Anything).Return(uint64(10000000)).Maybe()
	mockVMKeeper.On("GetEvmCoinInfo", mock.Anything).Return(constants.ChainsCoinInfo[constants.EighteenDecimalsChainID]).Maybe()

	// Setup account mocks for all test accounts
	for _, acc := range accounts {
		mockVMKeeper.On("GetAccount", mock.Anything, acc.address).Return(&statedb.Account{
			Nonce:   acc.nonce,
			Balance: uint256.NewInt(acc.initialBalance),
		}).Maybe()
		mockVMKeeper.On("GetNonce", acc.address).Return(acc.nonce).Maybe()
		mockVMKeeper.On("GetBalance", acc.address).Return(uint256.NewInt(1e18)).Maybe() // 1 ETH
		mockVMKeeper.On("GetCodeHash", acc.address).Return(common.Hash{}).Maybe()
	}

	mockVMKeeper.On("GetState", mock.Anything, mock.Anything).Return(common.Hash{}).Maybe()
	mockVMKeeper.On("GetCode", mock.Anything, mock.Anything).Return([]byte{}).Maybe()
	mockVMKeeper.On("ForEachStorage", mock.Anything, mock.Anything, mock.Anything).Maybe()
	mockVMKeeper.On("KVStoreKeys").Return(make(map[string]*storetypes.KVStoreKey)).Maybe()

	mockVMKeeper.On("SetEvmMempool", mock.Anything).Maybe()

	// Create context callback
	getCtxCallback := func(height int64, prove bool) (sdk.Context, error) {
		storeKey := storetypes.NewKVStoreKey("test")
		transientKey := storetypes.NewTransientStoreKey("transient_test")
		ctx := testutil.DefaultContext(storeKey, transientKey)
		return ctx.
			WithBlockTime(time.Now()).
			WithBlockHeader(cmtproto.Header{AppHash: []byte("00000000000000000000000000000000")}).
			WithBlockHeight(height).
			WithChainID(strconv.Itoa(constants.EighteenDecimalsChainID)), nil
	}

	// Create TxConfig
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	vmtypes.RegisterInterfaces(interfaceRegistry)
	txmodule.RegisterInterfaces(interfaceRegistry)
	protoCodec := codec.NewProtoCodec(interfaceRegistry)
	txConfig := tx.NewTxConfig(protoCodec, tx.DefaultSignModes)

	// Create client context
	clientCtx := client.Context{}.
		WithCodec(protoCodec).
		WithInterfaceRegistry(interfaceRegistry).
		WithTxConfig(txConfig)

	// Create mempool config
	legacyConfig := legacypool.DefaultConfig
	legacyConfig.Journal = "" // Disable journal for tests
	legacyConfig.PriceLimit = 1
	legacyConfig.PriceBump = 10 // 10% price bump for replacement

	config := &mempool.EVMMempoolConfig{
		LegacyPoolConfig: &legacyConfig,
		BlockGasLimit:    30000000,
		MinTip:           uint256.NewInt(0),
		AnteHandler:      nil, // No ante handler for this test
	}

	// Create mempool
	mp := mempool.NewExperimentalEVMMempool(
		getCtxCallback,
		log.NewNopLogger(),
		mockVMKeeper,
		mockFeeMarketKeeper,
		txConfig,
		clientCtx,
		mempool.NewTxEncoder(txConfig),
		mockRechecker,
		config,
		1000, // cosmos pool max tx
	)
	require.NotNil(t, mp)

	eventBus := cmttypes.NewEventBus()
	require.NoError(t, eventBus.Start())
	mp.SetEventBus(eventBus)

	return mp, mockVMKeeper, txConfig, mockRechecker, eventBus, accounts
}

func createMsgEthereumTx(
	t *testing.T,
	txConfig client.TxConfig,
	key *ecdsa.PrivateKey,
	nonce uint64,
	gasPrice *big.Int,
) sdk.Tx {
	t.Helper()

	tx := types.NewTransaction(
		nonce,
		common.Address{0x01}, // Send to a dummy address
		big.NewInt(txValue),
		txGasLimit,
		gasPrice,
		nil,
	)

	chainID := vmtypes.GetChainConfig().ChainId
	signer := types.LatestSignerForChainID(new(big.Int).SetUint64(chainID))
	signedTx, err := types.SignTx(tx, signer, key)
	require.NoError(t, err)

	return wrapInCosmosSDKTx(t, txConfig, signedTx)
}

func createMsgEthereumTxWithValue(
	t *testing.T,
	txConfig client.TxConfig,
	key *ecdsa.PrivateKey,
	value uint64,
	nonce uint64,
	gasPrice *big.Int,
) sdk.Tx {
	t.Helper()

	tx := types.NewTransaction(
		nonce,
		common.Address{0x01}, // Send to a dummy address
		new(big.Int).SetUint64(value),
		txGasLimit,
		gasPrice,
		nil,
	)

	chainID := vmtypes.GetChainConfig().ChainId
	signer := types.LatestSignerForChainID(new(big.Int).SetUint64(chainID))
	signedTx, err := types.SignTx(tx, signer, key)
	require.NoError(t, err)

	return wrapInCosmosSDKTx(t, txConfig, signedTx)
}

func wrapInCosmosSDKTx(t *testing.T, txConfig client.TxConfig, ethTx *types.Transaction) sdk.Tx {
	t.Helper()

	msg := &vmtypes.MsgEthereumTx{}
	msg.FromEthereumTx(ethTx)

	txBuilder := txConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msg)
	require.NoError(t, err)

	return txBuilder.GetTx()
}

// decodeTxBytes decodes transaction bytes returned from ReapNewValidTxs back into an Ethereum transaction
func decodeTxBytes(t *testing.T, txConfig client.TxConfig, txBytes []byte) *types.Transaction {
	t.Helper()

	// Decode cosmos SDK tx
	cosmosTx, err := txConfig.TxDecoder()(txBytes)
	require.NoError(t, err, "failed to decode tx bytes")

	// Extract MsgEthereumTx
	msgs := cosmosTx.GetMsgs()
	require.Len(t, msgs, 1, "expected exactly one message in tx")

	ethMsg, ok := msgs[0].(*vmtypes.MsgEthereumTx)
	require.True(t, ok, "expected message to be MsgEthereumTx")

	// Convert to Ethereum transaction
	ethTx := ethMsg.AsTransaction()
	require.NotNil(t, ethTx, "ethereum transaction should not be nil")

	return ethTx
}

// getTxNonce extracts the nonce from transaction bytes
func getTxNonce(t *testing.T, txConfig client.TxConfig, txBytes []byte) uint64 {
	t.Helper()

	ethTx := decodeTxBytes(t, txConfig, txBytes)
	return ethTx.Nonce()
}

type MockRechecker struct {
	RecheckFn func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error)
}

func (mr *MockRechecker) GetContext() (sdk.Context, func()) {
	return sdk.Context{}, func() {}
}

func (mr *MockRechecker) Recheck(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
	if mr.RecheckFn != nil {
		return mr.RecheckFn(ctx, tx)
	}
	return sdk.Context{}, nil
}

func (mr *MockRechecker) Update(chain legacypool.BlockChain, header *types.Header) {}
