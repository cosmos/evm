package mempool_test

import (
	"crypto/ecdsa"
	"errors"
	"math/big"
	"strconv"
	"sync"
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

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/mocks"
	"github.com/cosmos/evm/mempool/reserver"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/vm/statedb"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	txValue    = 100
	txGasLimit = 50000
)

func TestMempool_Reserver(t *testing.T) {
	storeKey := storetypes.NewKVStoreKey("test")
	transientKey := storetypes.NewTransientStoreKey("transient_test")
	ctx := testutil.DefaultContext(storeKey, transientKey)
	anteHandler := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		return ctx, nil
	}
	mp, _, txConfig, _, _, accounts := setupMempoolWithAnteHandler(t, anteHandler, 3)

	accountKey := accounts[0].key

	// insert eth tx from account0
	ethTx := createMsgEthereumTx(t, txConfig, accountKey, 0, big.NewInt(1e8))
	err := mp.Insert(sdk.Context{}, ethTx)
	require.NoError(t, err)

	// insert cosmos tx from acount0, should error
	cosmosTx := createTestCosmosTx(t, txConfig, accountKey, 0)
	err = mp.Insert(ctx, cosmosTx)
	require.ErrorIs(t, err, reserver.ErrAlreadyReserved)

	// remove the eth tx
	err = mp.Remove(ethTx)
	require.NoError(t, err)

	// pool should be clear
	require.Equal(t, 0, mp.CountTx())

	// should be able to insert the cosmos tx now
	err = mp.Insert(ctx, cosmosTx)
	require.NoError(t, err)

	// should be able to send another tx from the same account to the same pool.
	cosmosTx2 := createTestCosmosTx(t, txConfig, accountKey, 1)
	err = mp.Insert(ctx, cosmosTx2)
	require.NoError(t, err)

	// there should be 2 txs at this point
	require.Equal(t, 2, mp.CountTx())

	// eth tx should now fail.
	err = mp.Insert(sdk.Context{}, ethTx)
	require.ErrorIs(t, err, reserver.ErrAlreadyReserved)
}

func TestMempool_ReserverMultiSigner(t *testing.T) {
	storeKey := storetypes.NewKVStoreKey("test")
	transientKey := storetypes.NewTransientStoreKey("transient_test")
	ctx := testutil.DefaultContext(storeKey, transientKey)
	anteHandler := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		return ctx, nil
	}
	mp, _, txConfig, _, _, accounts := setupMempoolWithAnteHandler(t, anteHandler, 4)

	accountKey := accounts[0].key

	// insert eth tx from account0
	ethTx := createMsgEthereumTx(t, txConfig, accountKey, 0, big.NewInt(1e8))
	err := mp.Insert(sdk.Context{}, ethTx)
	require.NoError(t, err)

	// inserting accounts 1 & 2 should be fine.
	cosmosTx := createTestMultiSignerCosmosTx(t, txConfig, accounts[1].key, accounts[2].key)
	err = mp.Insert(ctx, cosmosTx)
	require.NoError(t, err)

	// submitting account1 key should fail, since it was part of the signer group in the cosmos tx.
	ethTx2 := createMsgEthereumTx(t, txConfig, accounts[1].key, 1, big.NewInt(1e8))
	err = mp.Insert(ctx, ethTx2)
	require.ErrorIs(t, err, reserver.ErrAlreadyReserved)

	// account 0 already has ethTx in pool, should fail.
	comsosTx := createTestMultiSignerCosmosTx(t, txConfig, accounts[3].key, accounts[0].key)
	err = mp.Insert(ctx, comsosTx)
	require.ErrorIs(t, err, reserver.ErrAlreadyReserved)
}

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
	rechecker.SetRecheck(func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		if tx.Nonce() == 1 {
			return sdk.Context{}, errors.New("recheck failed on tx with nonce 1")
		}
		return sdk.Context{}, nil
	})

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
	rechecker.SetRecheck(func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		return sdk.Context{}, nil
	})

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
	rechecker.SetRecheck(func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		return sdk.Context{}, nil
	})

	// insert a tx that will make it into the pending pool and use up the
	// accounts entire balance
	account := accounts[0]
	gasPrice := (account.initialBalance - txValue) / txGasLimit // assuming they divide evenly
	pendingTx := createMsgEthereumTx(t, txConfig, accounts[0].key, 0, new(big.Int).SetUint64(gasPrice))
	require.NoError(t, mp.Insert(sdk.Context{}, pendingTx))

	pending, queued := legacyPool.ContentFrom(account.address)
	require.Len(t, pending, 1)
	require.Len(t, queued, 0)

	// we should write if we are not resetting from promote
	// promoate should write if it is being called out side of the context of a
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
	var expectedNonce uint64
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
	rechecker.SetRecheck(func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		if tx.Nonce() == 0 {
			return sdk.Context{}, errors.New("recheck failed on tx with nonce 0")
		}
		return sdk.Context{}, nil
	})

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
	rechecker.SetRecheck(func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		return sdk.Context{}, nil
	})

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

