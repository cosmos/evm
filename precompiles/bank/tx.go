package bank

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/core/vm"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func (p Precompile) TransferFrom(
	ctx sdk.Context,
	args TransferFromCall,
	stateDB vm.StateDB,
	contract *vm.Contract,
) (*TransferFromReturn, error) {
	// don't handle gas token here
	if args.Denom == evmtypes.GetEVMCoinDenom() {
		return nil, errors.New("cannot transfer gas token with bank precompile")
	}

	// authorization: only from address or deterministic erc20 contract address can call this method
	caller := contract.Caller()
	erc20, err := ERC20ContractAddress(p.Address(), args.Denom)
	if err != nil {
		return nil, fmt.Errorf("failed to get erc20 contract address: %w", err)
	}
	if caller != args.From && caller != erc20 {
		return nil, ErrUnauthorized
	}

	coins := sdk.Coins{{Denom: args.Denom, Amount: sdkmath.NewIntFromBigInt(args.Value)}}
	if err := coins.Validate(); err != nil {
		return nil, fmt.Errorf("invalid coins: %w", err)
	}

	// execute the transfer with bank keeper
	msg := banktypes.NewMsgSend(args.From.Bytes(), args.To.Bytes(), coins)
	if _, err := p.bankMsgServer.Send(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to send coins: %w", err)
	}

	return &TransferFromReturn{true}, nil
}
