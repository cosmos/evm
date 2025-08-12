package utils

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

func SendTransaction(rCtx *types.RPCContext, from, to string, value *big.Int) (string, error) {
	// Create a simple transaction object for testing
	tx := map[string]interface{}{
		"from":     from,
		"to":       to,
		"value":    fmt.Sprintf("0x%x", value),
		"gas":      "0x5208",        // 21000 gas
		"gasPrice": "0x9184e72a000", // 10000000000000
	}

	var txHash string
	err := rCtx.Evmd.RPCClient().Call(&txHash, string("eth_sendTransaction"), tx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	return txHash, nil
}

func GetAccounts(rCtx *types.RPCContext, isGeth bool) ([]string, error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	var accounts []string
	err := ethCli.RPCClient().Call(&accounts, string("eth_accounts"))
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	return accounts, err
}

func NewERC20FilterLogs(rCtx *types.RPCContext, isGeth bool) (ethereum.FilterQuery, string, error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	fErc20Transfer := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(0), // Start from genesis
		Addresses: []common.Address{ethCli.ERC20Addr},
		Topics: [][]common.Hash{
			{ethCli.ERC20Abi.Events["Transfer"].ID}, // Filter for Transfer event
		},
	}

	// Create filter on evmd
	args, err := ToFilterArg(fErc20Transfer)
	if err != nil {
		return fErc20Transfer, "", fmt.Errorf("failed to create filter args: %w", err)
	}
	var evmdFilterID string
	if err = ethCli.RPCClient().CallContext(context.Background(), &evmdFilterID, "eth_newFilter", args); err != nil {
		return fErc20Transfer, "", fmt.Errorf("failed to create filter on evmd: %w", err)
	}

	return fErc20Transfer, evmdFilterID, nil
}
