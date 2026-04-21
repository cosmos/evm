package mempool

import (
	"context"
	"errors"
	"fmt"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	CodeTypeNoRetry = uint32(1)
)

// NewReapTxsHandler is the handler for ABCI.ReapTxs. It's used by CometBFT
// to reap valid txs from the mempool and share them with other peers. Handles concurrent requests.
func (m *Mempool) NewReapTxsHandler() sdk.ReapTxsHandler {
	return func(req *abci.RequestReapTxs) (*abci.ResponseReapTxs, error) {
		maxBytes, maxGas := req.GetMaxBytes(), req.GetMaxGas()

		txs, err := m.ReapNewValidTxs(maxBytes, maxGas)
		if err != nil {
			return nil, fmt.Errorf(
				"reaping new valid txs from evm mempool with %d max bytes and %d max gas: %w",
				maxBytes, maxGas, err,
			)
		}

		return &abci.ResponseReapTxs{Txs: txs}, nil
	}
}

// NewInsertTxHandler is the handler for ABCI.InsertTx. Used by CometBFT to asynchronously
// insert a new tx into the mempool. Supersedes ABCI.CheckTx. Handles concurrent requests
func (m *Mempool) NewInsertTxHandler(txDecoder sdk.TxDecoder) sdk.InsertTxHandler {
	return func(req *abci.RequestInsertTx) (*abci.ResponseInsertTx, error) {
		tx, err := txDecoder(req.GetTx())
		if err != nil {
			return nil, fmt.Errorf("decoding tx: %w", err)
		}

		code := abci.CodeTypeOK

		if err := m.InsertAsync(tx); err != nil {
			// since we are using InsertAsync here, the only errors that will
			// be returned are via the InsertQueue if it is full (for EVM txs),
			// in which case we should retry, or some level of validation
			// failed on a cosmos tx (CheckTx), invalid encoding, etc, in which
			// case we should not retry
			if errors.Is(err, ErrQueueFull) {
				code = abci.CodeTypeRetry
			} else {
				code = CodeTypeNoRetry
			}
		}

		return &abci.ResponseInsertTx{Code: code}, nil
	}
}

// NewCheckTxHandler is the handler for ABCI.CheckTx. Note: it's async and doesn't expect the caller
// to acquire ABCI lock. Used ONLY to support BroadcastTxSync (cosmos rpc). All EVM txs
// should be inserted via InsertTx handler or EVM RPC.
func (m *Mempool) NewCheckTxHandler(txDecoder sdk.TxDecoder, timeout time.Duration) sdk.CheckTxHandler {
	return func(_ sdk.RunTx, req *abci.RequestCheckTx) (*abci.ResponseCheckTx, error) {
		if req.Type != abci.CheckTxType_New {
			return nil, fmt.Errorf("unsupported abci.RequestCheckTx.Type: %s", req.Type)
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		tx, err := txDecoder(req.GetTx())
		if err != nil {
			return nil, fmt.Errorf("decoding tx: %w", err)
		}

		err = m.Insert(ctx, tx)

		return ErrAsCheckTxResponse(err), nil
	}
}

// ErrAsCheckTxResponse converts an error to a ResponseCheckTx object, respecting error wrapping.
func ErrAsCheckTxResponse(err error) *abci.ResponseCheckTx {
	if err == nil {
		return &abci.ResponseCheckTx{Code: abci.CodeTypeOK}
	}

	space, code, log := errorsmod.ABCIInfo(err, false)

	// walk the wrap chain for the innermost sdk error
	for e := errors.Unwrap(err); e != nil && space == errorsmod.UndefinedCodespace; e = errors.Unwrap(e) {
		space, code, log = errorsmod.ABCIInfo(e, false)
	}

	return &abci.ResponseCheckTx{
		Codespace: space,
		Code:      code,
		Log:       log,
		GasWanted: 0,
		GasUsed:   0,
		Events:    nil,
	}
}
