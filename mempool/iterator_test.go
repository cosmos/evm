package mempool

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/mempool/txpool"
	testconstants "github.com/cosmos/evm/testutil/constants"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

type fakeFeeTx struct {
	gas uint64
	fee sdk.Coins
}

func (t fakeFeeTx) GetMsgs() []sdk.Msg { return nil }

func (t fakeFeeTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

func (t fakeFeeTx) GetGas() uint64 { return t.gas }

func (t fakeFeeTx) GetFee() sdk.Coins { return t.fee }

func (t fakeFeeTx) FeePayer() []byte { return nil }

func (t fakeFeeTx) FeeGranter() []byte { return nil }

type fakeCosmosIter struct {
	tx   sdk.Tx
	next sdkmempool.Iterator
}

func (it *fakeCosmosIter) Next() sdkmempool.Iterator { return it.next }

func (it *fakeCosmosIter) Tx() sdk.Tx { return it.tx }

type fakeEVMIter struct {
	firstFee  *uint256.Int
	secondFee *uint256.Int
	tx        *txpool.LazyTransaction

	peekCalls  int
	shiftCalls int
}

func (it *fakeEVMIter) Peek() (*txpool.LazyTransaction, *uint256.Int) {
	it.peekCalls++
	if it.peekCalls == 1 {
		return it.tx, it.firstFee
	}
	return it.tx, it.secondFee
}

func (it *fakeEVMIter) Shift() { it.shiftCalls++ }

func (it *fakeEVMIter) Empty() bool { return false }

func TestEVMMempoolIterator_AdvancesSameSourceAsTx(t *testing.T) {
	denom := "aatom"

	cosmosTx := fakeFeeTx{
		gas: 1,
		fee: sdk.NewCoins(sdk.NewInt64Coin(denom, 1000)),
	}
	cosmosNext := &fakeCosmosIter{tx: nil, next: nil}
	cosmosIter := &fakeCosmosIter{tx: cosmosTx, next: cosmosNext}

	to := common.Address{}
	ethTx := ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: 0, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(0)})
	lazy := &txpool.LazyTransaction{Tx: ethTx}
	evmIter := &fakeEVMIter{firstFee: uint256.NewInt(1), secondFee: uint256.NewInt(5000), tx: lazy}

	it := &EVMMempoolIterator{
		evmIterator:    evmIter,
		cosmosIterator: cosmosIter,
		logger:         log.NewNopLogger(),
		bondDenom:      denom,
		chainID:        big.NewInt(1),
		blockchain:     nil,
	}

	got := it.Tx()
	feeTx, ok := got.(sdk.FeeTx)
	if !ok {
		t.Fatalf("expected Cosmos FeeTx, got %T", got)
	}
	if feeTx.GetFee().AmountOf(denom).Int64() != 1000 {
		t.Fatalf("expected cosmos fee 1000%s, got %s", denom, feeTx.GetFee().String())
	}

	if it.Next() == nil {
		t.Fatalf("expected iterator to continue (EVM still present)")
	}
	if evmIter.shiftCalls != 0 {
		t.Fatalf("expected EVM not to advance, shiftCalls=%d", evmIter.shiftCalls)
	}
	if it.cosmosIterator != cosmosNext {
		t.Fatalf("expected Cosmos iterator to advance")
	}
}

