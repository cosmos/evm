package types

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// AccountKeeper defines the contract required for account APIs.
type AccountKeeper interface {
	NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI
	SetAccount(ctx context.Context, acc sdk.AccountI)
}

// EVMKeeper defines the expected EVM keeper interface used on erc20
type EVMKeeper interface {
	CallEVM(ctx sdk.Context, abi abi.ABI, from, contract common.Address, commit bool, method string, args ...interface{}) (*evmtypes.MsgEthereumTxResponse, error)
	CallEVMWithData(ctx sdk.Context, from common.Address, contract *common.Address, data []byte, commit bool) (*evmtypes.MsgEthereumTxResponse, error)
}
