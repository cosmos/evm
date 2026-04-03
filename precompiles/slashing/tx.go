package slashing

import (
	"fmt"
	"math/big"

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
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrInvalidNumberOfArgs, big.NewInt(1), big.NewInt(int64(len(args))))
	}

	validatorAddress, ok := args[0].(common.Address)
	if !ok {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrInvalidAddress, fmt.Sprintf("%v", args[0]))
	}

	msgSender := contract.Caller()
	if msgSender != validatorAddress {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, validatorAddress)
	}

	valAddr, err := p.valCodec.BytesToString(validatorAddress.Bytes())
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrInvalidAddress, err.Error())
	}

	msg := &types.MsgUnjail{
		ValidatorAddr: valAddr,
	}

	if _, err := p.slashingMsgServer.Unjail(ctx, msg); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, UnjailMethod, err.Error())
	}

	if err := p.EmitValidatorUnjailedEvent(ctx, stateDB, validatorAddress); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, UnjailMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}
