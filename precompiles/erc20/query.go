package erc20

import (
	"fmt"
	"math"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/ibc"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// NameMethod defines the ABI method name for the ERC-20 Name
	// query.
	NameMethod = "name"
	// SymbolMethod defines the ABI method name for the ERC-20 Symbol
	// query.
	SymbolMethod = "symbol"
	// DecimalsMethod defines the ABI method name for the ERC-20 Decimals
	// query.
	DecimalsMethod = "decimals"
	// TotalSupplyMethod defines the ABI method name for the ERC-20 TotalSupply
	// query.
	TotalSupplyMethod = "totalSupply"
	// BalanceOfMethod defines the ABI method name for the ERC-20 BalanceOf
	// query.
	BalanceOfMethod = "balanceOf"
	// AllowanceMethod defines the ABI method name for the Allowance
	// query.
	AllowanceMethod = "allowance"
)

// Name returns the name of the token. If the token metadata is registered in the
// bank module, it returns its name. Otherwise, it returns the base denomination of
// the token capitalized (e.g. uatom -> Atom).
func (p Precompile) Name(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	_ []interface{},
) ([]byte, error) {
	metadata, found := p.BankKeeper.GetDenomMetaData(ctx, p.tokenPair.Denom)
	if found {
		return method.Outputs.Pack(metadata.Name)
	}

	baseDenom, err := p.getBaseDenomFromIBCVoucher(ctx, p.tokenPair.Denom)
	if err != nil {
		return nil, ConvertErrToERC20Error(err)
	}

	name := strings.ToUpper(string(baseDenom[1])) + baseDenom[2:]
	return method.Outputs.Pack(name)
}

// Symbol returns the symbol of the token. If the token metadata is registered in the
// bank module, it returns its symbol. Otherwise, it returns the base denomination of
// the token in uppercase (e.g. uatom -> ATOM).
func (p Precompile) Symbol(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	_ []interface{},
) ([]byte, error) {
	metadata, found := p.BankKeeper.GetDenomMetaData(ctx, p.tokenPair.Denom)
	if found {
		return method.Outputs.Pack(metadata.Symbol)
	}

	baseDenom, err := p.getBaseDenomFromIBCVoucher(ctx, p.tokenPair.Denom)
	if err != nil {
		return nil, ConvertErrToERC20Error(err)
	}

	symbol := strings.ToUpper(baseDenom[1:])
	return method.Outputs.Pack(symbol)
}

// Decimals returns the decimals places of the token. If the token metadata is registered in the
// bank module, it returns the display denomination exponent. Otherwise, it infers the decimal
// value from the first character of the base denomination (e.g. uatom -> 6).
func (p Precompile) Decimals(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	_ []interface{},
) ([]byte, error) {
	metadata, found := p.BankKeeper.GetDenomMetaData(ctx, p.tokenPair.Denom)
	if !found {
		denom, err := ibc.GetDenom(p.transferKeeper, ctx, p.tokenPair.Denom)
		if err != nil {
			return nil, ConvertErrToERC20Error(err)
		}

		// we assume the decimal from the first character of the denomination
		decimals, err := ibc.DeriveDecimalsFromDenom(denom.Base)
		if err != nil {
			return nil, ConvertErrToERC20Error(err)
		}
		return method.Outputs.Pack(decimals)
	}

	var (
		decimals     uint32
		displayFound bool
	)
	for i := len(metadata.DenomUnits) - 1; i >= 0; i-- {
		var match bool
		if strings.HasPrefix(metadata.Base, "ibc/") {
			displays := strings.Split(metadata.Display, "/")
			match = metadata.DenomUnits[i].Denom == displays[len(displays)-1]
		} else {
			match = metadata.DenomUnits[i].Denom == metadata.Display
		}
		if match {
			decimals = metadata.DenomUnits[i].Exponent
			displayFound = true
			break
		}
	}

	if !displayFound {
		return nil, ConvertErrToERC20Error(fmt.Errorf(
			"display denomination not found for denom: %q",
			p.tokenPair.Denom,
		))
	}

	if decimals > math.MaxUint8 {
		return nil, ConvertErrToERC20Error(fmt.Errorf(
			"uint8 overflow: invalid decimals: %d",
			decimals,
		))
	}

	return method.Outputs.Pack(uint8(decimals)) //#nosec G115 // we are checking for overflow above
}

// TotalSupply returns the amount of tokens in existence. It fetches the supply
// of the coin from the bank keeper and returns zero if not found.
func (p Precompile) TotalSupply(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	_ []interface{},
) ([]byte, error) {
	supply := p.BankKeeper.GetSupply(ctx, p.tokenPair.Denom)

	return method.Outputs.Pack(supply.Amount.BigInt())
}

// BalanceOf returns the amount of tokens owned by account. It fetches the balance
// of the coin from the bank keeper and returns zero if not found.
func (p Precompile) BalanceOf(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	account, err := ParseBalanceOfArgs(args)
	if err != nil {
		return nil, err
	}

	balance := p.BankKeeper.SpendableCoin(ctx, account.Bytes(), p.tokenPair.Denom)

	return method.Outputs.Pack(balance.Amount.BigInt())
}

// Allowance returns the remaining allowance of a spender for a given owner.
func (p Precompile) Allowance(
	ctx sdk.Context,
	_ *vm.Contract,
	_ vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	owner, spender, err := ParseAllowanceArgs(args)
	if err != nil {
		return nil, err
	}

	allowance, err := p.erc20Keeper.GetAllowance(ctx, p.Address(), owner, spender)
	if err != nil {
		// NOTE: We are not returning the error here, because we want to align the behavior with
		// standard ERC20 smart contracts, which return zero if an allowance is not found.
		allowance = common.Big0
	}

	return method.Outputs.Pack(allowance)
}

// getBaseDenomFromIBCVoucher returns the base denomination from the given IBC voucher denomination.
func (p Precompile) getBaseDenomFromIBCVoucher(ctx sdk.Context, voucherDenom string) (string, error) {
	// Infer the denomination name from the coin denomination base voucherDenom
	denom, err := ibc.GetDenom(p.transferKeeper, ctx, voucherDenom)
	if err != nil {
		// FIXME: return 'not supported' (same error as when you call the method on an ERC20.sol)
		return "", err
	}

	// safety check
	if len(denom.Base) < 3 {
		// FIXME: return not supported (same error as when you call the method on an ERC20.sol)
		return "", fmt.Errorf("invalid base denomination; should be at least length 3; got: %q", denom.Base)
	}

	return denom.Base, nil
}
