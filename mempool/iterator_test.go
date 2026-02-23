package mempool

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/mempool/miner"
	"github.com/cosmos/evm/mempool/txpool"
	"github.com/cosmos/evm/testutil/constants"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log/v2"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	cosmostx "github.com/cosmos/cosmos-sdk/client/tx"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	sdktxsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	testBondDenom = "aatom"
	testGas       = uint64(21_000)
)

func TestNewEVMMempoolIterator_BothEmpty(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	iter := NewEVMMempoolIterator(nil, nil, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.Nil(t, iter)
}

func TestNewEVMMempoolIterator_EmptyEVMIterator(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{}, nil)
	iter := NewEVMMempoolIterator(evmIter, nil, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.Nil(t, iter)
}

func TestNewEVMMempoolIterator_NilCosmosIterator(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	addr, key := newAddrKey(t)

	tx := buildEVMTx(t, key, 0, big.NewInt(2_000_000_000), big.NewInt(2_000_000_000), chainID)

	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{addr: {tx}}, nil)
	iter := NewEVMMempoolIterator(evmIter, nil, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)
}

func TestNewEVMMempoolIterator_NilEVMWithCosmosOnly(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	_, key := newAddrKey(t)

	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, key, 1_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	iter := NewEVMMempoolIterator(nil, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)
}

func TestIterator_EVMOnly_SingleTx(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	addr, key := newAddrKey(t)

	tx := buildEVMTx(t, key, 0, big.NewInt(2_000_000_000), big.NewInt(2_000_000_000), chainID)

	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{addr: {tx}}, nil)
	iter := NewEVMMempoolIterator(evmIter, nil, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 1)
	require.True(t, isEVMTx(result[0]))
}

func TestIterator_EVMOnly_MultipleTxsSameAccount(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	addr, key := newAddrKey(t)

	var lazyTxs []*txpool.LazyTransaction
	for nonce := uint64(0); nonce < 3; nonce++ {
		tx := buildEVMTx(t, key, nonce, big.NewInt(2_000_000_000), big.NewInt(2_000_000_000), chainID)
		lazyTxs = append(lazyTxs, tx)
	}

	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{addr: lazyTxs}, nil)
	iter := NewEVMMempoolIterator(evmIter, nil, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 3)
	for i, tx := range result {
		require.True(t, isEVMTx(tx), "tx %d should be EVM", i)
	}
}

func TestIterator_EVMOnly_MultipleAccounts(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	txsByAddr := make(map[common.Address][]*txpool.LazyTransaction)
	for i := 0; i < 3; i++ {
		addr, key := newAddrKey(t)
		tx := buildEVMTx(t, key, 0, big.NewInt(2_000_000_000), big.NewInt(2_000_000_000), chainID)
		txsByAddr[addr] = []*txpool.LazyTransaction{tx}
	}

	evmIter := makeEVMIterator(txsByAddr, nil)
	iter := NewEVMMempoolIterator(evmIter, nil, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 3)
}

func TestIterator_EVMMultipleAccountsDifferentPrices(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	gasPrices := []int64{10_000_000_000, 5_000_000_000, 1_000_000_000}
	txsByAddr := make(map[common.Address][]*txpool.LazyTransaction)
	for _, gp := range gasPrices {
		addr, key := newAddrKey(t)
		tx := buildEVMTx(t, key, 0, big.NewInt(gp), big.NewInt(gp), chainID)
		txsByAddr[addr] = []*txpool.LazyTransaction{tx}
	}

	evmIter := makeEVMIterator(txsByAddr, nil)
	iter := NewEVMMempoolIterator(evmIter, nil, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 3)
	for i, tx := range result {
		require.True(t, isEVMTx(tx), "tx %d should be EVM", i)
		require.Equal(t, big.NewInt(gasPrices[i]), txGasPrice(t, tx), "tx %d gas price", i)
	}
}

func TestIterator_CosmosOnly_SingleTx(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	_, key := newAddrKey(t)

	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, key, 1_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	iter := NewEVMMempoolIterator(nil, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 1)
	require.True(t, isCosmosTx(result[0]))
}

