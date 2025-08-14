package backend

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/rpc/types"
)

const (
	StatusPending = "pending"
	StatusQueued  = "queued"
)

// The code style for this API is based off of the Go-Ethereum implementation:

// Content returns the transactions contained within the transaction pool.
func (b *Backend) Content() (map[string]map[string]map[string]*types.RPCTransaction, error) {
	content := map[string]map[string]map[string]*types.RPCTransaction{
		StatusPending: make(map[string]map[string]*types.RPCTransaction),
		StatusQueued:  make(map[string]map[string]*types.RPCTransaction),
	}

	// Get the global mempool instance
	evmMempool := mempool.GetGlobalEVMMempool()
	if evmMempool == nil {
		return content, nil
	}

	// Get pending (runnable) and queued (blocked) transactions from the mempool
	pending, queued := evmMempool.GetTxPool().Content()

	// Convert pending (pending) transactions
	for addr, txList := range pending {
		addrStr := addr.Hex()
		if content[StatusPending][addrStr] == nil {
			content[StatusPending][addrStr] = make(map[string]*types.RPCTransaction)
		}

		for _, tx := range txList {
			rpcTx, err := b.convertToRPCTransaction(tx, addr)
			if err != nil {
				b.Logger.Error("failed to convert transaction to RPC format", "error", err, "hash", tx.Hash())
				continue
			}
			content[StatusPending][addrStr][strconv.FormatUint(tx.Nonce(), 10)] = rpcTx
		}
	}

	// Convert queued (queued) transactions
	for addr, txList := range queued {
		addrStr := addr.Hex()
		if content[StatusQueued][addrStr] == nil {
			content[StatusQueued][addrStr] = make(map[string]*types.RPCTransaction)
		}

		for _, tx := range txList {
			rpcTx, err := b.convertToRPCTransaction(tx, addr)
			if err != nil {
				b.Logger.Error("failed to convert transaction to RPC format", "error", err, "hash", tx.Hash())
				continue
			}
			content[StatusQueued][addrStr][strconv.FormatUint(tx.Nonce(), 10)] = rpcTx
		}
	}

	return content, nil
}

// ContentFrom returns the transactions contained within the transaction pool
func (b *Backend) ContentFrom(addr common.Address) (map[string]map[string]*types.RPCTransaction, error) {
	content := make(map[string]map[string]*types.RPCTransaction, 2)

	// Get the global mempool instance
	evmMempool := mempool.GetGlobalEVMMempool()
	if evmMempool == nil {
		return content, nil
	}

	// Get transactions for the specific address
	pending, queue := evmMempool.GetTxPool().ContentFrom(addr)

	// Build the pending transactions
	dump := make(map[string]*types.RPCTransaction, len(pending)) // variable name comes from go-ethereum: https://github.com/ethereum/go-ethereum/blob/0dacfef8ac42e7be5db26c2956f2b238ba7c75e8/internal/ethapi/api.go#L221
	for _, tx := range pending {
		rpcTx, err := b.convertToRPCTransaction(tx, addr)
		if err != nil {
			b.Logger.Error("failed to convert transaction to RPC format", "error", err, "hash", tx.Hash())
			continue
		}
		dump[fmt.Sprintf("%d", tx.Nonce())] = rpcTx
	}
	content[StatusPending] = dump

	// Build the queued transactions
	dump = make(map[string]*types.RPCTransaction, len(queue)) // variable name comes from go-ethereum: https://github.com/ethereum/go-ethereum/blob/0dacfef8ac42e7be5db26c2956f2b238ba7c75e8/internal/ethapi/api.go#L221
	for _, tx := range queue {
		rpcTx, err := b.convertToRPCTransaction(tx, addr)
		if err != nil {
			b.Logger.Error("failed to convert transaction to RPC format", "error", err, "hash", tx.Hash())
			continue
		}
		dump[fmt.Sprintf("%d", tx.Nonce())] = rpcTx
	}
	content[StatusQueued] = dump

	return content, nil
}

