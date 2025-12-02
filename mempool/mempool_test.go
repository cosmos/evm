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

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txmodule "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"

	"github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/mocks"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/vm/statedb"
	vmtypes "github.com/cosmos/evm/x/vm/types"
)

func TestMempool_Reaping(t *testing.T) {
	t.Run("reap during promote demote cycle", func(t *testing.T) {
		mp, _, txConfig, bus, accounts := setupMempoolWithAccounts(t, 3)
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
			tx := createMsgEthereumTx(t, txConfig, accounts[0].key, nonce, 50000, big.NewInt(1e9), 100)
			err := mp.InsertEVMTxAsync(tx)
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
		legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
		legacyPool.RecheckTxFnFactory = func(_ legacypool.BlockChain) legacypool.RecheckTxFn {
			return func(t *types.Transaction) error {
				if t.Nonce() == 1 {
					return errors.New("recheck failed on tx with nonce 1")
				}
				return nil
			}
		}

		// sync the pool to make sure the above happens
		require.NoError(t, mp.GetTxPool().Sync())
		pending, queued := legacyPool.ContentFrom(accounts[0].address)
		require.Len(t, pending, 1)
		require.Len(t, queued, 1)

		// reap should now return no txs, since no new txs have been validated
		txs, err = mp.ReapNewValidTxs(0, 0)
		require.NoError(t, err)
		require.Len(t, txs, 0)

		// setup recheck to not fail any txs again, tx 2 will not fail this but
		// it wont be promoted since it is nonce gapped from tx 1
		legacyPool.RecheckTxFnFactory = func(chain legacypool.BlockChain) legacypool.RecheckTxFn {
			return func(t *types.Transaction) error {
				return nil
			}
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
		tx := createMsgEthereumTx(t, txConfig, accounts[0].key, 1, 50000, big.NewInt(1e9), 100)
		err = mp.InsertEVMTxAsync(tx)
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
		require.Equal(t, 1, getTxNonce(t, txConfig, txs[0]))
	})

	t.Run("promote and demoted tx is never reaped", func(t *testing.T) {
		mp, _, txConfig, bus, accounts := setupMempoolWithAccounts(t, 3)
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
		tx := createMsgEthereumTx(t, txConfig, accounts[0].key, 0, 50000, big.NewInt(1e9), 100)
		require.NoError(t, mp.InsertEVMTxAsync(tx))

		// wait for another reset to make sure the pool processes the above
		// txn into pending
		require.NoError(t, mp.GetTxPool().Sync())
		require.Equal(t, 1, mp.CountTx())

		// setup tx with nonce 0 to fail recheck.
		legacyPool := mp.GetTxPool().Subpools[0].(*legacypool.LegacyPool)
		legacyPool.RecheckTxFnFactory = func(_ legacypool.BlockChain) legacypool.RecheckTxFn {
			return func(t *types.Transaction) error {
				if t.Nonce() == 0 {
					return errors.New("recheck failed on tx with nonce 0")
				}
				return nil
			}
		}

		// sync the pool to make sure the above happens
		require.NoError(t, mp.GetTxPool().Sync())
		pending, queued := legacyPool.ContentFrom(accounts[0].address)
		require.Len(t, pending, 0)
		require.Len(t, queued, 0)

		// reap should now return no txs, since even though a new tx was
		// validated since the last reap call, it was then invalidated and
		// dropped before the next reap call
		txs, err := mp.ReapNewValidTxs(0, 0)
		require.NoError(t, err)
		require.Len(t, txs, 0)

		// recheck will pass for all txns again
		legacyPool.RecheckTxFnFactory = func(_ legacypool.BlockChain) legacypool.RecheckTxFn {
			return func(t *types.Transaction) error {
				return nil
			}
		}

		// insert the same tx again and make sure the tx can still be returned
		// from the next call to reap
		tx = createMsgEthereumTx(t, txConfig, accounts[0].key, 0, 50000, big.NewInt(1e9), 100)
		require.NoError(t, mp.InsertEVMTxAsync(tx))

		// sync the pool to make sure its promoted to pending
		require.NoError(t, mp.GetTxPool().Sync())
		pending, queued = legacyPool.ContentFrom(accounts[0].address)
		require.Len(t, pending, 1)
		require.Len(t, queued, 0)

		// reap should now return our tx again
		txs, err = mp.ReapNewValidTxs(0, 0)
		require.NoError(t, err)
		require.Len(t, txs, 1)
		require.Equal(t, 1, getTxNonce(t, txConfig, txs[0]))
	})

	t.Run("txs in pending are dropped from reap list from new block", func(t *testing.T) {
		mp, vmKeeper, txConfig, bus, accounts := setupMempoolWithAccounts(t, 3)
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

		tx0 := createMsgEthereumTx(t, txConfig, accounts[0].key, 0, 50000, big.NewInt(1e9), 100)
		require.NoError(t, mp.InsertEVMTxAsync(tx0))
		tx1 := createMsgEthereumTx(t, txConfig, accounts[0].key, 1, 50000, big.NewInt(1e9), 100)
		require.NoError(t, mp.InsertEVMTxAsync(tx1))
		tx2 := createMsgEthereumTx(t, txConfig, accounts[0].key, 2, 50000, big.NewInt(1e9), 100)
		require.NoError(t, mp.InsertEVMTxAsync(tx2))

		// wait for another reset to make sure the pool processes the above
		// txns into pending
		require.NoError(t, mp.GetTxPool().Sync())
		require.Equal(t, 3, mp.CountTx())

		// simulate comet calling removeTx, a new height being published, and
		// our accounts nonce increments to 1, so tx 1 will be invalidated
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
		require.GreaterOrEqual(t, getTxNonce(t, txConfig, txs[0]), 1)
		require.GreaterOrEqual(t, getTxNonce(t, txConfig, txs[1]), 1)
	})
}

// Helper types and functions

type testAccount struct {
	key     *ecdsa.PrivateKey
	address common.Address
	nonce   uint64
}

func setupMempoolWithAccounts(t *testing.T, numAccounts int) (*mempool.ExperimentalEVMMempool, *mocks.VMKeeper, client.TxConfig, *cmttypes.EventBus, []testAccount) {
	logger := log.NewNopLogger()

	// Create accounts
	accounts := make([]testAccount, numAccounts)
	for i := 0; i < numAccounts; i++ {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		accounts[i] = testAccount{
			key:     key,
			address: crypto.PubkeyToAddress(key.PublicKey),
			nonce:   0,
		}
	}

	// Setup EVM chain config
	ethCfg := vmtypes.DefaultChainConfig(constants.EighteenDecimalsChainID)
	err := vmtypes.SetChainConfig(ethCfg)
	require.NoError(t, err)

	err = vmtypes.NewEVMConfigurator().
		WithEVMCoinInfo(constants.ChainsCoinInfo[constants.EighteenDecimalsChainID]).
		Configure()
	require.NoError(t, err)

	// Create mocks
	mockVMKeeper := mocks.NewVMKeeper(t)
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
			Balance: uint256.NewInt(1e18),
		}).Maybe()
		mockVMKeeper.On("GetNonce", acc.address).Return(acc.nonce).Maybe()
		mockVMKeeper.On("GetBalance", acc.address).Return(uint256.NewInt(1e18)).Maybe() // 1 ETH
		mockVMKeeper.On("GetCodeHash", acc.address).Return(common.Hash{}).Maybe()
	}

	mockVMKeeper.On("GetState", mock.Anything, mock.Anything).Return(common.Hash{}).Maybe()
	mockVMKeeper.On("GetCode", mock.Anything, mock.Anything).Return([]byte{}).Maybe()
	mockVMKeeper.On("ForEachStorage", mock.Anything, mock.Anything, mock.Anything).Maybe()
	mockVMKeeper.On("KVStoreKeys").Return(make(map[string]*storetypes.KVStoreKey)).Maybe()

	mockVMKeeper.On("SetEvmMempool", mock.Anything).Once()

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
		logger,
		mockVMKeeper,
		mockFeeMarketKeeper,
		txConfig,
		clientCtx,
		config,
		1000, // cosmos pool max tx
	)
	require.NotNil(t, mp)

	eventBus := cmttypes.NewEventBus()
	require.NoError(t, eventBus.Start())
	mp.SetEventBus(eventBus)

	return mp, mockVMKeeper, txConfig, eventBus, accounts
}