func TestIterator_CosmosOnly_MultipleTxs(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	pool := newCosmosPriorityPool()

	// different accounts, different gas prices, ordered by priority
	gasPrices := []int64{3_000_000_000, 2_000_000_000, 1_000_000_000}
	for _, gp := range gasPrices {
		_, key := newAddrKey(t)
		tx := buildCosmosTx(t, txConfig, key, gp, testGas, testBondDenom)
		ctx := sdk.Context{}.WithContext(context.Background())
		require.NoError(t, pool.Insert(ctx, tx))
	}

	ctx := sdk.Context{}.WithContext(context.Background())
	cosmosIter := pool.Select(ctx, nil)
	iter := NewEVMMempoolIterator(nil, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 3)
	for i, tx := range result {
		require.True(t, isCosmosTx(tx), "tx %d should be cosmos", i)
		require.Equal(t, big.NewInt(gasPrices[i]), txGasPrice(t, tx), "tx %d gas price", i)
	}
}

func TestIterator_CosmosOnlyWithBaseFee(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	baseFee := big.NewInt(1_000_000_000)
	pool := newCosmosPriorityPool()

	gasPrices := []int64{10_000_000_000, 5_000_000_000, 2_000_000_000}
	for _, gp := range gasPrices {
		_, key := newAddrKey(t)
		tx := buildCosmosTx(t, txConfig, key, gp, testGas, testBondDenom)
		ctx := sdk.Context{}.WithContext(context.Background())
		require.NoError(t, pool.Insert(ctx, tx))
	}

	ctx := sdk.Context{}.WithContext(context.Background())
	cosmosIter := pool.Select(ctx, nil)
	bc := makeBlockchain(t, baseFee)
	iter := NewEVMMempoolIterator(nil, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, bc)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 3)
	for i, tx := range result {
		require.True(t, isCosmosTx(tx), "tx %d should be cosmos", i)
		require.Equal(t, big.NewInt(gasPrices[i]), txGasPrice(t, tx), "tx %d gas price", i)
	}
}

func TestIterator_Mixed_EVMHigherFee(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(10_000_000_000), big.NewInt(10_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, nil)

	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 1_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 2)
	require.True(t, isEVMTx(result[0]), "first tx should be EVM (higher fee)")
	require.Equal(t, big.NewInt(10_000_000_000), txGasPrice(t, result[0]))
	require.True(t, isCosmosTx(result[1]), "second tx should be cosmos")
	require.Equal(t, big.NewInt(1_000_000_000), txGasPrice(t, result[1]))
}

func TestIterator_Mixed_CosmosHigherFee(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(1_000_000_000), big.NewInt(1_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, nil)

	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 10_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 2)
	require.True(t, isCosmosTx(result[0]), "first tx should be cosmos (higher fee)")
	require.Equal(t, big.NewInt(10_000_000_000), txGasPrice(t, result[0]))
	require.True(t, isEVMTx(result[1]), "second tx should be EVM")
	require.Equal(t, big.NewInt(1_000_000_000), txGasPrice(t, result[1]))
}

func TestIterator_Mixed_EqualFees_PrefersEVM(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(5_000_000_000), big.NewInt(5_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, nil)

	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 5_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 2)
	require.True(t, isEVMTx(result[0]), "first tx should be EVM when fees are equal")
	require.True(t, isCosmosTx(result[1]))
}

func TestIterator_Mixed_InterleavedByFee(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	// EVM fees (tip): 8, 5, 2 gwei (3 accounts)
	// Cosmos fees (no base fee, tip = gas price): 9, 6, 1 gwei (3 accounts)
	// Expected: C(9), E(8), C(6), E(5), E(2), C(1)

	evmGasPrices := []int64{8_000_000_000, 5_000_000_000, 2_000_000_000}
	txsByAddr := make(map[common.Address][]*txpool.LazyTransaction)
	for _, gp := range evmGasPrices {
		addr, key := newAddrKey(t)
		tx := buildEVMTx(t, key, 0, big.NewInt(gp), big.NewInt(gp), chainID)
		txsByAddr[addr] = []*txpool.LazyTransaction{tx}
	}
	evmIter := makeEVMIterator(txsByAddr, nil)

	cosmosGasPrices := []int64{9_000_000_000, 6_000_000_000, 1_000_000_000}
	pool := newCosmosPriorityPool()
	for _, gp := range cosmosGasPrices {
		_, key := newAddrKey(t)
		tx := buildCosmosTx(t, txConfig, key, gp, testGas, testBondDenom)
		ctx := sdk.Context{}.WithContext(context.Background())
		require.NoError(t, pool.Insert(ctx, tx))
	}
	ctx := sdk.Context{}.WithContext(context.Background())
	cosmosIter := pool.Select(ctx, nil)

	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 6, "should yield all 6 transactions")

	expectedEVM := []bool{false, true, false, true, true, false} // true = EVM
	expectedGasPrices := []int64{9_000_000_000, 8_000_000_000, 6_000_000_000, 5_000_000_000, 2_000_000_000, 1_000_000_000}
	for i, tx := range result {
		if expectedEVM[i] {
			require.True(t, isEVMTx(tx), "pos %d: expected EVM", i)
		} else {
			require.True(t, isCosmosTx(tx), "pos %d: expected cosmos", i)
		}
		require.Equal(t, big.NewInt(expectedGasPrices[i]), txGasPrice(t, tx), "pos %d gas price", i)
	}
}

