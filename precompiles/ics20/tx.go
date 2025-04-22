package ics20

import (
	"fmt"

	errorsmod "cosmossdk.io/errors"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v10/modules/core/24-host"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	sdk "github.com/cosmos/cosmos-sdk/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

const (
	// TransferMethod defines the ABI method name for the ICS20 Transfer
	// transaction.
	TransferMethod = "transfer"
)

// Transfer implements the ICS20 transfer transactions.
func (p *Precompile) Transfer(
	ctx sdk.Context,
	origin common.Address,
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
	} else if v2ClientIdErr := host.ClientIdentifierValidator(msg.SourceChannel); v2ClientIdErr != nil {
		return nil, errorsmod.Wrapf(
			channeltypes.ErrInvalidChannel,
			"invalid channel ID (%s) on v2 packet",
			msg.SourceChannel,
		)
	}

	// isCallerSender is true when the contract caller is the same as the sender
	isCallerSender := contract.CallerAddress == sender

	// If the contract caller is not the same as the sender, the sender must be the origin
	if !isCallerSender && origin != sender {
		return nil, fmt.Errorf(ErrDifferentOriginFromSender, origin.String(), sender.String())
	}

	// no need to have authorization when the contract caller is the same as origin (owner of funds)
	// and the sender is the origin
	resp, expiration, err := CheckAndAcceptAuthorizationIfNeeded(ctx, contract, origin, p.AuthzKeeper, msg)
	if err != nil {
		return nil, err
	}

	res, err := p.transferKeeper.Transfer(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err := UpdateGrantIfNeeded(ctx, contract, p.AuthzKeeper, origin, expiration, resp); err != nil {
		return nil, err
	}

	evmDenom := evmtypes.GetEVMCoinDenom()
	if contract.CallerAddress != origin && msg.Token.Denom == evmDenom {
		// escrow address is also changed on this tx, and it is not a module account
		// so we need to account for this on the UpdateDirties
		escrowAccAddress := transfertypes.GetEscrowAddress(msg.SourcePort, msg.SourceChannel)
		escrowHexAddr := common.BytesToAddress(escrowAccAddress)
		// NOTE: This ensures that the changes in the bank keeper are correctly mirrored to the EVM stateDB
		// when calling the precompile from another smart contract.
		// This prevents the stateDB from overwriting the changed balance in the bank keeper when committing the EVM state.
		amt := msg.Token.Amount.BigInt()
		p.SetBalanceChangeEntries(
			cmn.NewBalanceChangeEntry(sender, amt, cmn.Sub),
			cmn.NewBalanceChangeEntry(escrowHexAddr, amt, cmn.Add),
		)
	}

	if err = EmitIBCTransferEvent(
		ctx,
		stateDB,
		p.ABI.Events[EventTypeIBCTransfer],
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
