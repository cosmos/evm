package bank

import (
	"errors"
	"math"
	"math/big"

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
			Contract: contractAddress,
			Amount:   coin.Amount.BigInt(),
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
	ctx sdk.Context,
	args TotalSupplyCall,
) (*TotalSupplyReturn, error) {
	i := 0
	totalSupply := make([]Balance, 0)

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

		totalSupply = append(totalSupply, Balance{
			Contract: contractAddress,
			Amount:   coin.Amount.BigInt(),
		})

		return false
	})

	return &TotalSupplyReturn{totalSupply}, nil
}

// SupplyOf returns the total supply of a given registered erc20 token
// from the x/bank module. If the ERC20 token doesn't have a registered
// TokenPair, the method returns a supply of zero.
// The amount returned with this query has the original decimals precision
// stored in the x/bank.
func (p Precompile) SupplyOf(
	ctx sdk.Context,
	args SupplyOfCall,
) (*SupplyOfReturn, error) {
	tokenPairID := p.erc20Keeper.GetERC20Map(ctx, args.Contract)
	tokenPair, found := p.erc20Keeper.GetTokenPair(ctx, tokenPairID)
	if !found {
		return &SupplyOfReturn{big.NewInt(0)}, nil
	}

	supply := p.bankKeeper.GetSupply(ctx, tokenPair.Denom)

	return &SupplyOfReturn{supply.Amount.BigInt()}, nil
}

func (p Precompile) Name(
	ctx sdk.Context,
	args NameCall,
) (*NameReturn, error) {
	metadata, found := p.bankKeeper.GetDenomMetaData(ctx, args.Denom)
	if !found {
		return nil, ErrDenomNotFound
	}

	return &NameReturn{metadata.Name}, nil
}

func (p Precompile) Symbol(
	ctx sdk.Context,
	args SymbolCall,
) (*SymbolReturn, error) {
	metadata, found := p.bankKeeper.GetDenomMetaData(ctx, args.Denom)
	if !found {
		return nil, ErrDenomNotFound
	}

	return &SymbolReturn{metadata.Symbol}, nil
}

func (p Precompile) Decimals(
	ctx sdk.Context,
	args DecimalsCall,
) (*DecimalsReturn, error) {
	m, found := p.bankKeeper.GetDenomMetaData(ctx, args.Denom)
	if !found {
		return nil, ErrDenomNotFound
	}

	if len(m.DenomUnits) == 0 {
		return &DecimalsReturn{0}, nil
	}

	// look up Display denom unit
	index := -1
	for i, denomUnit := range m.DenomUnits {
		if denomUnit.Denom == m.Display {
			index = i
			break
		}
	}

	var exponent uint32
	if index == -1 {
		exponent = 0
	} else {
		exponent = m.DenomUnits[index].Exponent
	}

	if exponent > math.MaxUint8 {
		return nil, errors.New("exponent too large")
	}

	return &DecimalsReturn{uint8(exponent)}, nil
}

func (p Precompile) TotalSupplyV2(
	ctx sdk.Context,
	args TotalSupply0Call,
) (*TotalSupply0Return, error) {
	supply := p.bankKeeper.GetSupply(ctx, args.Denom)

	return &TotalSupply0Return{supply.Amount.BigInt()}, nil
}

func (p Precompile) BalanceOf(
	ctx sdk.Context,
	args BalanceOfCall,
) (*BalanceOfReturn, error) {
	balance := p.bankKeeper.GetBalance(ctx, args.Account.Bytes(), args.Denom)

	return &BalanceOfReturn{balance.Amount.BigInt()}, nil
}
