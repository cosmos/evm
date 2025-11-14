package ics20

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

const (
	// DefaultRevisionNumber is the default value used to not set a timeout revision number
	DefaultRevisionNumber = 0

	// DefaultRevisionHeight is the default value used to not set a timeout revision height
	DefaultRevisionHeight = 0

	// DefaultTimeoutMinutes is the default value in minutes used to set a timeout timestamp
	DefaultTimeoutMinutes = 10
)

// DefaultTimeoutHeight is the default value used to set a timeout height
var DefaultTimeoutHeight = clienttypes.NewHeight(DefaultRevisionNumber, DefaultRevisionHeight)

// NewMsgTransfer returns a new transfer message from the given arguments.
func NewMsgTransfer(args TransferCall) (*transfertypes.MsgTransfer, common.Address, error) {
	// Use instance to prevent errors on denom or amount
	token := sdk.Coin{
		Denom:  args.Denom,
		Amount: math.NewIntFromBigInt(args.Amount),
	}

	timeoutHeight := clienttypes.NewHeight(args.TimeoutHeight.RevisionNumber, args.TimeoutHeight.RevisionHeight)

	msg, err := CreateAndValidateMsgTransfer(args.SourcePort, args.SourceChannel, token, sdk.AccAddress(args.Sender.Bytes()).String(), args.Receiver, timeoutHeight, args.TimeoutTimestamp, args.Memo)
	if err != nil {
		return nil, common.Address{}, err
	}

	return msg, args.Sender, nil
}

// CreateAndValidateMsgTransfer creates a new MsgTransfer message and run validate basic.
func CreateAndValidateMsgTransfer(
	sourcePort, sourceChannel string,
	coin sdk.Coin, senderAddress, receiverAddress string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
	memo string,
) (*transfertypes.MsgTransfer, error) {
	msg := transfertypes.NewMsgTransfer(
		sourcePort,
		sourceChannel,
		coin,
		senderAddress,
		receiverAddress,
		timeoutHeight,
		timeoutTimestamp,
		memo,
	)

	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	return msg, nil
}

// NewDenomRequest returns a new denom request from the given arguments.
func NewDenomRequest(args DenomCall) (*transfertypes.QueryDenomRequest, error) {
	req := &transfertypes.QueryDenomRequest{
		Hash: args.Hash,
	}

	return req, nil
}

// NewDenomsRequest returns a new denoms request from the given arguments.
func NewDenomsRequest(args DenomsCall) (*transfertypes.QueryDenomsRequest, error) {
	req := &transfertypes.QueryDenomsRequest{
		Pagination: &query.PageRequest{
			Key:        args.PageRequest.Key,
			Offset:     args.PageRequest.Offset,
			Limit:      args.PageRequest.Limit,
			CountTotal: args.PageRequest.CountTotal,
			Reverse:    args.PageRequest.Reverse,
		},
	}

	return req, nil
}

// NewDenomHashRequest returns a new denom hash request from the given arguments.
func NewDenomHashRequest(args DenomHashCall) (*transfertypes.QueryDenomHashRequest, error) {
	req := &transfertypes.QueryDenomHashRequest{
		Trace: args.Trace,
	}

	return req, nil
}

// CheckOriginAndSender ensures the correct sender is being used.
func CheckOriginAndSender(contract *vm.Contract, origin common.Address, sender common.Address) (common.Address, error) {
	if contract.Caller() == sender {
		return sender, nil
	} else if origin != sender {
		return common.Address{}, fmt.Errorf(ErrDifferentOriginFromSender, origin.String(), sender.String())
	}
	return sender, nil
}
