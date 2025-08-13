package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// waitForTransactionReceipt waits for a transaction receipt
func WaitForTx(rCtx *types.RPCContext, txHash common.Hash, timeout time.Duration, isGeth bool) (*ethtypes.Receipt, error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction %s", txHash.Hex())
		case <-ticker.C:
			receipt, err := ethCli.TransactionReceipt(context.Background(), txHash)
			if err != nil {
				continue // Transaction not mined yet
			}
			return receipt, nil
		}
	}
}
