package keeper

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	logTransferSig     = []byte("Transfer(address,address,uint256)")
	logTransferSigHash = crypto.Keccak256Hash(logTransferSig)

	logApprovalSig     = []byte("Approval(address,address,uint256)")
	logApprovalSigHash = crypto.Keccak256Hash(logApprovalSig)
)

// QueryERC20 returns the data of a deployed ERC20 contract
func (k Keeper) QueryERC20(
	ctx sdk.Context,
	contract common.Address,
) (types.ERC20Data, error) {
	erc20 := contracts.ERC20MinterBurnerDecimalsContract.ABI

	// Name - with fallback support for bytes32
	name, err := k.queryERC20String(ctx, erc20, contract, "name")
	if err != nil {
		return types.ERC20Data{}, err
	}

	// Symbol - with fallback support for bytes32
	symbol, err := k.queryERC20String(ctx, erc20, contract, "symbol")
	if err != nil {
		return types.ERC20Data{}, err
	}

	// Decimals - standard uint8, no fallback needed
	stateDB := statedb.New(ctx, k.evmKeeper, statedb.NewEmptyTxConfig())
	// Okay to assume we're not calling from a precompile, as queries will just revert state changes.
	res, err := k.evmKeeper.CallEVM(ctx, stateDB, erc20, types.ModuleAddress, contract, false, false, nil, "decimals")
	if err != nil {
		return types.ERC20Data{}, err
	}

	var decimalRes types.ERC20Uint8Response
	if err := erc20.UnpackIntoInterface(&decimalRes, "decimals", res.Ret); err != nil {
		return types.ERC20Data{}, errorsmod.Wrapf(
			types.ErrABIUnpack, "failed to unpack decimals: %s", err.Error(),
		)
	}

	return types.NewERC20Data(name, symbol, decimalRes.Value), nil
}

// queryERC20String attempts to query an ERC20 string field with fallback to bytes32
func (k Keeper) queryERC20String(
	ctx sdk.Context,
	erc20 abi.ABI,
	contract common.Address,
	method string,
) (string, error) {
	// 1) Call into the EVM
	stateDB := statedb.New(ctx, k.evmKeeper, statedb.NewEmptyTxConfig())
	// Okay to assume we're not calling from a precompile, as queries will just revert state changes.
	res, err := k.evmKeeper.CallEVM(ctx, stateDB, erc20, types.ModuleAddress, contract, false, false, nil, method)
	if err != nil {
		return "", err
	}

	// 2) First try to unpack as a normal ABI “string”
	var strResp types.ERC20StringResponse
	if err := erc20.UnpackIntoInterface(&strResp, method, res.Ret); err == nil {
		return strResp.Value, nil
	}

	// 3) Fallback: if we got exactly 32 bytes back, treat it as bytes32
	if len(res.Ret) == 32 {
		var b [32]byte
		copy(b[:], res.Ret)
		return utils.Bytes32ToString(b), nil
	}

	// 4) Otherwise it really is neither a string nor a 32‐byte static, so error
	return "", errorsmod.Wrapf(
		types.ErrABIUnpack,
		"failed to unpack %s as both string and raw bytes32 (len=%d)",
		method,
		len(res.Ret),
	)
}

// BalanceOf queries an account's balance for a given ERC20 contract
func (k Keeper) BalanceOf(
	ctx sdk.Context,
	abi abi.ABI,
	contract, account common.Address,
) *big.Int {
	stateDB := statedb.New(ctx, k.evmKeeper, statedb.NewEmptyTxConfig())
	// Okay to assume we're not calling from a precompile, as queries will just revert state changes.
	res, err := k.evmKeeper.CallEVM(ctx, stateDB, abi, types.ModuleAddress, contract, false, false, nil, "balanceOf", account)
	if err != nil {
		return nil
	}

	unpacked, err := abi.Unpack("balanceOf", res.Ret)
	if err != nil || len(unpacked) == 0 {
		return nil
	}

	balance, ok := unpacked[0].(*big.Int)
	if !ok {
		return nil
	}

	return balance
}
