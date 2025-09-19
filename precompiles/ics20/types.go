package ics20

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	errorsmod "cosmossdk.io/errors"
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

// EventIBCTransfer is the event type emitted when a transfer is executed.
type EventIBCTransfer struct {
	Sender        common.Address
	Receiver      common.Hash
	SourcePort    string
	SourceChannel string
	Denom         string
	Amount        *big.Int
	Memo          string
}

// EventTransferAuthorization is the event type emitted when a transfer authorization is created.
type EventTransferAuthorization struct {
	Grantee     common.Address
	Granter     common.Address
	Allocations []cmn.ICS20Allocation
}

// DenomResponse defines the data for the denom response.
type DenomResponse struct {
	Denom transfertypes.Denom
}

// PageRequest defines the data for the page request.
type PageRequest struct {
	PageRequest query.PageRequest
}

// DenomsResponse defines the data for the denoms response.
type DenomsResponse struct {
	Denoms       []transfertypes.Denom
	PageResponse query.PageResponse
}

// height is a struct used to parse the TimeoutHeight parameter
// used as input in the transfer method
type height struct {
	TimeoutHeight clienttypes.Height
}

// NewMsgTransfer returns a new transfer message from the given arguments.
func NewMsgTransfer(method *abi.Method, args []interface{}) (*transfertypes.MsgTransfer, common.Address, error) {
	if len(args) != 9 {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 9, len(args))
	}

	sourcePort, ok := args[0].(string)
	if !ok {
		return nil, common.Address{}, errors.New(ErrInvalidSourcePort)
	}

	sourceChannel, ok := args[1].(string)
	if !ok {
		return nil, common.Address{}, errors.New(ErrInvalidSourceChannel)
	}

	denom, ok := args[2].(string)
	if !ok {
		return nil, common.Address{}, errorsmod.Wrapf(transfertypes.ErrInvalidDenomForTransfer, cmn.ErrInvalidDenom, args[2])
	}

	amount, ok := args[3].(*big.Int)
	if !ok || amount == nil {
		return nil, common.Address{}, errorsmod.Wrapf(transfertypes.ErrInvalidAmount, cmn.ErrInvalidAmount, args[3])
	}

	sender, ok := args[4].(common.Address)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(ErrInvalidSender, args[4])
	}

	receiver, ok := args[5].(string)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(ErrInvalidReceiver, args[5])
	}

	var input height
	heightArg := abi.Arguments{method.Inputs[6]}
	if err := heightArg.Copy(&input, []interface{}{args[6]}); err != nil {
		return nil, common.Address{}, fmt.Errorf("error while unpacking args to TransferInput struct: %s", err)
	}

	timeoutTimestamp, ok := args[7].(uint64)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(ErrInvalidTimeoutTimestamp, args[7])
	}

	memo, ok := args[8].(string)
	if !ok {
		return nil, common.Address{}, fmt.Errorf(ErrInvalidMemo, args[8])
	}

	// Use instance to prevent errors on denom or amount
	token := sdk.Coin{
		Denom:  denom,
		Amount: math.NewIntFromBigInt(amount),
	}

	msg, err := CreateAndValidateMsgTransfer(sourcePort, sourceChannel, token, sdk.AccAddress(sender.Bytes()).String(), receiver, input.TimeoutHeight, timeoutTimestamp, memo)
	if err != nil {
		return nil, common.Address{}, err
	}

	return msg, sender, nil
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
func NewDenomRequest(args []interface{}) (*transfertypes.QueryDenomRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("invalid input arguments. Expected 1, got %d", len(args))
	}

	hash, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf(ErrInvalidHash, args[0])
	}

	req := &transfertypes.QueryDenomRequest{
		Hash: hash,
	}

	return req, nil
}

// NewDenomsRequest returns a new denoms request from the given arguments.
func NewDenomsRequest(method *abi.Method, args []interface{}) (*transfertypes.QueryDenomsRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	var pageRequest PageRequest
	if err := safeCopyInputs(method, args, &pageRequest); err != nil {
		return nil, fmt.Errorf("error while unpacking args to PageRequest: %w", err)
	}

	req := &transfertypes.QueryDenomsRequest{
		Pagination: &pageRequest.PageRequest,
	}

	return req, nil
}

// NewDenomHashRequest returns a new denom hash request from the given arguments.
func NewDenomHashRequest(args []interface{}) (*transfertypes.QueryDenomHashRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("invalid input arguments. Expected 1, got %d", len(args))
	}

	trace, ok := args[0].(string)
	if !ok {
		return nil, fmt.Errorf("invalid trace")
	}

	req := &transfertypes.QueryDenomHashRequest{
		Trace: trace,
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

// safeCopyInputs is a helper function to safely copy inputs from the method to the args.
// It recovers from any panic that might occur during the copy operation and returns an error instead.
func safeCopyInputs(method *abi.Method, args []interface{}, pageRequest *PageRequest) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during method.Inputs.Copy: %v", r)
		}
	}()
	err = method.Inputs.Copy(pageRequest, args)
	return err
}