func TestMempool_InsertMultiMsgCosmosTx(t *testing.T) {
	anteHandler := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		return ctx, nil
	}
	mp, _, txConfig, _, bus, _ := setupMempoolWithAnteHandler(t, anteHandler, 3)

	err := bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
		Header: cmttypes.Header{
			Height:  1,
			Time:    time.Now(),
			ChainID: strconv.Itoa(constants.EighteenDecimalsChainID),
		},
	})
	require.NoError(t, err)

	// create a multimsg cosmos tx
	txBuilder := txConfig.NewTxBuilder()

	fromAddr := sdk.AccAddress([]byte("from"))
	toAddr1 := sdk.AccAddress([]byte("addr1"))
	toAddr2 := sdk.AccAddress([]byte("addr2"))

	msg1 := banktypes.NewMsgSend(
		fromAddr,
		toAddr1,
		sdk.NewCoins(sdk.NewInt64Coin("stake", 1000)),
	)
	msg2 := banktypes.NewMsgSend(
		fromAddr,
		toAddr2,
		sdk.NewCoins(sdk.NewInt64Coin("stake", 2000)),
	)
	err = txBuilder.SetMsgs(msg1, msg2)
	require.NoError(t, err)

	err = txBuilder.SetSignatures(signingtypes.SignatureV2{
		PubKey: secp256k1.GenPrivKey().PubKey(),
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
			Signature: []byte("signature"),
		},
		Sequence: 0,
	})
	require.NoError(t, err)

	multiMsgTx := txBuilder.GetTx()

	require.Len(t, multiMsgTx.GetMsgs(), 2, "transaction should have 2 messages")

	// create a context for the insert operation (must have a multistore on it
	// for ante handler execution, so we have to use the more complicated
	// setup)
	storeKey := storetypes.NewKVStoreKey("test")
	transientKey := storetypes.NewTransientStoreKey("transient_test")
	ctx := testutil.DefaultContext(storeKey, transientKey)

	require.NoError(t, mp.Insert(ctx, multiMsgTx))
	require.Equal(t, 1, mp.CountTx(), "expected a single tx to be in the mempool")

	txs, err := mp.ReapNewValidTxs(0, 0)
	require.NoError(t, err)
	require.Len(t, txs, 1, "expected a single tx to be reaped")
}

func TestMempool_InsertMultiMsgEthereumTx(t *testing.T) {
	anteHandler := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		return ctx, nil
	}
	mp, _, txConfig, _, bus, _ := setupMempoolWithAnteHandler(t, anteHandler, 3)

	err := bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
		Header: cmttypes.Header{
			Height:  1,
			Time:    time.Now(),
			ChainID: strconv.Itoa(constants.EighteenDecimalsChainID),
		},
	})
	require.NoError(t, err)

	txBuilder := txConfig.NewTxBuilder()

	msg1 := banktypes.NewMsgSend(
		sdk.AccAddress([]byte("from")),
		sdk.AccAddress([]byte("addr")),
		sdk.NewCoins(sdk.NewInt64Coin("stake", 2000)),
	)
	msg2 := &vmtypes.MsgEthereumTx{}
	err = txBuilder.SetMsgs(msg1, msg2)
	require.NoError(t, err)

	err = txBuilder.SetSignatures(signingtypes.SignatureV2{
		PubKey: secp256k1.GenPrivKey().PubKey(),
		Data: &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
			Signature: []byte("signature"),
		},
		Sequence: 0,
	})
	require.NoError(t, err)

	multiMsgTx := txBuilder.GetTx()
	require.Len(t, multiMsgTx.GetMsgs(), 2, "transaction should have 2 messages")

	storeKey := storetypes.NewKVStoreKey("test")
	transientKey := storetypes.NewTransientStoreKey("transient_test")
	ctx := testutil.DefaultContext(storeKey, transientKey)

	err = mp.Insert(ctx, multiMsgTx)
	require.ErrorIs(t, err, mempool.ErrMultiMsgEthereumTransaction)
	require.Equal(t, 0, mp.CountTx(), "expected no txs to be in the mempool")

	txs, err := mp.ReapNewValidTxs(0, 0)
	require.NoError(t, err)
	require.Len(t, txs, 0, "expected no txs to be reaped")
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
	return setupMempoolWithAnteHandler(t, nil, numAccounts)
}

