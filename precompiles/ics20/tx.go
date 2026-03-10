package ics20

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/hashicorp/go-metrics"

	cmn "github.com/cosmos/evm/precompiles/common"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	callbackstypes "github.com/cosmos/evm/x/ibc/callbacks/types"
	"github.com/cosmos/evm/x/vm/statedb"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v10/modules/core/24-host"

	errorsmod "cosmossdk.io/errors"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TODO TEST suite for precompile

const (
	// TransferMethod defines the ABI method name for the ICS20 Transfer
	// transaction.
	TransferMethod = "transfer"
)

// validateV1TransferChannel does the following validation on an ibc v1 channel specified in a MsgTransfer:
// - check if the channel exists
// - check if the channel is OPEN
// - check if the underlying connection exists
// - check if the underlying connection is OPEN
func (p *Precompile) validateV1TransferChannel(ctx sdk.Context, msg *transfertypes.MsgTransfer) error {
	if msg == nil {
		return fmt.Errorf("msg cannot be nil")
	}

	if err := msg.ValidateBasic(); err != nil {
		return fmt.Errorf("msg invalid: %w", err)
	}

	// check if channel exists and is open
	channel, found := p.channelKeeper.GetChannel(ctx, msg.SourcePort, msg.SourceChannel)
	if !found {
		return errorsmod.Wrapf(
			channeltypes.ErrChannelNotFound,
			"port ID (%s) channel ID (%s)",
			msg.SourcePort,
			msg.SourceChannel,
		)
	}
	if err := channel.ValidateBasic(); err != nil {
		return fmt.Errorf("channel invalid: %w", err)
	}

	// Validate channel is in OPEN state
	if channel.State != channeltypes.OPEN {
		return errorsmod.Wrapf(
			channeltypes.ErrInvalidChannelState,
			"channel (%s) is not open, current state: %s",
			msg.SourceChannel,
			channel.State.String(),
		)
	}

	// Validate underlying connection exists and is active
	connection, err := p.channelKeeper.GetConnection(ctx, channel.ConnectionHops[0])
	if err != nil {
		return errorsmod.Wrapf(
			err,
			"connection (%s) not found for channel (%s)",
			channel.ConnectionHops[0],
			msg.SourceChannel,
		)
	}

	// Validate connection is in OPEN state
	if connection.State != connectiontypes.OPEN {
		return errorsmod.Wrapf(
			connectiontypes.ErrInvalidConnectionState,
			"connection (%s) is not open, current state: %s",
			channel.ConnectionHops[0],
			connection.State.String(),
		)
	}

	return nil
}

// transferWithStateDB handles IBC transfers with ERC20 token conversion support.
// If user doesn't have enough balance of coin, it will attempt to convert
// ERC20 tokens to the coin denomination, and continue with a regular transfer.
func (p *Precompile) transferWithStateDB(ctx sdk.Context, stateDB *statedb.StateDB, msg *transfertypes.MsgTransfer) (*transfertypes.MsgTransferResponse, error) {
	// Temporarily save the KV and transient KV gas config. To avoid extra costs for relayers
	// these two gas config are replaced with empty one and should be restored before exiting this function.
	kvGasCfg := ctx.KVGasConfig()
	transientKVGasCfg := ctx.TransientKVGasConfig()
	ctx = ctx.
		WithKVGasConfig(storetypes.GasConfig{}).
		WithTransientKVGasConfig(storetypes.GasConfig{})

	defer func() {
		// Return the KV gas config to initial values
		ctx = ctx.
			WithKVGasConfig(kvGasCfg).
			WithTransientKVGasConfig(transientKVGasCfg)
	}()

	// use native denom or contract address
	denom := strings.TrimPrefix(msg.Token.Denom, erc20types.Erc20NativeCoinDenomPrefix)

	pairID := p.erc20Keeper.GetTokenPairID(ctx, denom)
	if len(pairID) == 0 {
		// no-op: token is not registered so we can proceed with regular transfer
		return p.transferKeeper.Transfer(ctx, msg)
	}

	pair, _ := p.erc20Keeper.GetTokenPair(ctx, pairID)
	if !pair.Enabled {
		// no-op: pair is not enabled so we can proceed with regular transfer
		return p.transferKeeper.Transfer(ctx, msg)
	}

	sender := sdk.MustAccAddressFromBech32(msg.Sender)

	if !p.erc20Keeper.IsERC20Enabled(ctx) {
		// no-op: continue with regular transfer
		return p.transferKeeper.Transfer(ctx, msg)
	}

	// update the msg denom to the token pair denom
	msg.Token.Denom = pair.Denom

	if !pair.IsNativeERC20() {
		return p.transferKeeper.Transfer(ctx, msg)
	}
	// if the user has enough balance of the Cosmos representation, then we don't need to Convert
	balance := p.bankKeeper.SpendableCoin(ctx, sender, pair.Denom)
	if balance.Amount.GTE(msg.Token.Amount) {

		defer func() {
			telemetry.IncrCounterWithLabels(
				[]string{"erc20", "ibc", "transfer", "total"},
				1,
				[]metrics.Label{
					telemetry.NewLabel("denom", pair.Denom),
				},
			)
		}()

		return p.transferKeeper.Transfer(ctx, msg)
	}

	// Only convert if the pair is a native ERC20
	// only convert the remaining difference
	difference := msg.Token.Amount.Sub(balance.Amount)

	// Convert the ERC20 tokens to Cosmos IBC Coin
	erc20Sender := common.BytesToAddress(sender.Bytes())
	if _, err := p.erc20Keeper.ConvertERC20IntoCoinsForNativeToken(ctx, stateDB, pair.GetERC20Contract(), difference, sender, erc20Sender, true, true); err != nil {
		return nil, err
	}

	defer func() {
		telemetry.IncrCounterWithLabels(
			[]string{"erc20", "ibc", "transfer", "total"},
			1,
			[]metrics.Label{
				telemetry.NewLabel("denom", pair.Denom),
			},
		)
	}()

	return p.transferKeeper.Transfer(ctx, msg)
}

// Transfer implements the ICS20 transfer transactions.
func (p *Precompile) Transfer(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	inSourceCallback := callbackstypes.IsSourceCallbackExecution(ctx)
	if !inSourceCallback {
		if stateDBExp, ok := stateDB.(*statedb.StateDB); ok {
			inSourceCallback = callbackstypes.IsSourceCallbackExecution(stateDBExp.GetContext())
		}
	}

	if inSourceCallback {
		return nil, errorsmod.Wrap(channeltypes.ErrInvalidChannelState, ErrTransferBlockedInSourceCallback)
	}

	msg, sender, err := NewMsgTransfer(method, args)
	if err != nil {
		return nil, err
	}

	// If the channel is in v1 format, check if channel exists and is open
	if channeltypes.IsChannelIDFormat(msg.SourceChannel) {
		if err := p.validateV1TransferChannel(ctx, msg); err != nil {
			return nil, err
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

	stateDBExp := stateDB.(*statedb.StateDB)
	res, err := p.transferWithStateDB(ctx, stateDBExp, msg)
	if err != nil {
		return nil, err
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
