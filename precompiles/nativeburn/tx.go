package nativeburn

import (
	"fmt"
	"math/big"

	"cosmossdk.io/math"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	BurnTokenMethod = "burnToken"
	ModuleName      = "nativeburn"
)

func (p *Precompile) BurnToken(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	burnerAddr, amount, err := parseTokenBurnedArgs(args)
	if err != nil {
		return nil, err
	}

	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}

	coins := sdk.NewCoins(sdk.NewCoin(bondDenom, math.NewIntFromBigInt(amount)))
	burnerAccAddr := sdk.AccAddress(burnerAddr.Bytes())

	// Step 1: Send to module account
	err = p.bankKeeper.SendCoinsFromAccountToModule(ctx, burnerAccAddr, ModuleName, coins)
	if err != nil {
		return nil, fmt.Errorf("failed to send coins to module: %w", err)
	}

	// Step 2: PERMANENTLY BURN - reduces total supply
	err = p.bankKeeper.BurnCoins(ctx, ModuleName, coins)
	if err != nil {
		return nil, fmt.Errorf("failed to burn coins: %w", err)
	}

	if err := p.EmitTokenBurnedEvent(ctx, stateDB, burnerAddr, amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func parseTokenBurnedArgs(args []interface{}) (common.Address, *big.Int, error) {
	if len(args) != 2 {
		return common.Address{}, nil, fmt.Errorf("invalid number of arguments; expected 2, got %d", len(args))
	}

	burnerAddr, ok := args[0].(common.Address)
	if !ok {
		return common.Address{}, nil, fmt.Errorf("invalid burner address type")
	}

	if burnerAddr == (common.Address{}) {
		return common.Address{}, nil, fmt.Errorf("burner address cannot be zero")
	}

	amount, ok := args[1].(*big.Int)
	if !ok {
		return common.Address{}, nil, fmt.Errorf("invalid amount type")
	}

	if amount.Sign() <= 0 {
		return common.Address{}, nil, fmt.Errorf("amount must be positive")
	}

	return burnerAddr, amount, nil
}
