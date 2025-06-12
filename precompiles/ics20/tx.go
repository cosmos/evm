package ics20

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v10/modules/core/24-host"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// TransferMethod defines the ABI method name for the ICS20 Transfer
	// transaction.
	TransferMethod = "transfer"
)

// Transfer implements the ICS20 transfer transactions.
func (p *Precompile) Transfer(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, sender, err := NewMsgTransfer(method, args)
	if err != nil {
		return nil, err
	}

	// If the channel is in v1 format, check if channel exists and is open
	if channeltypes.IsChannelIDFormat(msg.SourceChannel) {
		// check if channel exists and is open
		hasV1Channel := p.channelKeeper.HasChannel(ctx, msg.SourcePort, msg.SourceChannel)
		if !hasV1Channel {
			return nil, errorsmod.Wrapf(
				channeltypes.ErrChannelNotFound,
				"port ID (%s) channel ID (%s)",
				msg.SourcePort,
				msg.SourceChannel,
			)
		}
		// otherwise, it’s a v2 packet, so perform client ID validation
	} else if v2ClientIDErr := host.ClientIdentifierValidator(msg.SourceChannel); v2ClientIDErr != nil {
		return nil, errorsmod.Wrapf(
			channeltypes.ErrInvalidChannel,
			"invalid channel ID (%s) on v2 packet",
			msg.SourceChannel,
		)
	}

	msgSender := contract.Caller()
	if msgSender != sender {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), sender.String())
	}

	res, err := p.transferKeeper.Transfer(ctx, msg)
	if err != nil {
		return nil, err
	}

	evmDenom := evmtypes.GetEVMCoinDenom()
	if msg.Token.Denom == evmDenom {
		// escrow address is also changed on this tx, and it is not a module account
		// so we need to account for this on the UpdateDirties
		escrowAccAddress := transfertypes.GetEscrowAddress(msg.SourcePort, msg.SourceChannel)
		escrowHexAddr := common.BytesToAddress(escrowAccAddress)
		// NOTE: This ensures that the changes in the bank keeper are correctly mirrored to the EVM stateDB
		// when calling the precompile from another smart contract.
		// This prevents the stateDB from overwriting the changed balance in the bank keeper when committing the EVM state.
		amt, err := utils.Uint256FromBigInt(msg.Token.Amount.BigInt())
		if err != nil {
			return nil, err
		}
		p.SetBalanceChangeEntries(
			cmn.NewBalanceChangeEntry(sender, amt, cmn.Sub),
			cmn.NewBalanceChangeEntry(escrowHexAddr, amt, cmn.Add),
		)
	}

	if err = EmitIBCTransferEvent(
		ctx,
		stateDB,
		p.Events[EventTypeIBCTransfer],
		p.Address(),
		sender,
		msg.Receiver,
		msg.SourcePort,
		msg.SourceChannel,
		msg.Token,
		msg.Memo,
	); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(res.Sequence)
}
