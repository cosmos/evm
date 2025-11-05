package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

func NewAllowance(erc20 common.Address, owner common.Address, spender common.Address, value *big.Int) Allowance {
	return Allowance{
		Erc20Address: erc20.Hex(),
		Owner:        owner.Hex(),
		Spender:      spender.Hex(),
		Value:        math.NewIntFromBigInt(value),
	}
}

func (a Allowance) Validate() error {
	if !common.IsHexAddress(a.Erc20Address) {
		return errorsmod.Wrapf(errortypes.ErrInvalidAddress, "invalid erc20 hex address %s", a.Erc20Address)
	}

	if !common.IsHexAddress(a.Owner) {
		return errorsmod.Wrapf(errortypes.ErrInvalidAddress, "invalid owner hex address %s", a.Owner)
	}

	if !common.IsHexAddress(a.Spender) {
		return errorsmod.Wrapf(errortypes.ErrInvalidAddress, "invalid sender hex address %s", a.Spender)
	}

	if !a.Value.IsPositive() {
		return errorsmod.Wrap(ErrInvalidAllowance, "invalid allowance value")
	}

	return nil
}