func createMsgEthereumTx(
	t *testing.T,
	txConfig client.TxConfig,
	key *ecdsa.PrivateKey,
	nonce uint64,
	gasLimit uint64,
	gasPrice *big.Int,
	value int64,
) sdk.Tx {
	tx := types.NewTransaction(
		nonce,
		common.Address{0x01}, // Send to a dummy address
		big.NewInt(value),
		gasLimit,
		gasPrice,
		nil,
	)

	chainID := vmtypes.GetChainConfig().ChainId
	signer := types.LatestSignerForChainID(big.NewInt(int64(chainID)))
	signedTx, err := types.SignTx(tx, signer, key)
	require.NoError(t, err)

	return wrapInCosmosSDKTx(t, txConfig, signedTx)
}

func wrapInCosmosSDKTx(t *testing.T, txConfig client.TxConfig, ethTx *types.Transaction) sdk.Tx {
	msg := &vmtypes.MsgEthereumTx{}
	msg.FromEthereumTx(ethTx)

	txBuilder := txConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msg)
	require.NoError(t, err)

	return txBuilder.GetTx()
}

// decodeTxBytes decodes transaction bytes returned from ReapNewValidTxs back into an Ethereum transaction
func decodeTxBytes(t *testing.T, txConfig client.TxConfig, txBytes []byte) *types.Transaction {
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
func getTxNonce(t *testing.T, txConfig client.TxConfig, txBytes []byte) int {
	ethTx := decodeTxBytes(t, txConfig, txBytes)
	return int(ethTx.Nonce())
}

// getTxHash extracts the hash from transaction bytes
func getTxHash(t *testing.T, txConfig client.TxConfig, txBytes []byte) common.Hash {
	ethTx := decodeTxBytes(t, txConfig, txBytes)
	return ethTx.Hash()
}

// getTxGas extracts the gas limit from transaction bytes
func getTxGas(t *testing.T, txConfig client.TxConfig, txBytes []byte) int {
	ethTx := decodeTxBytes(t, txConfig, txBytes)
	return int(ethTx.Gas())
}
