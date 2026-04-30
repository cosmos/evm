package mempool_test

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
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
	evmtestutiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/vm/statedb"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	mempooltypes "github.com/cosmos/cosmos-sdk/types/mempool"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func TestMempool_Reserver(t *testing.T) {
	mp, s := setupMempoolWithAccounts(t, 3)
	txConfig, accounts := s.txConfig, s.accounts

	accountKey := accounts[0].key

	// insert eth tx from account0
	ethTx := createMsgEthereumTx(t, txConfig, accountKey, 0, big.NewInt(1e8))
	err := mp.Insert(context.Background(), ethTx)
	require.NoError(t, err)

	// insert cosmos tx from acount0, should error
	cosmosTx := createTestCosmosTx(t, txConfig, accountKey, 0)
	err = mp.Insert(context.Background(), cosmosTx)
	require.ErrorIs(t, err, reserver.ErrAlreadyReserved)

	// remove the eth tx
	err = mp.RemoveWithReason(context.Background(), ethTx, mempooltypes.RemoveReason{Error: errors.New("some error")})
	require.NoError(t, err)

	// pool should be clear
	require.Equal(t, 0, mp.CountTx())

	// should be able to insert the cosmos tx now
	err = mp.Insert(context.Background(), cosmosTx)
	require.NoError(t, err)

	// should be able to send another tx from the same account to the same pool.
	cosmosTx2 := createTestCosmosTx(t, txConfig, accountKey, 1)
	err = mp.Insert(context.Background(), cosmosTx2)
	require.NoError(t, err)

	// there should be 2 txs at this point
	require.Equal(t, 2, mp.CountTx())

	// eth tx should now fail.
	err = mp.Insert(context.Background(), ethTx)
	require.ErrorIs(t, err, reserver.ErrAlreadyReserved)
}

func TestMempool_ReserverMultiSigner(t *testing.T) {
	mp, s := setupMempoolWithAccounts(t, 4)
	txConfig, accounts := s.txConfig, s.accounts

	accountKey := accounts[0].key

	// insert eth tx from account0
	ethTx := createMsgEthereumTx(t, txConfig, accountKey, 0, big.NewInt(1e8))
	err := mp.Insert(context.Background(), ethTx)
	require.NoError(t, err)

	// inserting accounts 1 & 2 should be fine.
	cosmosTx := createTestMultiSignerCosmosTx(t, txConfig, accounts[1].key, accounts[2].key)
	err = mp.Insert(context.Background(), cosmosTx)
	require.NoError(t, err)

	// submitting account1 key should fail, since it was part of the signer group in the cosmos tx.
	ethTx2 := createMsgEthereumTx(t, txConfig, accounts[1].key, 1, big.NewInt(1e8))
	err = mp.Insert(context.Background(), ethTx2)
	require.ErrorIs(t, err, reserver.ErrAlreadyReserved)

	// account 0 already has ethTx in pool, should fail.
	comsosTx := createTestMultiSignerCosmosTx(t, txConfig, accounts[3].key, accounts[0].key)
	err = mp.Insert(context.Background(), comsosTx)
	require.ErrorIs(t, err, reserver.ErrAlreadyReserved)
}

