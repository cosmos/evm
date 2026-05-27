package mempool_test

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/mempool"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

func TestMempoolHandlers(t *testing.T) {
	asEvmTx := func(t *testing.T, tx sdk.Tx) *vmtypes.EthereumTx {
		t.Helper()
		msg, ok := tx.GetMsgs()[0].(*vmtypes.MsgEthereumTx)
		require.True(t, ok)

		return &msg.Raw
	}

	t.Run("CheckTx", func(t *testing.T) {
		t.Run("EVM", func(t *testing.T) {
			// ARRANGE
			// given the mempool
			mp, deps := setupMempool(t, 2, 1)

			// given checkTxHandler
			const timeout = time.Second
			checkTxHandler := mp.NewCheckTxHandler(deps.txConfig.TxDecoder(), timeout)

			// given tx
			tx := createMsgEthereumTx(t, deps.txConfig, deps.accounts[0].key, 0, big.NewInt(1e8))
			evmTx := asEvmTx(t, tx)

			txBytes, err := deps.txConfig.TxEncoder()(tx)
			require.NoError(t, err)

			// ACT
			resp, err := checkTxHandler(sdk.RunTx(nil), &abci.RequestCheckTx{
				Type: abci.CheckTxType_New,
				Tx:   txBytes,
			})

			// ASSERT
			require.NoError(t, err)
			require.Equal(t, abci.CodeTypeOK, resp.Code)

			mempoolTx := mp.GetTxPool().Get(evmTx.Hash())
			require.NotNil(t, mempoolTx)

			t.Run("Duplicate", func(t *testing.T) {
				// ACT
				// Add again
				resp, err := checkTxHandler(sdk.RunTx(nil), &abci.RequestCheckTx{
					Type: abci.CheckTxType_New,
					Tx:   txBytes,
				})

				// ASSERT
				require.NoError(t, err)
				require.Equal(t, uint32(1), resp.Code)
				require.Contains(t, resp.Log, "already known")
			})

			t.Run("TimedOut", func(t *testing.T) {
				// ARRANGE
				// Given a slow decoder
				decoder := func(tx []byte) (sdk.Tx, error) {
					time.Sleep(100 * time.Millisecond)
					return deps.txConfig.TxDecoder()(tx)
				}

				// Given a checkTxHandler that times out
				checkTxHandler := mp.NewCheckTxHandler(decoder, 50*time.Millisecond)

				// Given tx2
				tx2 := createMsgEthereumTx(t, deps.txConfig, deps.accounts[1].key, 0, big.NewInt(1e8))
				tx2Bytes, err := deps.txConfig.TxEncoder()(tx2)
				require.NoError(t, err)

				// ACT
				resp, err := checkTxHandler(sdk.RunTx(nil), &abci.RequestCheckTx{
					Type: abci.CheckTxType_New,
					Tx:   tx2Bytes,
				})

				// ASSERT
				require.NoError(t, err)
				require.Equal(t, uint32(1), resp.Code)
				require.Contains(t, resp.Log, "context deadline exceeded")
			})
		})

		t.Run("Cosmos", func(t *testing.T) {
			// ARRANGE
			// given the mempool
			mp, deps := setupMempool(t, 2, 1000)

			// given checkTxHandler
			const timeout = time.Second
			checkTxHandler := mp.NewCheckTxHandler(deps.txConfig.TxDecoder(), timeout)

			// given a cosmos tx
			tx := createTestCosmosTx(t, deps.txConfig, deps.accounts[0].key, 0)
			txBytes, err := deps.txConfig.TxEncoder()(tx)
			require.NoError(t, err)

			// ACT
			resp, err := checkTxHandler(sdk.RunTx(nil), &abci.RequestCheckTx{
				Type: abci.CheckTxType_New,
				Tx:   txBytes,
			})

			// ASSERT
			require.NoError(t, err)
			require.Equal(t, abci.CodeTypeOK, resp.Code)
			require.Equal(t, 1, mp.CountTx())

			t.Run("TimedOut", func(t *testing.T) {
				// ARRANGE
				// Given a slow decoder
				decoder := func(tx []byte) (sdk.Tx, error) {
					time.Sleep(100 * time.Millisecond)
					return deps.txConfig.TxDecoder()(tx)
				}

				// Given a checkTxHandler that times out
				checkTxHandler := mp.NewCheckTxHandler(decoder, 50*time.Millisecond)

				// Given a tx from a different signer so reserver doesn't collide
				tx2 := createTestCosmosTx(t, deps.txConfig, deps.accounts[1].key, 0)
				tx2Bytes, err := deps.txConfig.TxEncoder()(tx2)
				require.NoError(t, err)

				// ACT
				resp, err := checkTxHandler(sdk.RunTx(nil), &abci.RequestCheckTx{
					Type: abci.CheckTxType_New,
					Tx:   tx2Bytes,
				})

				// ASSERT
				require.NoError(t, err)
				assert.Equal(t, uint32(1), resp.Code)
				assert.Contains(t, resp.Log, "context deadline exceeded")
			})
		})

		t.Run("Recheck", func(t *testing.T) {
			// ARRANGE
			mp, deps := setupMempool(t, 1, 1)
			checkTxHandler := mp.NewCheckTxHandler(deps.txConfig.TxDecoder(), time.Second)

			// ACT
			resp, err := checkTxHandler(sdk.RunTx(nil), &abci.RequestCheckTx{
				Type: abci.CheckTxType_Recheck,
				Tx:   []byte("anything"),
			})

			// ASSERT
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported")
			assert.Nil(t, resp)
		})

		t.Run("DecodeError", func(t *testing.T) {
			// ARRANGE
			mp, deps := setupMempool(t, 1, 1)
			checkTxHandler := mp.NewCheckTxHandler(deps.txConfig.TxDecoder(), time.Second)

			// ACT
			resp, err := checkTxHandler(sdk.RunTx(nil), &abci.RequestCheckTx{
				Type: abci.CheckTxType_New,
				Tx:   []byte("not a valid tx"),
			})

			// ASSERT
			require.Error(t, err)
			assert.Contains(t, err.Error(), "decoding tx")
			assert.Nil(t, resp)
		})
	})
}

