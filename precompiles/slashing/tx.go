package slashing

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
	contract *vm.Contract,
	args []interface{},
) ([]byte, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	validatorAddress, ok := args[0].(common.Address)
	if !ok {
		return nil, fmt.Errorf("invalid validator hex address")
	}

	msgSender := contract.Caller()
	if msgSender != validatorAddress {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), validatorAddress.String())
	}

	valAddr, err := p.valCodec.BytesToString(validatorAddress.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to convert validator address: %w", err)
	}

	msg := &types.MsgUnjail{
		ValidatorAddr: valAddr,
	}

	if _, err := p.slashingMsgServer.Unjail(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitValidatorUnjailedEvent(ctx, stateDB, validatorAddress); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