// Ensures txs are not reaped multiple times when promoting and demoting the
// same tx
func TestMempool_ReapPromoteDemotePromote(t *testing.T) {
	mp, s := setupMempoolWithAccounts(t, 3)
	txConfig, rechecker, bus, accounts := s.txConfig, s.evmRechecker, s.eventBus, s.accounts

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
		err := mp.Insert(sdk.Context{}.WithContext(context.Background()), tx)
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
	rechecker.SetEVMRecheck(func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
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
	rechecker.SetEVMRecheck(func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
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
	err = mp.Insert(sdk.Context{}.WithContext(context.Background()), tx)
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
	mp, s := setupMempoolWithAccounts(t, 3)
	txConfig, rechecker, bus, accounts := s.txConfig, s.evmRechecker, s.eventBus, s.accounts
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
	rechecker.SetEVMRecheck(func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		return sdk.Context{}, nil
	})

	// insert a tx that will make it into the pending pool and use up the
	// accounts entire balance
	account := accounts[0]
	gasPrice := (account.initialBalance - txValue) / txGasLimit // assuming they divide evenly
	pendingTx := createMsgEthereumTx(t, txConfig, accounts[0].key, 0, new(big.Int).SetUint64(gasPrice))
	require.NoError(t, mp.Insert(sdk.Context{}.WithContext(context.Background()), pendingTx))
	require.NoError(t, mp.GetTxPool().Sync())

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
	require.Error(t, mp.Insert(sdk.Context{}.WithContext(context.Background()), queuedTx))

	pending, queued = legacyPool.ContentFrom(account.address)
	require.Len(t, pending, 1)
	var expectedNonce uint64
	require.Equal(t, expectedNonce, pending[0].Nonce())
	require.Len(t, queued, 0)
}

func TestMempool_ReapPromoteDemoteReap(t *testing.T) {
	mp, s := setupMempoolWithAccounts(t, 3)
	txConfig, rechecker, bus, accounts := s.txConfig, s.evmRechecker, s.eventBus, s.accounts
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
	require.NoError(t, mp.Insert(sdk.Context{}.WithContext(context.Background()), tx))

	// wait for another reset to make sure the pool processes the above
	// txn into pending
	require.NoError(t, mp.GetTxPool().Sync())
	require.Equal(t, 1, mp.CountTx())

	// setup tx with nonce 0 to fail recheck.
	legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
	rechecker.SetEVMRecheck(func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
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
	rechecker.SetEVMRecheck(func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		return sdk.Context{}, nil
	})

	// insert the same tx again and make sure the tx can still be returned
	// from the next call to reap
	tx = createMsgEthereumTx(t, txConfig, accounts[0].key, 0, big.NewInt(1e8))
	require.NoError(t, mp.Insert(sdk.Context{}.WithContext(context.Background()), tx))

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
	mp, s := setupMempoolWithAccounts(t, 3)
	vmKeeper, txConfig, bus, accounts := s.vmKeeper, s.txConfig, s.eventBus, s.accounts
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
	require.NoError(t, mp.Insert(sdk.Context{}.WithContext(context.Background()), tx0))
	tx1 := createMsgEthereumTx(t, txConfig, accounts[0].key, 1, big.NewInt(1e8))
	require.NoError(t, mp.Insert(sdk.Context{}.WithContext(context.Background()), tx1))
	tx2 := createMsgEthereumTx(t, txConfig, accounts[0].key, 2, big.NewInt(1e8))
	require.NoError(t, mp.Insert(sdk.Context{}.WithContext(context.Background()), tx2))

	// wait for another reset to make sure the pool processes the above
	// txns into pending
	require.NoError(t, mp.GetTxPool().Sync())
	legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
	require.Eventually(t, func() bool {
		pending, queued := legacyPool.ContentFrom(accounts[0].address)
		return len(pending) == 3 && len(queued) == 0 && mp.CountTx() == 3
	}, time.Second, 10*time.Millisecond)

	// simulate comet calling removeTx with RunTxFinalize for tx0 (the included
	// tx), a new height being published, and our account's nonce incrementing
	// to 1. On the next reset, ScheduleForRemoval drives tx0 out of the pool.
	require.NoError(t, mp.RemoveWithReason(context.Background(), tx0, mempooltypes.RemoveReason{
		Caller: mempooltypes.CallerRunTxFinalize,
	}))
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
	require.Eventually(t, func() bool {
		pending, queued := legacyPool.ContentFrom(accounts[0].address)
		return len(pending) == 2 && len(queued) == 0
	}, time.Second, 10*time.Millisecond)
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

func TestMempool_CosmosNonceAdvanceDropsStaleEVM(t *testing.T) {
	mp, s := setupMempoolWithAccounts(t, 1)
	txConfig, bus, accounts := s.txConfig, s.eventBus, s.accounts
	account := accounts[0]

	require.NoError(t, bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
		Header: cmttypes.Header{
			Height:  1,
			Time:    time.Now(),
			ChainID: strconv.Itoa(constants.EighteenDecimalsChainID),
		},
	}))
	require.NoError(t, mp.GetTxPool().Sync())

	// seed the pool with two pending EVM txs at nonces 0 and 1
	for nonce := uint64(0); nonce < 2; nonce++ {
		tx := createMsgEthereumTx(t, txConfig, account.key, nonce, big.NewInt(1e8))
		require.NoError(t, mp.Insert(context.Background(), tx))
	}
	require.NoError(t, mp.GetTxPool().Sync())

	// ensure initial state is correct
	legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
	pending, queued := legacyPool.ContentFrom(account.address)
	require.Equal(t, 2, len(pending), "expected 2 txs in the pending pool after setup")
	require.Equal(t, 0, len(queued), "expected 0 txs in the queued pool after setup")

	// mock that another val had a cosmos tx with nonce 1 and included that on
	// chain (note that this will never be our node doing this, since the
	// reserver prevents us from having an evm tx and cosmos tx with the same
	// signer(s) in the both pools at the same time)
	cosmosTx := createTestCosmosTx(t, txConfig, account.key, 1)
	err := mp.RemoveWithReason(context.Background(), cosmosTx, mempooltypes.RemoveReason{
		Caller: mempooltypes.CallerRunTxFinalize,
	})
	require.ErrorIs(t, err, mempooltypes.ErrTxNotFound)

	// after we finish finalizing the above block, the chain will advance and
	// the pool will reset
	require.NoError(t, bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
		Header: cmttypes.Header{
			Height:  2,
			Time:    time.Now(),
			ChainID: strconv.Itoa(constants.EighteenDecimalsChainID),
		},
	}))
	require.NoError(t, mp.GetTxPool().Sync())

	// using eventually here since publishing a new block happens async
	require.Eventually(t, func() bool {
		pending, queued := legacyPool.ContentFrom(account.address)
		return len(pending) == 0 && len(queued) == 0
	}, time.Second, 10*time.Millisecond, "expected pending and queued pools to drain after stale txs dropped")
}