func TestEVMMempoolIterator_getPreferredTransaction(t *testing.T) {
	denom := "aatom"
	chainID := uint64(testconstants.EighteenDecimalsChainID)
	chainIDBig := new(big.Int)
	chainIDBig.SetUint64(chainID)

	configurator := vmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	t.Cleanup(configurator.ResetTestConfig)
	err := configurator.WithEVMCoinInfo(vmtypes.EvmCoinInfo{
		Denom:         denom,
		ExtendedDenom: denom,
		DisplayDenom:  "atom",
		Decimals:      vmtypes.EighteenDecimals.Uint32(),
	}).Configure()
	require.NoError(t, err)

	cosmosFeeTx := fakeFeeTx{gas: 1, fee: sdk.NewCoins(sdk.NewInt64Coin(denom, 1000))}

	to := common.Address{}
	unsigned := ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: 0, GasPrice: big.NewInt(1), Gas: 21000, To: &to, Value: big.NewInt(0)})
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	signed, err := ethtypes.SignTx(unsigned, ethtypes.LatestSignerForChainID(chainIDBig), key)
	require.NoError(t, err)
	lazy := &txpool.LazyTransaction{Tx: signed}

	// conversion failure due to wrong chain ID
	wrongChainIDSigned, err := ethtypes.SignTx(unsigned, ethtypes.LatestSignerForChainID(big.NewInt(2)), key)
	require.NoError(t, err)
	wrongChainLazy := &txpool.LazyTransaction{Tx: wrongChainIDSigned}

	it := &EVMMempoolIterator{
		logger:    log.NewNopLogger(),
		txConfig:  encoding.MakeConfig(chainID).TxConfig,
		bondDenom: denom,
		chainID:   chainIDBig,
	}

	type testCase struct {
		name      string
		evmTx     *txpool.LazyTransaction
		evmFee    *uint256.Int
		cosmosTx  sdk.Tx
		cosmosFee *uint256.Int
		wantSrc   txSource
		wantNil   bool
		wantShift bool
	}

	testCases := []testCase{
		{name: "nil both", wantSrc: txSourceUnknown, wantNil: true},
		{name: "cosmos only", cosmosTx: cosmosFeeTx, cosmosFee: uint256.NewInt(5), wantSrc: txSourceCosmos},
		{name: "evm only", evmTx: lazy, evmFee: uint256.NewInt(5), wantSrc: txSourceEVM},
		{name: "prefer cosmos when strictly higher", evmTx: lazy, evmFee: uint256.NewInt(5), cosmosTx: cosmosFeeTx, cosmosFee: uint256.NewInt(6), wantSrc: txSourceCosmos},
		{name: "prefer evm on tie", evmTx: lazy, evmFee: uint256.NewInt(5), cosmosTx: cosmosFeeTx, cosmosFee: uint256.NewInt(5), wantSrc: txSourceEVM},
		{name: "prefer evm when cosmos tip is zero", evmTx: lazy, evmFee: uint256.NewInt(5), cosmosTx: cosmosFeeTx, cosmosFee: uint256.NewInt(0), wantSrc: txSourceEVM},
		{name: "prefer evm when cosmos tip is nil", evmTx: lazy, evmFee: uint256.NewInt(5), cosmosTx: cosmosFeeTx, cosmosFee: nil, wantSrc: txSourceEVM},
		{name: "fallback to cosmos when evm conversion fails", evmTx: wrongChainLazy, evmFee: uint256.NewInt(10), cosmosTx: cosmosFeeTx, cosmosFee: uint256.NewInt(1), wantSrc: txSourceCosmos, wantShift: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			it.shiftEVMOnNext = false
			tx, src := it.getPreferredTransaction(tc.evmTx, tc.evmFee, tc.cosmosTx, tc.cosmosFee)
			require.Equal(t, tc.wantSrc, src)
			require.Equal(t, tc.wantShift, it.shiftEVMOnNext)

			if tc.wantNil {
				require.Nil(t, tx)
				return
			}
			require.NotNil(t, tx)

			if tc.wantSrc == txSourceCosmos {
				feeTx, ok := tx.(sdk.FeeTx)
				require.True(t, ok, "expected Cosmos FeeTx, got %T", tx)
				require.Equal(t, int64(1000), feeTx.GetFee().AmountOf(denom).Int64())
			}
		})
	}

	t.Run("Next advances EVM on fallback", func(t *testing.T) {
		cosmosTx := fakeFeeTx{gas: 1, fee: sdk.NewCoins(sdk.NewInt64Coin(denom, 1))}
		cosmosNext := &fakeCosmosIter{tx: nil, next: nil}
		cosmosIter := &fakeCosmosIter{tx: cosmosTx, next: cosmosNext}

		evmi := &fakeEVMIter{firstFee: uint256.NewInt(10), secondFee: uint256.NewInt(10), tx: wrongChainLazy}
		iter := &EVMMempoolIterator{
			evmIterator:    evmi,
			cosmosIterator: cosmosIter,
			logger:         log.NewNopLogger(),
			txConfig:       encoding.MakeConfig(chainID).TxConfig,
			bondDenom:      denom,
			chainID:        chainIDBig,
			blockchain:     nil,
		}

		got := iter.Tx()
		feeTx, ok := got.(sdk.FeeTx)
		require.True(t, ok, "expected Cosmos FeeTx, got %T", got)
		require.Equal(t, int64(1), feeTx.GetFee().AmountOf(denom).Int64())

		require.NotNil(t, iter.Next())
		require.Equal(t, 1, evmi.shiftCalls)
		require.Equal(t, cosmosNext, iter.cosmosIterator)
	})
}