func TestIterator_CosmosZeroFee_PrefersEVM(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(1_000_000_000), big.NewInt(1_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, nil)

	// Cosmos tx with zero gas price
	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 0, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 2)
	require.True(t, isEVMTx(result[0]), "EVM should come first when cosmos has zero fee")
	require.True(t, isCosmosTx(result[1]))
}

func TestIterator_CosmosWrongDenom_PrefersEVM(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(1_000_000_000), big.NewInt(1_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, nil)

	// Cosmos tx with fees in "uosmo" instead of testBondDenom
	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 100_000_000_000, testGas, "uosmo")
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 2)
	require.True(t, isEVMTx(result[0]), "EVM should come first when cosmos fee is in wrong denom")
	require.True(t, isCosmosTx(result[1]))
}

func TestIterator_WithBaseFee_CosmosEffectiveTip(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	// base fee 1 gwei:
	//   EVM: gasFeeCap=gasTipCap=4 gwei, miner tip = min(4, 4-1) = 3
	//   Cosmos: gasPrice=5 gwei, effective tip = 5 - 1 = 4
	//   Cosmos tip (4) > EVM tip (3) → cosmos first
	baseFee := big.NewInt(1_000_000_000)

	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(4_000_000_000), big.NewInt(4_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, baseFee)

	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 5_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	bc := makeBlockchain(t, baseFee)
	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, bc)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 2)
	require.True(t, isCosmosTx(result[0]), "cosmos should come first (effective tip 4 > 3)")
	require.True(t, isEVMTx(result[1]))
}

func TestIterator_WithBaseFee_CosmosBelowBaseFee(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	// cosmos gas price 2 gwei < base fee 3 gwei → effective tip = 0 → EVM preferred
	baseFee := big.NewInt(3_000_000_000)

	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(5_000_000_000), big.NewInt(5_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, baseFee)

	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 2_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	bc := makeBlockchain(t, baseFee)
	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, bc)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 2)
	require.True(t, isEVMTx(result[0]), "EVM should come first when cosmos gas price < base fee")
	require.True(t, isCosmosTx(result[1]))
}

func TestIterator_WithBaseFee_CosmosEqualToBaseFee(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	// cosmos gas price == base fee → effective tip = 0 → EVM preferred
	baseFee := big.NewInt(5_000_000_000)

	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(6_000_000_000), big.NewInt(6_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, baseFee)

	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 5_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	bc := makeBlockchain(t, baseFee)
	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, bc)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 2)
	require.True(t, isEVMTx(result[0]), "EVM should come first when cosmos effective tip is 0")
	require.True(t, isCosmosTx(result[1]))
}

func TestIterator_NoBaseFee_CosmosGasPriceUsedDirectly(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	// nil blockchain → no base fee → cosmos gas price used as effective tip
	// Cosmos 10 gwei vs EVM 5 gwei → cosmos first
	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(5_000_000_000), big.NewInt(5_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, nil)

	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 10_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 2)
	require.True(t, isCosmosTx(result[0]), "cosmos should come first (10 > 5, no base fee)")
	require.True(t, isEVMTx(result[1]))
}

func TestIterator_NilBaseFeeInHeader(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	// blockchain exists but header has nil BaseFee → same as no base fee
	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(5_000_000_000), big.NewInt(5_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, nil)

	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 10_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	bc := makeBlockchain(t, nil)
	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, bc)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 2)
	require.True(t, isCosmosTx(result[0]), "cosmos should win when base fee is nil in header")
}