// Inspect returns the content of the transaction pool and flattens it into an easily inspectable list.
func (b *Backend) Inspect() (map[string]map[string]map[string]string, error) {
	inspect := map[string]map[string]map[string]string{
		StatusPending: make(map[string]map[string]string),
		StatusQueued:  make(map[string]map[string]string),
	}

	// Get the global mempool instance
	evmMempool := mempool.GetGlobalEVMMempool()
	if evmMempool == nil {
		return inspect, nil
	}

	// Get pending (runnable) and queued (blocked) transactions from the mempool
	pending, queued := evmMempool.GetTxPool().Content()

	// Helper function to format transaction for inspection
	format := func(tx *ethtypes.Transaction) string {
		if to := tx.To(); to != nil {
			return fmt.Sprintf("%s: %v wei + %v gas × %v wei",
				tx.To().Hex(), tx.Value(), tx.Gas(), tx.GasPrice())
		}
		return fmt.Sprintf("contract creation: %v wei + %v gas × %v wei",
			tx.Value(), tx.Gas(), tx.GasPrice())
	}

	// Flatten the pending transactions
	for account, txs := range pending {
		dump := make(map[string]string)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = format(tx)
		}
		inspect[StatusPending][account.Hex()] = dump
	}

	// Flatten the queued transactions
	for account, txs := range queued {
		dump := make(map[string]string)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = format(tx)
		}
		inspect[StatusQueued][account.Hex()] = dump
	}

	return inspect, nil
}

// Status returns the number of pending and queued transaction in the pool.
func (b *Backend) Status() (map[string]hexutil.Uint, error) {
	// Get the global mempool instance
	evmMempool := mempool.GetGlobalEVMMempool()
	if evmMempool == nil {
		return map[string]hexutil.Uint{
			StatusPending: hexutil.Uint(0),
			StatusQueued:  hexutil.Uint(0),
		}, nil
	}

	pending, queued := evmMempool.GetTxPool().Stats()
	return map[string]hexutil.Uint{
		StatusPending: hexutil.Uint(pending), // #nosec G115 -- overflow not a concern for tx counts, as the mempool will limit far before this number is hit. This is taken directly from Geth.
		StatusQueued:  hexutil.Uint(queued),  // #nosec G115 -- overflow not a concern for tx counts, as the mempool will limit far before this number is hit. This is taken directly from Geth.
	}, nil
}

// convertToRPCTransaction converts an Ethereum transaction to RPC format for mempool display
func (b *Backend) convertToRPCTransaction(tx *ethtypes.Transaction, from common.Address) (*types.RPCTransaction, error) {
	curHeader, err := b.CurrentHeader()
	if err != nil {
		return nil, err
	}
	chainConfig := b.ChainConfig()

	// Calculate base fee for pending transactions
	var baseFee *big.Int
	if curHeader != nil && curHeader.BaseFee != nil {
		baseFee = curHeader.BaseFee
	}

	// Create RPC transaction directly
	v, r, s := tx.RawSignatureValues()
	rpcTx := &types.RPCTransaction{
		Type:     hexutil.Uint64(tx.Type()),
		From:     from,
		Gas:      hexutil.Uint64(tx.Gas()),
		GasPrice: (*hexutil.Big)(tx.GasPrice()),
		Hash:     tx.Hash(),
		Input:    hexutil.Bytes(tx.Data()),
		Nonce:    hexutil.Uint64(tx.Nonce()),
		To:       tx.To(),
		Value:    (*hexutil.Big)(tx.Value()),
		V:        (*hexutil.Big)(v),
		R:        (*hexutil.Big)(r),
		S:        (*hexutil.Big)(s),
		ChainID:  (*hexutil.Big)(chainConfig.ChainID),
	}

	// Handle transaction type specific fields
	switch tx.Type() {
	case ethtypes.AccessListTxType:
		al := tx.AccessList()
		rpcTx.Accesses = &al
		rpcTx.ChainID = (*hexutil.Big)(tx.ChainId())
	case ethtypes.DynamicFeeTxType:
		al := tx.AccessList()
		rpcTx.Accesses = &al
		rpcTx.ChainID = (*hexutil.Big)(tx.ChainId())
		rpcTx.GasFeeCap = (*hexutil.Big)(tx.GasFeeCap())
		rpcTx.GasTipCap = (*hexutil.Big)(tx.GasTipCap())
		// Calculate effective gas price for pending EIP-1559 transactions
		if baseFee != nil {
			// price = min(tip, gasFeeCap - baseFee) + baseFee
			price := new(big.Int).Add(tx.GasTipCap(), baseFee)
			if price.Cmp(tx.GasFeeCap()) > 0 {
				price = tx.GasFeeCap()
			}
			rpcTx.GasPrice = (*hexutil.Big)(price)
		} else {
			rpcTx.GasPrice = (*hexutil.Big)(tx.GasFeeCap())
		}
	}

	return rpcTx, nil
}