func TestMempool_InsertMultiMsgCosmosTx(t *testing.T) {
	mp, s := setupMempoolWithAccounts(t, 3)
	txConfig, bus := s.txConfig, s.eventBus

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

func TestMempool_InsertSynchronous(t *testing.T) {
	mp, s := setupMempoolWithAccounts(t, 3)
	txConfig, bus, accounts := s.txConfig, s.eventBus, s.accounts
	err := bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
		Header: cmttypes.Header{
			Height:  1,
			Time:    time.Now(),
			ChainID: strconv.Itoa(constants.EighteenDecimalsChainID),
		},
	})
	require.NoError(t, err)

	// Wait for reset to happen for block 1
	require.NoError(t, mp.GetTxPool().Sync())

	legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)

	// Insert a transaction using the synchronous Insert method
	// This should wait for the transaction to be added to the pool before returning
	tx := createMsgEthereumTx(t, txConfig, accounts[0].key, 0, big.NewInt(1e8))
	err = mp.Insert(sdk.Context{}.WithContext(context.Background()), tx)
	require.NoError(t, err)

	// After Insert returns, the transaction should already be in the pool
	// (either pending or queued). We don't need to call Sync() to wait.
	pending, queued := legacyPool.ContentFrom(accounts[0].address)
	totalTxs := len(pending) + len(queued)
	require.Equal(t, 1, totalTxs, "transaction should be in pool immediately after Insert returns")

	// Create a transaction with a gas price that would exceed the account balance
	// Account balance is 100000000000100, so set gas price extremely high
	excessiveGasPrice := new(big.Int).SetUint64(accounts[0].initialBalance * 100)
	tx = createMsgEthereumTx(t, txConfig, accounts[0].key, 0, excessiveGasPrice)
	err = mp.Insert(sdk.Context{}.WithContext(context.Background()), tx)

	// The synchronous Insert should return the error from the tx pool
	require.Error(t, err, "Insert should return error when tx pool rejects transaction")
	require.Contains(t, err.Error(), "insufficient funds", "error should indicate insufficient funds")
}