func TestErrAsCheckTxResponse(t *testing.T) {
	assertResponse := func(codespace string, code uint32, log string) func(t *testing.T, resp *abci.ResponseCheckTx) {
		return func(t *testing.T, resp *abci.ResponseCheckTx) {
			t.Helper()

			require.Equal(t, codespace, resp.Codespace)
			require.Equal(t, code, resp.Code)
			require.Equal(t, log, resp.Log)
		}
	}

	for _, tt := range []struct {
		name   string
		err    error
		assert func(t *testing.T, resp *abci.ResponseCheckTx)
	}{
		{
			name: "nil error",
			err:  nil,
			assert: func(t *testing.T, resp *abci.ResponseCheckTx) {
				t.Helper()
				require.Equal(t, abci.CodeTypeOK, resp.Code)
			},
		},
		{
			name:   "std error",
			err:    fmt.Errorf("oops"),
			assert: assertResponse(errorsmod.UndefinedCodespace, 1, "oops"),
		},
		{
			name:   "std wrapped error",
			err:    errorsmod.Wrap(fmt.Errorf("oops"), "wrapped"),
			assert: assertResponse(errorsmod.UndefinedCodespace, 1, "oops"),
		},
		{
			name:   "sdk error",
			err:    errortypes.ErrInsufficientFee,
			assert: assertResponse(errortypes.RootCodespace, 13, "insufficient fee"),
		},
		{
			name:   "wrapped sdk error",
			err:    errorsmod.Wrap(errortypes.ErrOutOfGas, "unable to exec tx"),
			assert: assertResponse(errortypes.RootCodespace, 11, "unable to exec tx: out of gas"),
		},
		{
			name:   "nested sdk wrapped sdk error",
			err:    errorsmod.Wrap(errorsmod.Wrap(errortypes.ErrOutOfGas, "unable to exec tx"), "wrapped"),
			assert: assertResponse(errortypes.RootCodespace, 11, "wrapped: unable to exec tx: out of gas"),
		},
		{
			name:   "std wrapped sdk error",
			err:    fmt.Errorf("something went wrong: %w", errortypes.ErrOutOfGas),
			assert: assertResponse(errortypes.RootCodespace, 11, "out of gas"),
		},
		{
			name:   "nested std wrapped sdk error",
			err:    fmt.Errorf("something went wrong: %w", fmt.Errorf("oops: %w", errortypes.ErrTxTimeout)),
			assert: assertResponse(errortypes.RootCodespace, 42, "tx timeout"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resp := mempool.ErrAsCheckTxResponse(tt.err)
			tt.assert(t, resp)
		})
	}
}