func TestIterator_MixedWithBaseFee_OrderingCorrect(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	// EVM: gasFeeCap=10, gasTipCap=10, baseFee=2 → miner tip = min(10, 10-2) = 8
	// Cosmos: gasPrice=7, baseFee=2 → effective tip = 7 - 2 = 5
	// EVM tip (8) > Cosmos tip (5) → EVM first
	baseFee := big.NewInt(2_000_000_000)

	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(10_000_000_000), big.NewInt(10_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, baseFee)

	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 7_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	bc := makeBlockchain(t, baseFee)
	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, bc)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 2)
	require.True(t, isEVMTx(result[0]), "EVM should come first (tip 8 > 5)")
	require.True(t, isCosmosTx(result[1]))
}

func TestIterator_CosmosExhaustedFirstThenEVM(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	// 1 cosmos tx with high fee, 2 EVM txs with lower fee
	evmAddr, evmKey := newAddrKey(t)

	var lazyTxs []*txpool.LazyTransaction
	for nonce := uint64(0); nonce < 2; nonce++ {
		tx := buildEVMTx(t, evmKey, nonce, big.NewInt(1_000_000_000), big.NewInt(1_000_000_000), chainID)
		lazyTxs = append(lazyTxs, tx)
	}
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{evmAddr: lazyTxs}, nil)

	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 50_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 3)
	require.True(t, isCosmosTx(result[0]))
	require.Equal(t, big.NewInt(50_000_000_000), txGasPrice(t, result[0]))
	require.True(t, isEVMTx(result[1]))
	require.Equal(t, big.NewInt(1_000_000_000), txGasPrice(t, result[1]))
	require.True(t, isEVMTx(result[2]))
	require.Equal(t, big.NewInt(1_000_000_000), txGasPrice(t, result[2]))
}

func TestIterator_EVMExhaustedFirstThenCosmos(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	// 1 EVM tx with high fee, 2 cosmos txs with lower fee
	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(50_000_000_000), big.NewInt(50_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, nil)

	pool := newCosmosPriorityPool()
	gasPrices := []int64{2_000_000_000, 1_000_000_000}
	for _, gp := range gasPrices {
		_, key := newAddrKey(t)
		cosmosTx := buildCosmosTx(t, txConfig, key, gp, testGas, testBondDenom)
		ctx := sdk.Context{}.WithContext(context.Background())
		require.NoError(t, pool.Insert(ctx, cosmosTx))
	}
	ctx := sdk.Context{}.WithContext(context.Background())
	cosmosIter := pool.Select(ctx, nil)

	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	result := collectAll(t, iter)
	require.Len(t, result, 3)
	require.True(t, isEVMTx(result[0]))
	require.Equal(t, big.NewInt(50_000_000_000), txGasPrice(t, result[0]))
	require.True(t, isCosmosTx(result[1]))
	require.Equal(t, big.NewInt(2_000_000_000), txGasPrice(t, result[1]))
	require.True(t, isCosmosTx(result[2]))
	require.Equal(t, big.NewInt(1_000_000_000), txGasPrice(t, result[2]))
}

func TestIterator_Tx_CalledMultipleTimesReturnsSameType(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	evmAddr, evmKey := newAddrKey(t)

	tx := buildEVMTx(t, evmKey, 0, big.NewInt(2_000_000_000), big.NewInt(2_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		evmAddr: {tx},
	}, nil)

	_, cosmosKey := newAddrKey(t)
	pool := newCosmosPriorityPool()
	cosmosTx := buildCosmosTx(t, txConfig, cosmosKey, 1_000_000_000, testGas, testBondDenom)
	cosmosIter := insertCosmosTxs(t, pool, cosmosTx)

	iter := NewEVMMempoolIterator(evmIter, cosmosIter, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)

	// Tx() should be idempotent
	tx1 := iter.Tx()
	tx2 := iter.Tx()
	tx3 := iter.Tx()
	require.True(t, isEVMTx(tx1))
	require.True(t, isEVMTx(tx2))
	require.True(t, isEVMTx(tx3))
}

