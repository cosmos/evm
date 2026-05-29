package types

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"
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
	CallEVM(ctx sdk.Context, abi abi.ABI, from, contract common.Address, commit bool, gasCap *big.Int, method string, args ...interface{}) (*evmtypes.MsgEthereumTxResponse, error)
	CallEVMWithData(ctx sdk.Context, from common.Address, contract *common.Address, data []byte, commit bool, gasCap *big.Int) (*evmtypes.MsgEthereumTxResponse, error)
	GetAccountOrEmpty(ctx sdk.Context, addr common.Address) statedb.Account
	GetAccount(ctx sdk.Context, addr common.Address) *statedb.Account
	IsContract(ctx sdk.Context, addr common.Address) bool
}

type ERC20Keeper interface {
	GetTokenPairID(ctx sdk.Context, token string) []byte
	GetTokenPair(ctx sdk.Context, id []byte) (types.TokenPair, bool)
	SetAllowance(ctx sdk.Context, erc20 common.Address, owner common.Address, spender common.Address, value *big.Int) error
	BalanceOf(ctx sdk.Context, abi abi.ABI, contract, account common.Address) *big.Int
}