func TestMempool_InsertMultiMsgEthereumTx(t *testing.T) {
	mp, s := setupMempoolWithAccounts(t, 3)
	txConfig, bus := s.txConfig, s.eventBus

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

const (
	txValue    = 100
	txGasLimit = 50000
)

type testAccount struct {
	key            *ecdsa.PrivateKey
	address        common.Address
	nonce          uint64
	initialBalance uint64
}

type testMempoolDependencies struct {
	vmKeeper        *mocks.VMKeeperI
	txConfig        client.TxConfig
	evmRechecker    *MockRechecker
	cosmosRechecker *MockRechecker
	eventBus        *cmttypes.EventBus
	accounts        []testAccount
}

// AdvanceNonce updates the mock vmKeeper to return newNonce for GetNonce(addr)
// and a matching Account from GetAccount. Used to simulate chain-state advance
// between blocks so legacypool's reactive reset path can observe the change.
//
// Assumes prior GetNonce/GetAccount expectations were registered with a
// concrete common.Address argument (as setupMempool does). Expectations
// registered with mock.Anything or other non-Address matchers would not match
// the type assertion below and would survive — leaving stale entries that race
// with the new ones.
func (s *testMempoolDependencies) AdvanceNonce(t *testing.T, addr common.Address, newNonce uint64) {
	t.Helper()
	var toUnset []*mock.Call
	for _, c := range s.vmKeeper.ExpectedCalls {
		switch c.Method {
		case "GetNonce":
			if len(c.Arguments) >= 1 {
				if a, ok := c.Arguments[0].(common.Address); ok && a == addr {
					toUnset = append(toUnset, c)
				}
			}
		case "GetAccount":
			if len(c.Arguments) >= 2 {
				if a, ok := c.Arguments[1].(common.Address); ok && a == addr {
					toUnset = append(toUnset, c)
				}
			}
		}
	}
	for _, c := range toUnset {
		c.Unset() // acquires the mock's internal mutex
	}
	s.vmKeeper.On("GetNonce", addr).Return(newNonce).Maybe()
	s.vmKeeper.On("GetAccount", mock.Anything, addr).Return(&statedb.Account{
		Nonce:   newNonce,
		Balance: uint256.NewInt(1e18),
	}).Maybe()
}

// InstallNonceCheckingRechecker wires the EVM rechecker to reject txs whose
// nonce is below the mock's GetAccount(sender).Nonce. Simulates the ante
// handler's sequence check during reset's demoteUnexecutables.
func (s *testMempoolDependencies) InstallNonceCheckingRechecker(t *testing.T) {
	t.Helper()
	signer := types.LatestSignerForChainID(big.NewInt(int64(constants.EighteenDecimalsChainID)))
	s.evmRechecker.SetEVMRecheck(func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
		from, err := types.Sender(signer, tx)
		if err != nil {
			return ctx, nil
		}
		acc := s.vmKeeper.GetAccount(sdk.Context{}, from)
		if acc == nil {
			return ctx, nil
		}
		if tx.Nonce() < acc.Nonce {
			return sdk.Context{}, fmt.Errorf("stale tx: nonce %d < chain %d", tx.Nonce(), acc.Nonce)
		}
		return ctx, nil
	})
}

func setupMempoolWithAccounts(t *testing.T, numAccounts int) (*mempool.Mempool, testMempoolDependencies) {
	t.Helper()

	return setupMempool(t, numAccounts, 1000)
}

//nolint:unparam
func setupMempool(t *testing.T, numAccounts, insertQueueSize int) (*mempool.Mempool, testMempoolDependencies) {
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
	mockVMKeeper := mocks.NewVMKeeperI(t)
	mockFeeMarketKeeper := mocks.NewFeeMarketKeeper(t)

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
	// Register bank types so cosmos MsgSend txs can be decoded
	banktypes.RegisterInterfaces(encodingConfig.InterfaceRegistry)
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

	config := &mempool.Config{
		LegacyPoolConfig: &legacyConfig,
		BlockGasLimit:    30000000,
		MinTip:           uint256.NewInt(0),
		InsertQueueSize:  insertQueueSize,
	}

	// Create mempool
	evmRechecker := &MockRechecker{}
	cosmosRechecker := &MockRechecker{}
	mp := mempool.NewMempool(
		getCtxCallback,
		log.NewNopLogger(),
		mockVMKeeper,
		mockFeeMarketKeeper,
		txConfig,
		evmRechecker,
		cosmosRechecker,
		config,
		1000, // cosmos pool max tx
	)
	require.NotNil(t, mp)

	mp.SetClientCtx(clientCtx)

	eventBus := cmttypes.NewEventBus()
	require.NoError(t, eventBus.Start())
	mp.SetEventBus(eventBus)

	return mp, testMempoolDependencies{
		vmKeeper:        mockVMKeeper,
		txConfig:        txConfig,
		evmRechecker:    evmRechecker,
		cosmosRechecker: cosmosRechecker,
		eventBus:        eventBus,
		accounts:        accounts,
	}
}