func TestIterator_NextReturnsNilWhenExhausted(t *testing.T) {
	txConfig, chainID := setupIteratorTest(t)
	addr, key := newAddrKey(t)

	tx := buildEVMTx(t, key, 0, big.NewInt(2_000_000_000), big.NewInt(2_000_000_000), chainID)
	evmIter := makeEVMIterator(map[common.Address][]*txpool.LazyTransaction{
		addr: {tx},
	}, nil)

	iter := NewEVMMempoolIterator(evmIter, nil, log.NewNopLogger(), txConfig, testBondDenom, chainID, nil)
	require.NotNil(t, iter)
	require.NotNil(t, iter.Tx())

	next := iter.Next()
	require.Nil(t, next)
}

func setupIteratorTest(t *testing.T) (client.TxConfig, *big.Int) {
	t.Helper()

	chainID := uint64(constants.EighteenDecimalsChainID)

	configurator := vmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()

	err := vmtypes.SetChainConfig(vmtypes.DefaultChainConfig(chainID))
	require.NoError(t, err)

	err = configurator.
		WithEVMCoinInfo(constants.ChainsCoinInfo[chainID]).
		Configure()
	require.NoError(t, err)

	enc := encoding.MakeConfig(chainID)
	vmtypes.RegisterInterfaces(enc.InterfaceRegistry)

	return enc.TxConfig, vmtypes.GetEthChainConfig().ChainID
}

// newAddrKey generates an Ethereum address and its corresponding private key.
func newAddrKey(t *testing.T) (common.Address, *ethsecp256k1.PrivKey) {
	t.Helper()
	privkey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	ecdsaKey, err := privkey.ToECDSA()
	require.NoError(t, err)
	return crypto.PubkeyToAddress(ecdsaKey.PublicKey), privkey
}

// makeEVMIterator builds a TransactionsByPriceAndNonce from txs grouped by address.
func makeEVMIterator(txsByAddr map[common.Address][]*txpool.LazyTransaction, baseFee *big.Int) *miner.TransactionsByPriceAndNonce {
	return miner.NewTransactionsByPriceAndNonce(nil, txsByAddr, baseFee)
}

// buildEVMTx creates a signed EVM send transaction.
func buildEVMTx(
	t *testing.T,
	key *ethsecp256k1.PrivKey,
	nonce uint64,
	gasFeeCap,
	gasTipCap *big.Int,
	chainID *big.Int,
) *txpool.LazyTransaction {
	t.Helper()
	ecdsaKey, err := key.ToECDSA()
	require.NoError(t, err)
	to := common.HexToAddress("0x0000000000000000000000000000000000000001")
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &to,
		Value:     big.NewInt(1000),
		Gas:       testGas,
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
	})
	signer := ethtypes.LatestSignerForChainID(chainID)
	signed, err := ethtypes.SignTx(tx, signer, ecdsaKey)
	require.NoError(t, err)
	return &txpool.LazyTransaction{
		Hash:      signed.Hash(),
		Tx:        signed,
		Time:      time.Now(),
		GasFeeCap: uint256.MustFromBig(signed.GasFeeCap()),
		GasTipCap: uint256.MustFromBig(signed.GasTipCap()),
		Gas:       signed.Gas(),
	}
}

// buildCosmosTx creates a signed Cosmos SDK bank send transaction.
func buildCosmosTx(
	t *testing.T,
	txConfig client.TxConfig,
	privKey cryptotypes.PrivKey,
	gasPriceWei int64,
	gas uint64,
	denom string,
) authsigning.Tx {
	t.Helper()
	fromAddr := sdk.AccAddress(privKey.PubKey().Address().Bytes())
	toAddr := sdk.AccAddress(common.HexToAddress("0x0000000000000000000000000000000000000002").Bytes())
	msg := banktypes.NewMsgSend(fromAddr, toAddr, sdk.NewCoins(sdk.NewInt64Coin(denom, 1000)))

	txBuilder := txConfig.NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(msg))

	txBuilder.SetGasLimit(gas)
	feeAmount := new(big.Int).Mul(big.NewInt(gasPriceWei), big.NewInt(int64(gas)))
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(denom, sdkmath.NewIntFromBigInt(feeAmount))))

	signMode, err := authsigning.APISignModeToInternal(txConfig.SignModeHandler().DefaultMode())
	require.NoError(t, err)

	require.NoError(t, txBuilder.SetSignatures(sdktxsigning.SignatureV2{
		PubKey: privKey.PubKey(),
		Data:   &sdktxsigning.SingleSignatureData{SignMode: signMode},
	}))

	// setting sequence doesnt actually matter for these tests
	signerData := authsigning.SignerData{
		ChainID: strconv.Itoa(constants.EighteenDecimalsChainID),
		Address: fromAddr.String(),
		PubKey:  privKey.PubKey(),
	}
	sig, err := cosmostx.SignWithPrivKey(context.TODO(), signMode, signerData, txBuilder, privKey, txConfig, 0)
	require.NoError(t, err)
	require.NoError(t, txBuilder.SetSignatures(sig))

	return txBuilder.GetTx()
}

