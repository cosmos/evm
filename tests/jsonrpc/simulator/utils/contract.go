package utils

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// TransferTokensToAccount transfers ERC20 tokens from owner to a specific account
func TransferTokensToAccount(rCtx *types.RPCContext, recipient common.Address, amount *big.Int, ownerPrivateKey string, isGeth bool) (*ethtypes.Receipt, error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	// Create transaction
	data, err := ethCli.ERC20Abi.Pack("transfer", recipient, amount)
	if err != nil {
		return nil, fmt.Errorf("failed to pack transfer data: %w", err)
	}
	return SendRawTransaction(rCtx, ownerPrivateKey, ethCli.ERC20Addr, big.NewInt(0), data, isGeth)
}

// VerifyTokenBalances verifies that token balances are identical on both networks
func VerifyTokenBalances(rCtx *types.RPCContext) error {
	fmt.Printf("\n=== Verifying Token Balance Synchronization ===\n")

	accounts := []string{config.Dev0PrivateKey, config.Dev1PrivateKey, config.Dev2PrivateKey, config.Dev3PrivateKey}

	for _, privateKeyHex := range accounts {
		_, addr, err := GetPrivateKeyAndAddress(privateKeyHex)
		if err != nil {
			return fmt.Errorf("failed to get address for verification: %w", err)
		}

		// Get balance on evmd
		evmdBalance, err := getTokenBalance(rCtx, addr, false)
		if err != nil {
			return fmt.Errorf("failed to get evmd balance for %s: %w", addr.Hex(), err)
		}

		// Get balance on geth
		gethBalance, err := getTokenBalance(rCtx, addr, true)
		if err != nil {
			return fmt.Errorf("failed to get geth balance for %s: %w", addr.Hex(), err)
		}

		// Compare balances
		if evmdBalance.Cmp(gethBalance) != 0 {
			return fmt.Errorf("balance mismatch for %s: evmd=%s, geth=%s",
				addr.Hex(), evmdBalance.String(), gethBalance.String())
		}

		readableBalance := new(big.Int).Div(evmdBalance, big.NewInt(1e18))
		fmt.Printf("  ✓ %s: %s tokens (identical on both networks)\n",
			addr.Hex()[:10]+"...", readableBalance.String())
	}

	fmt.Printf("✓ All token balances verified as identical\n")
	return nil
}

// getTokenBalance gets the ERC20 token balance for an address
func getTokenBalance(rCtx *types.RPCContext, account common.Address, isGeth bool) (*big.Int, error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	data, err := ethCli.ERC20Abi.Pack("balanceOf", account)
	if err != nil {
		return nil, fmt.Errorf("failed to pack balanceOf data: %w", err)
	}

	msg := ethereum.CallMsg{
		To:   &ethCli.ERC20Addr,
		Data: data,
	}
	result, err := ethCli.CallContract(context.Background(), msg, nil)
	if err != nil {
		return nil, err
	}

	// Convert result to big.Int
	balance := new(big.Int).SetBytes(result)
	return balance, nil
}