func createMsgEthereumTx(
	t *testing.T,
	txConfig client.TxConfig,
	key *ecdsa.PrivateKey,
	nonce uint64,
	gasPrice *big.Int,
) sdk.Tx {
	t.Helper()

	to := common.Address{0x01} // dummy recipient
	chainID := new(big.Int).SetUint64(vmtypes.GetChainConfig().ChainId)
	msg := vmtypes.NewTx(&vmtypes.EvmTxArgs{
		ChainID:  chainID,
		Nonce:    nonce,
		To:       &to,
		Amount:   big.NewInt(txValue),
		GasLimit: txGasLimit,
		GasPrice: gasPrice,
	})
	msg.From = crypto.PubkeyToAddress(key.PublicKey).Bytes()

	priv := &ethsecp256k1.PrivKey{Key: crypto.FromECDSA(key)}
	cosmosTx, err := evmtestutiltx.PrepareEthTx(txConfig, priv, msg)
	require.NoError(t, err)
	return cosmosTx
}

// createMsgEthereum7702Tx builds a cosmos-wrapped MsgEthereumTx of type-04
// (EIP-7702 SetCode) signed by senderKey with the supplied authorization
// list. Each `auths` entry is signed by its `key` field with the embedded
// nonce. Returns the cosmos-signed sdk.Tx and the underlying ethereum tx
// for assertion convenience.
func createMsgEthereum7702Tx(
	t *testing.T,
	txConfig client.TxConfig,
	senderKey *ecdsa.PrivateKey,
	auths []types.SetCodeAuthorization,
) sdk.Tx {
	t.Helper()

	chainID := vmtypes.GetEthChainConfig().ChainID
	to := common.Address{0x55}
	signedTx := types.MustSignNewTx(senderKey, types.LatestSignerForChainID(chainID), &types.SetCodeTx{
		ChainID:   uint256.MustFromBig(chainID),
		Nonce:     0,
		GasTipCap: uint256.NewInt(1),
		GasFeeCap: uint256.NewInt(1e9),
		Gas:       250000,
		To:        to,
		Value:     uint256.NewInt(0),
		AuthList:  auths,
	})

	msg := &vmtypes.MsgEthereumTx{
		Raw:  vmtypes.EthereumTx{Transaction: signedTx},
		From: crypto.PubkeyToAddress(senderKey.PublicKey).Bytes(),
	}

	// priv=nil → PrepareEthTx skips re-signing (we already have a signed eth
	// tx) and just wraps the message into a cosmos tx with the EVM extension
	// option.
	cosmosTx, err := evmtestutiltx.PrepareEthTx(txConfig, nil, msg)
	require.NoError(t, err)
	return cosmosTx
}

// signSetCodeAuth is a small wrapper around types.SignSetCode for tests.
func signSetCodeAuth(t *testing.T, key *ecdsa.PrivateKey, nonce uint64) types.SetCodeAuthorization {
	t.Helper()
	chainID := vmtypes.GetEthChainConfig().ChainID
	auth, err := types.SignSetCode(key, types.SetCodeAuthorization{
		ChainID: *uint256.MustFromBig(chainID),
		Address: common.Address{0x42},
		Nonce:   nonce,
	})
	require.NoError(t, err)
	return auth
}