// insertCosmosTxs inserts transactions into the pool and returns the Select
// iterator.
func insertCosmosTxs(t *testing.T, pool sdkmempool.ExtMempool, txs ...authsigning.Tx) sdkmempool.Iterator {
	t.Helper()
	ctx := sdk.Context{}.WithContext(context.Background())
	for _, tx := range txs {
		require.NoError(t, pool.Insert(ctx, tx))
	}
	return pool.Select(ctx, nil)
}

// newCosmosPriorityPool creates a priority nonce mempool so that txs can be
// inserted into it, and iterated over using the priority nonce iterator in the
// tests
func newCosmosPriorityPool() sdkmempool.ExtMempool {
	cfg := sdkmempool.PriorityNonceMempoolConfig[sdkmath.Int]{
		TxPriority: sdkmempool.TxPriority[sdkmath.Int]{
			GetTxPriority: func(_ context.Context, tx sdk.Tx) sdkmath.Int {
				feeTx, ok := tx.(sdk.FeeTx)
				if !ok {
					return sdkmath.ZeroInt()
				}
				found, coin := feeTx.GetFee().Find(testBondDenom)
				if !found {
					return sdkmath.ZeroInt()
				}
				return coin.Amount.Quo(sdkmath.NewIntFromUint64(feeTx.GetGas()))
			},
			Compare: func(a, b sdkmath.Int) int {
				return a.BigInt().Cmp(b.BigInt())
			},
			MinValue: sdkmath.ZeroInt(),
		},
	}
	return sdkmempool.NewPriorityMempool(cfg)
}

// makeBlockchain creates a minimal Blockchain whose CurrentBlock returns
// the given baseFee. If baseFee is nil, the header will have nil BaseFee.
func makeBlockchain(t *testing.T, baseFee *big.Int) *Blockchain {
	t.Helper()
	return &Blockchain{
		logger:        log.NewNopLogger(),
		blockGasLimit: 30_000_000,
		getCtxCallback: func(_ int64, _ bool) (sdk.Context, error) {
			return sdk.Context{}, errors.New("stub")
		},
		zeroHeader: &ethtypes.Header{
			Difficulty: big.NewInt(0),
			Number:     big.NewInt(0),
			BaseFee:    baseFee,
		},
	}
}

// collectAll drains an iterator, returning all transactions.
func collectAll(t *testing.T, iter sdkmempool.Iterator) []sdk.Tx {
	t.Helper()
	var result []sdk.Tx
	for iter != nil {
		tx := iter.Tx()
		if tx == nil {
			break
		}
		result = append(result, tx)
		iter = iter.Next()
	}
	return result
}

// txGasPrice extracts the gas price from either an EVM or Cosmos transaction.
func txGasPrice(t *testing.T, tx sdk.Tx) *big.Int {
	t.Helper()
	if isEVMTx(tx) {
		return tx.GetMsgs()[0].(*vmtypes.MsgEthereumTx).Raw.GasPrice()
	}
	feeTx, ok := tx.(sdk.FeeTx)
	require.True(t, ok, "expected FeeTx")
	return feeTx.GetFee().AmountOf(testBondDenom).Quo(sdkmath.NewIntFromUint64(feeTx.GetGas())).BigInt()
}

// isEVMTx returns true if the tx contains a MsgEthereumTx (converted from EVM).
func isEVMTx(tx sdk.Tx) bool {
	msgs := tx.GetMsgs()
	if len(msgs) != 1 {
		return false
	}
	_, ok := msgs[0].(*vmtypes.MsgEthereumTx)
	return ok
}

// isCosmosTx returns true if the tx contains a MsgSend (standard Cosmos bank tx).
func isCosmosTx(tx sdk.Tx) bool {
	msgs := tx.GetMsgs()
	if len(msgs) != 1 {
		return false
	}
	_, ok := msgs[0].(*banktypes.MsgSend)
	return ok
}
