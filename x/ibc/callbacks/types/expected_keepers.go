package types

import (
	"context"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EVMKeeper defines the expected EVM keeper interface used on erc20
type EVMKeeper interface {
	// TODO: should these methods also be converted to use context.Context?
	GetParams(ctx sdk.Context) evmtypes.Params
	GetAccountWithoutBalance(ctx sdk.Context, addr common.Address) *statedb.Account
	EstimateGasInternal(c context.Context, req *evmtypes.EthCallRequest, fromType evmtypes.CallType) (*evmtypes.EstimateGasResponse, error)
	ApplyMessage(ctx sdk.Context, msg core.Message, tracer vm.EVMLogger, commit bool) (*evmtypes.MsgEthereumTxResponse, error)
	DeleteAccount(ctx sdk.Context, addr common.Address) error
	IsAvailableStaticPrecompile(params *evmtypes.Params, address common.Address) bool
	CallEVM(ctx sdk.Context, abi abi.ABI, from, contract common.Address, commit bool, method string, args ...interface{}) (*evmtypes.MsgEthereumTxResponse, error)
	CallEVMWithData(ctx sdk.Context, from common.Address, contract *common.Address, data []byte, commit bool) (*evmtypes.MsgEthereumTxResponse, error)
	GetCode(ctx sdk.Context, hash common.Hash) []byte
	SetCode(ctx sdk.Context, hash []byte, bytecode []byte)
	SetAccount(ctx sdk.Context, address common.Address, account statedb.Account) error
	GetAccount(ctx sdk.Context, address common.Address) *statedb.Account
}