// TestStaleNonceHandling covers pool behavior on chain advance across
// scenarios where the eager mechanism can't or doesn't catch the new
// nonce. Cases assert reactive eviction via demoteUnexecutables →
// RecheckEVM, or queued-tx promotion when a 7702 authority bump closes
// a nonce gap.
func TestStaleNonceHandling(t *testing.T) {
	type expect struct {
		pending int
		queued  int
	}
	type scenario struct {
		name             string
		numAccounts      int
		targetIdx        int      // account whose pool we observe
		seedNonces       []uint64 // pending seeds (sequential, no gaps)
		seedQueuedNonces []uint64 // queued seeds (gapped from chain nonce)
		finalize7702     func(t *testing.T, mp *mempool.Mempool, txConfig client.TxConfig, accs []testAccount)
		advanceTo        uint64 // chain nonce to set for accounts[targetIdx]
		wantAfterReset   expect // expected counts after the reset
		// Eager-cache state for accounts[targetIdx] AFTER the test:
		// expectsEager=false when no SetLatestNonce was triggered for the target;
		// expectsEager=true with eagerNonce set when the eager path fired.
		expectsEager bool
		eagerNonce   uint64
	}

	cases := []scenario{
		{
			name:           "stale-sender-no-7702",
			numAccounts:    1,
			targetIdx:      0,
			seedNonces:     []uint64{0},
			advanceTo:      1,
			wantAfterReset: expect{pending: 0, queued: 0},
			// no RemoveWithReason call → eager cache untouched.
			expectsEager: false,
		},
		{
			name:        "authority-after-cross-account-7702",
			numAccounts: 2,
			targetIdx:   1,
			seedNonces:  []uint64{0},
			finalize7702: func(t *testing.T, mp *mempool.Mempool, txConfig client.TxConfig, accs []testAccount) {
				t.Helper()
				auths := []types.SetCodeAuthorization{signSetCodeAuth(t, accs[1].key, 1)}
				cosmosTx := createMsgEthereum7702Tx(t, txConfig, accs[0].key, auths)
				require.NoError(t, mp.RemoveWithReason(context.Background(), cosmosTx, mempooltypes.RemoveReason{
					Caller: mempooltypes.CallerRunTxFinalize,
				}))
			},
			advanceTo:      1,
			wantAfterReset: expect{pending: 0, queued: 0},
			// Eager fires for sender (idx 0); authority (idx 1, the target) is invisible.
			expectsEager: false,
		},
		{
			name:        "sender-+1-gap-after-self-sponsored-7702",
			numAccounts: 1,
			targetIdx:   0,
			seedNonces:  []uint64{0, 1},
			finalize7702: func(t *testing.T, mp *mempool.Mempool, txConfig client.TxConfig, accs []testAccount) {
				t.Helper()
				auths := []types.SetCodeAuthorization{signSetCodeAuth(t, accs[0].key, 1)}
				cosmosTx := createMsgEthereum7702Tx(t, txConfig, accs[0].key, auths)
				require.NoError(t, mp.RemoveWithReason(context.Background(), cosmosTx, mempooltypes.RemoveReason{
					Caller: mempooltypes.CallerRunTxFinalize,
				}))
			},
			advanceTo:      2,
			wantAfterReset: expect{pending: 0, queued: 0},
			// Eager fires for sender (== target) at tx.Nonce()=0; the +1 bump is reactive.
			expectsEager: true,
			eagerNonce:   0,
		},
		{
			// Queued tx whose nonce gap is closed by a self-sponsored 7702.
			// Sender +1 plus 4 self-auth bumps brings chain nonce 0 -> 5,
			// matching tx5's nonce. With nothing else in pending, the
			// noncer falls through to statedb on reset and tx5 promotes.
			name:             "queued-promotes-after-self-sponsored-7702",
			numAccounts:      1,
			targetIdx:        0,
			seedNonces:       nil,
			seedQueuedNonces: []uint64{5},
			finalize7702: func(t *testing.T, mp *mempool.Mempool, txConfig client.TxConfig, accs []testAccount) {
				t.Helper()
				auths := []types.SetCodeAuthorization{
					signSetCodeAuth(t, accs[0].key, 1),
					signSetCodeAuth(t, accs[0].key, 2),
					signSetCodeAuth(t, accs[0].key, 3),
					signSetCodeAuth(t, accs[0].key, 4),
				}
				cosmosTx := createMsgEthereum7702Tx(t, txConfig, accs[0].key, auths)
				require.NoError(t, mp.RemoveWithReason(context.Background(), cosmosTx, mempooltypes.RemoveReason{
					Caller: mempooltypes.CallerRunTxFinalize,
				}))
			},
			advanceTo:      5,
			wantAfterReset: expect{pending: 1, queued: 0},
			expectsEager:   true,
			eagerNonce:     0,
		},
		{
			// Cross-account: acc0 sends 7702 authorizing acc1; the auth
			// bump closes acc1's queued tx gap. Extractor only sees acc0
			// (sender), so acc1's eager LRU is never touched. tx1 promotes
			// on first reset via the statedb fallback.
			name:             "queued-promotes-after-cross-account-7702",
			numAccounts:      2,
			targetIdx:        1,
			seedNonces:       nil,
			seedQueuedNonces: []uint64{1},
			finalize7702: func(t *testing.T, mp *mempool.Mempool, txConfig client.TxConfig, accs []testAccount) {
				t.Helper()
				auths := []types.SetCodeAuthorization{signSetCodeAuth(t, accs[1].key, 0)}
				cosmosTx := createMsgEthereum7702Tx(t, txConfig, accs[0].key, auths)
				require.NoError(t, mp.RemoveWithReason(context.Background(), cosmosTx, mempooltypes.RemoveReason{
					Caller: mempooltypes.CallerRunTxFinalize,
				}))
			},
			advanceTo:      1,
			wantAfterReset: expect{pending: 1, queued: 0},
			expectsEager:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mp, s := setupMempoolWithAccounts(t, tc.numAccounts)
			txConfig, bus, accounts := s.txConfig, s.eventBus, s.accounts
			target := accounts[tc.targetIdx]

			require.NoError(t, bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
				Header: cmttypes.Header{Height: 1, Time: time.Now(), ChainID: strconv.Itoa(constants.EighteenDecimalsChainID)},
			}))
			require.NoError(t, mp.GetTxPool().Sync())

			for _, n := range tc.seedNonces {
				tx := createMsgEthereumTx(t, txConfig, target.key, n, big.NewInt(1e8))
				require.NoError(t, mp.Insert(context.Background(), tx))
			}
			for _, n := range tc.seedQueuedNonces {
				tx := createMsgEthereumTx(t, txConfig, target.key, n, big.NewInt(1e8))
				require.NoError(t, mp.Insert(context.Background(), tx))
			}
			require.NoError(t, mp.GetTxPool().Sync())

			legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
			pending, queued := legacyPool.ContentFrom(target.address)
			require.Len(t, pending, len(tc.seedNonces))
			require.Len(t, queued, len(tc.seedQueuedNonces))

			if tc.finalize7702 != nil {
				tc.finalize7702(t, mp, txConfig, accounts)
			}

			s.InstallNonceCheckingRechecker(t)
			s.AdvanceNonce(t, target.address, tc.advanceTo)

			require.NoError(t, bus.PublishEventNewBlockHeader(cmttypes.EventDataNewBlockHeader{
				Header: cmttypes.Header{Height: 2, Time: time.Now(), ChainID: strconv.Itoa(constants.EighteenDecimalsChainID)},
			}))
			require.NoError(t, mp.GetTxPool().Sync())

			require.Eventually(t, func() bool {
				p, q := legacyPool.ContentFrom(target.address)
				return len(p) == tc.wantAfterReset.pending && len(q) == tc.wantAfterReset.queued
			}, time.Second, 10*time.Millisecond, "reset state mismatch")

			got, ok := legacyPool.LatestNonce(target.address)
			if tc.expectsEager {
				require.True(t, ok, "eager cache should have an entry for target")
				require.Equal(t, tc.eagerNonce, got)
			} else {
				require.False(t, ok, "no eager signal expected; eviction proves reactive path")
			}
		})
	}
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
	EVMRecheckFn    func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error)
	CosmosRecheckFn func(ctx sdk.Context, tx sdk.Tx) (sdk.Context, error)
	ctx             sdk.Context
	lock            sync.Mutex
}