func setupMempoolWithAnteHandler(t *testing.T, anteHandler sdk.AnteHandler, numAccounts int) (*mempool.ExperimentalEVMMempool, *mocks.VMKeeper, client.TxConfig, *MockRechecker, *cmttypes.EventBus, []testAccount) {
	t.Helper()

	// Create accounts
	accounts := make([]testAccount, numAccounts)
	for i := range numAccounts {
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

	// Track latest height for the context callback (height=0 means "latest")
	var latestHeight int64 = 1

	// Create context callback
	getCtxCallback := func(height int64, prove bool) (sdk.Context, error) {
		storeKey := storetypes.NewKVStoreKey("test")
		transientKey := storetypes.NewTransientStoreKey("transient_test")
		ctx := testutil.DefaultContext(storeKey, transientKey)
		// height=0 means "latest" (matches SDK's CreateQueryContext behavior)
		if height == 0 {
			height = latestHeight
		}
		return ctx.
			WithBlockTime(time.Now()).
			WithBlockHeader(cmtproto.Header{AppHash: []byte("00000000000000000000000000000000")}).
			WithBlockHeight(height).
			WithChainID(strconv.Itoa(constants.EighteenDecimalsChainID)), nil
	}

	// Create TxConfig using proper encoding config with address codec
	encodingConfig := encoding.MakeConfig(constants.EighteenDecimalsChainID)
	// Register vm types so MsgEthereumTx can be decoded
	vmtypes.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	txConfig := encodingConfig.TxConfig

	// Create client context
	clientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
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
		AnteHandler:      anteHandler,
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
	lock      sync.Mutex
}

func (mr *MockRechecker) SetRecheck(recheck func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error)) {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	mr.RecheckFn = recheck
}

func (mr *MockRechecker) GetContext() (sdk.Context, func()) {
	return sdk.Context{}, func() {}
}

func (mr *MockRechecker) Recheck(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	if mr.RecheckFn != nil {
		return mr.RecheckFn(ctx, tx)
	}
	return sdk.Context{}, nil
}

func (mr *MockRechecker) Update(chain legacypool.BlockChain, header *types.Header) {}

// createTestCosmosTx creates a real Cosmos SDK transaction with the given signer
func createTestCosmosTx(t *testing.T, txConfig client.TxConfig, key *ecdsa.PrivateKey, sequence uint64) sdk.Tx {
	t.Helper()

	pubKeyBytes := crypto.CompressPubkey(&key.PublicKey)
	pubKey := &ethsecp256k1.PubKey{Key: pubKeyBytes}
	addr := pubKey.Address().Bytes()
	addrStr := sdk.MustBech32ifyAddressBytes(constants.ExampleBech32Prefix, addr)

	// Create a simple bank send message
	msg := &banktypes.MsgSend{
		FromAddress: addrStr,
		ToAddress:   addrStr, // send to self
		Amount:      sdk.NewCoins(sdk.NewInt64Coin("aevmos", 1000)),
	}

	txBuilder := txConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msg)
	require.NoError(t, err)

	txBuilder.SetGasLimit(100000)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("aevmos", 1000000)))

	// Set signature with pubkey (unsigned but has signer info)
	sigData := &signingtypes.SingleSignatureData{
		SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
		Signature: nil,
	}
	sig := signingtypes.SignatureV2{
		PubKey:   pubKey,
		Data:     sigData,
		Sequence: sequence,
	}
	err = txBuilder.SetSignatures(sig)
	require.NoError(t, err)

	return txBuilder.GetTx()
}

// createTestMultiSignerCosmosTx creates a Cosmos SDK transaction with multiple signers.
// Each key produces one MsgSend from that signer.
func createTestMultiSignerCosmosTx(t *testing.T, txConfig client.TxConfig, keys ...*ecdsa.PrivateKey) sdk.Tx {
	t.Helper()
	require.NotEmpty(t, keys, "must provide at least one key")

	var msgs []sdk.Msg
	var sigs []signingtypes.SignatureV2

	for i, key := range keys {
		pubKeyBytes := crypto.CompressPubkey(&key.PublicKey)
		pubKey := &ethsecp256k1.PubKey{Key: pubKeyBytes}
		addr := pubKey.Address().Bytes()
		addrStr := sdk.MustBech32ifyAddressBytes(constants.ExampleBech32Prefix, addr)

		// Each signer has their own MsgSend
		msg := &banktypes.MsgSend{
			FromAddress: addrStr,
			ToAddress:   addrStr, // send to self
			Amount:      sdk.NewCoins(sdk.NewInt64Coin("aevmos", 1000)),
		}
		msgs = append(msgs, msg)

		// Create signature info for this signer
		sigData := &signingtypes.SingleSignatureData{
			SignMode:  signingtypes.SignMode_SIGN_MODE_DIRECT,
			Signature: nil,
		}
		sig := signingtypes.SignatureV2{
			PubKey:   pubKey,
			Data:     sigData,
			Sequence: uint64(i), //nolint:gosec // its fine.
		}
		sigs = append(sigs, sig)
	}

	txBuilder := txConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgs...)
	require.NoError(t, err)

	txBuilder.SetGasLimit(100000 * uint64(len(keys)))
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewInt64Coin("aevmos", 1000000)))

	err = txBuilder.SetSignatures(sigs...)
	require.NoError(t, err)

	return txBuilder.GetTx()
}
