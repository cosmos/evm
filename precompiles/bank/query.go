package bank

import (
	"math/big"

	"github.com/yihuang/go-abi"

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

// Balances returns given account's balances of all tokens registered in the x/bank module
// and the corresponding ERC20 address (address, amount). The amount returned for each token
// has the original decimals precision stored in the x/bank.
// This method charges the account the corresponding value of an ERC-20
// balanceOf call for each token returned.
func (p Precompile) Balances(
	ctx sdk.Context,
	args BalancesCall,
) (*BalancesReturn, error) {
	i := 0
	balances := make([]Balance, 0)

	p.bankKeeper.IterateAccountBalances(ctx, args.Account.Bytes(), func(coin sdk.Coin) bool {
		defer func() { i++ }()

		// NOTE: we already charged for a single balanceOf request so we don't
		// need to charge on the first iteration
		if i > 0 {
			ctx.GasMeter().ConsumeGas(GasBalances, "ERC-20 extension balances method")
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

	return &BalancesReturn{balances}, nil
}

// TotalSupply returns the total supply of all tokens registered in the x/bank
// module. The amount returned for each token has the original
// decimals precision stored in the x/bank.
// This method charges the account the corresponding value of a ERC-20 totalSupply
// call for each token returned.
func (p Precompile) TotalSupply(
	ctx sdk.Context, _ abi.EmptyTuple,
) (TotalSupplyReturn, error) {
	i := 0
	balances := make([]Balance, 0)

	p.bankKeeper.IterateTotalSupply(ctx, func(coin sdk.Coin) bool {
		defer func() { i++ }()

		// NOTE: we already charged for a single totalSupply request so we don't
		// need to charge on the first iteration
		if i > 0 {
			ctx.GasMeter().ConsumeGas(GasTotalSupply, "ERC-20 extension totalSupply method")
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

	return TotalSupplyReturn{balances}, nil
}

// SupplyOf returns the total supply of a given registered erc20 token
// from the x/bank module. If the ERC20 token doesn't have a registered
// TokenPair, the method returns a supply of zero.
// The amount returned with this query has the original decimals precision
// stored in the x/bank.
func (p Precompile) SupplyOf(
	ctx sdk.Context,
	args SupplyOfCall,
) (SupplyOfReturn, error) {
	tokenPairID := p.erc20Keeper.GetERC20Map(ctx, args.Erc20Address)
	tokenPair, found := p.erc20Keeper.GetTokenPair(ctx, tokenPairID)
	if !found {
		return SupplyOfReturn{big.NewInt(0)}, nil
	}

	supply := p.bankKeeper.GetSupply(ctx, tokenPair.Denom)
	return SupplyOfReturn{supply.Amount.BigInt()}, nil
}