func (mr *MockRechecker) SetEVMRecheck(recheck func(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error)) {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	mr.EVMRecheckFn = recheck
}

func (mr *MockRechecker) SetCosmosRecheck(recheck func(ctx sdk.Context, tx sdk.Tx) (sdk.Context, error)) {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	mr.CosmosRecheckFn = recheck
}

func (mr *MockRechecker) GetContext() (sdk.Context, func()) {
	if mr.ctx.MultiStore() == nil {
		return sdk.Context{}, func() {}
	}
	return mr.ctx.CacheContext()
}

func (mr *MockRechecker) RecheckEVM(ctx sdk.Context, tx *types.Transaction) (sdk.Context, error) {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	if mr.EVMRecheckFn != nil {
		return mr.EVMRecheckFn(ctx, tx)
	}
	return sdk.Context{}, nil
}

func (mr *MockRechecker) RecheckCosmos(ctx sdk.Context, tx sdk.Tx) (sdk.Context, error) {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	if mr.CosmosRecheckFn != nil {
		return mr.CosmosRecheckFn(ctx, tx)
	}
	return ctx, nil
}

func (mr *MockRechecker) Update(ctx sdk.Context, _ *types.Header) {
	mr.ctx = ctx
}

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
			Sequence: uint64(i),
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
