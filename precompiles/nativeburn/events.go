package nativeburn

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const EventTypeTokenBurned = "TokenBurned"

func (p Precompile) EmitTokenBurnedEvent(
	ctx sdk.Context,
	stateDB vm.StateDB,
	burner common.Address,
	amount *big.Int,
) error {
	event := p.Events[EventTypeTokenBurned]

	topics := make([]common.Hash, 2)
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(burner)
	if err != nil {
		return err
	}

	arguments := abi.Arguments{event.Inputs[1]}
	packed, err := arguments.Pack(amount)
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        packed,
		BlockNumber: uint64(ctx.BlockHeight()),
	})

	return nil
}
