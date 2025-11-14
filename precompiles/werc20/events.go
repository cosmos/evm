package werc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/yihuang/go-abi"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EmitDepositEvent creates a new Deposit event emitted after a Deposit transaction.
func (p Precompile) EmitDepositEvent(
	ctx sdk.Context,
	stateDB vm.StateDB,
	caller common.Address,
	amount *big.Int,
) error {
	// Create the event using the generated constructor
	topics, data, err := abi.EncodeEvent(NewDepositEvent(caller, amount))
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}

// EmitWithdrawalEvent creates a new Withdrawal event emitted after a Withdraw transaction.
func (p Precompile) EmitWithdrawalEvent(
	ctx sdk.Context,
	stateDB vm.StateDB,
	src common.Address,
	amount *big.Int,
) error {
	// Create the event using the generated constructor
	topics, data, err := abi.EncodeEvent(NewWithdrawalEvent(src, amount))
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        data,
		BlockNumber: uint64(ctx.BlockHeight()), //nolint:gosec // G115
	})

	return nil
}
