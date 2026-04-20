package mempool_test

import (
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"

	vmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
