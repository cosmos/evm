package slashing

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/x/vm/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	"github.com/cosmos/cosmos-sdk/x/slashing/types"
)

const (
	// UnjailMethod defines the ABI method name for the slashing Unjail
	// transaction.
	UnjailMethod = "unjail"
)

// Unjail implements the unjail precompile transaction, which allows validators
// to unjail themselves after being jailed for downtime.
func (p Precompile) Unjail(
	ctx sdk.Context,
	method *abi.Method,
	stateDB vm.StateDB,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	validatorAddress, ok := args[0].(common.Address)
	if !ok {
		return nil, fmt.Errorf("invalid validator hex address")
	}

	msg := &types.MsgUnjail{
		ValidatorAddr: sdk.ValAddress(validatorAddress.Bytes()).String(),
	}

	msgSrv := slashingkeeper.NewMsgServerImpl(p.slashingKeeper)
	if _, err := msgSrv.Unjail(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitValidatorUnjailedEvent(ctx, stateDB, validatorAddress); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
