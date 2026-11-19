package bank

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// BalancesMethod defines the ABI method name for the bank Balances
	// query.
	BalancesMethod = "balances"
	// TotalSupplyMethod defines the ABI method name for the bank TotalSupply
	// query.
	TotalSupplyMethod = "totalSupply"
	// SupplyOfMethod defines the ABI method name for the bank SupplyOf
	// query.
	SupplyOfMethod = "supplyOf"
)

// Balances returns the given account's balances for denominations with registered ERC-20
// token pairs and their corresponding ERC-20 addresses (address, amount). The amount returned
// for each token has the original decimals precision stored in x/bank.
// This method charges for each denomination visited while enumerating balances.
func (p Precompile) Balances(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	account, err := ParseBalancesArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error calling account balances in bank precompile: %s", err)
	}

	i := 0
	balances := make([]Balance, 0)

	p.bankKeeper.IterateAccountBalances(ctx, account, func(coin sdk.Coin) bool {
		defer func() { i++ }()

		// Charge for every visited denomination, including unregistered ones that
		// are omitted from the result. Store gas is disabled for this precompile.
		// The base charge covers the first denomination.
		if i > 0 {
			ctx.GasMeter().ConsumeGas(GasBalances, "ERC-20 extension balances method")
		}

		// Only return addresses that SupplyOf can resolve to a registered token pair.
		// Unregistered IBC vouchers otherwise expose a hash-derived address with zero supply.
		if !p.erc20Keeper.IsDenomRegistered(ctx, coin.Denom) {
			return false
		}

		contractAddress, err := p.erc20Keeper.GetCoinAddress(ctx, coin.Denom)
		if err != nil {
			return false
		}

		balances = append(balances, Balance{
			ContractAddress: contractAddress,
			Amount:          coin.Amount.BigInt(),
		})

		return false
	})

	return method.Outputs.Pack(balances)
}

// TotalSupply returns the total supply for denominations with registered ERC-20 token pairs.
// The amount returned for each token has the original decimals precision stored in x/bank.
// This method charges for each denomination visited while enumerating total supply.
func (p Precompile) TotalSupply(
	ctx sdk.Context,
	method *abi.Method,
	_ []interface{},
) ([]byte, error) {
	i := 0
	totalSupply := make([]Balance, 0)

	p.bankKeeper.IterateTotalSupply(ctx, func(coin sdk.Coin) bool {
		defer func() { i++ }()

		// Charge for every visited denomination, including unregistered ones that
		// are omitted from the result. Store gas is disabled for this precompile.
		// The base charge covers the first denomination.
		if i > 0 {
			ctx.GasMeter().ConsumeGas(GasTotalSupply, "ERC-20 extension totalSupply method")
		}

		// Keep the listed tokens consistent with SupplyOf's registered-pair lookup.
		if !p.erc20Keeper.IsDenomRegistered(ctx, coin.Denom) {
			return false
		}

		contractAddress, err := p.erc20Keeper.GetCoinAddress(ctx, coin.Denom)
		if err != nil {
			return false
		}

		totalSupply = append(totalSupply, Balance{
			ContractAddress: contractAddress,
			Amount:          coin.Amount.BigInt(),
		})

		return false
	})

	return method.Outputs.Pack(totalSupply)
}

// SupplyOf returns the total supply of a given registered erc20 token
// from the x/bank module. If the ERC20 token doesn't have a registered
// TokenPair, the method returns a supply of zero.
// The amount returned with this query has the original decimals precision
// stored in the x/bank.
func (p Precompile) SupplyOf(
	ctx sdk.Context,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	erc20ContractAddress, err := ParseSupplyOfArgs(args)
	if err != nil {
		return nil, fmt.Errorf("error getting the supply in bank precompile: %s", err)
	}

	tokenPairID := p.erc20Keeper.GetERC20Map(ctx, erc20ContractAddress)
	tokenPair, found := p.erc20Keeper.GetTokenPair(ctx, tokenPairID)
	if !found {
		return method.Outputs.Pack(big.NewInt(0))
	}

	supply := p.bankKeeper.GetSupply(ctx, tokenPair.Denom)

	return method.Outputs.Pack(supply.Amount.BigInt())
}
