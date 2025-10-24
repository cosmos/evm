package keeper

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm/precompiles/ics02"
	"github.com/cosmos/evm/x/ibc/clients/types"
	"github.com/cosmos/evm/x/vm/statedb"

	"cosmossdk.io/collections"
	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k Keeper) GetClientPrecompileInstance(ctx sdk.Context, address common.Address) (vm.PrecompiledContract, bool, error) {
	precompile, err := k.AddressPrecompilesMap.Get(ctx, address.Bytes())
	if err != nil {
		if errorsmod.IsOf(err, collections.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if !precompile.Enabled {
		return nil, false, errorsmod.Wrapf(types.ErrPrecompileDisabled, "precompile at address %s is disabled", address.String())
	}

	return ics02.NewPrecompile(precompile, k.bankKeeper, k.clientKeeper), true, nil
}

// createNewPrecompile creates and stores a new client precompile mapping.
func (k Keeper) createNewPrecompile(ctx sdk.Context, clientID string, address common.Address) (types.ClientPrecompile, error) {
	if err := k.validateNewPrecompile(ctx, clientID, address); err != nil {
		return types.ClientPrecompile{}, err
	}

	precompile := types.ClientPrecompile{
		ClientId: clientID,
		Address:  address.Hex(),
		Enabled:  true,
	}

	if err := k.ClientPrecompilesMap.Set(ctx, clientID, precompile); err != nil {
		return types.ClientPrecompile{}, err
	}
	if err := k.AddressPrecompilesMap.Set(ctx, address.Bytes(), precompile); err != nil {
		return types.ClientPrecompile{}, err
	}
	if err := k.registerClientCodeHash(ctx, address); err != nil {
		return types.ClientPrecompile{}, err
	}

	return precompile, nil
}

// registerClientCodeHash sets the codehash for the client precompile account in the EVM.
func (k Keeper) registerClientCodeHash(ctx sdk.Context, address common.Address) error {
	var (
		bytecode = common.FromHex(types.SolidityLightClientBytecode)
		codeHash = crypto.Keccak256(bytecode)
	)
	// check if code was already stored
	code := k.evmKeeper.GetCode(ctx, common.Hash(codeHash))
	if len(code) != 0 {
		return fmt.Errorf("code already registered for client precompile at address %s", address.String())
	}

	k.evmKeeper.SetCode(ctx, codeHash, bytecode)

	var (
		nonce   uint64
		balance = common.U2560
	)
	// keep balance and nonce if account exists
	if acc := k.evmKeeper.GetAccount(ctx, address); acc != nil {
		nonce = acc.Nonce
		balance = acc.Balance
	}

	return k.evmKeeper.SetAccount(ctx, address, statedb.Account{
		CodeHash: codeHash,
		Nonce:    nonce,
		Balance:  balance,
	})
}

// validateNewPrecompile validates that a new precompile can be created for the given client ID and address.
func (k Keeper) validateNewPrecompile(ctx sdk.Context, clientID string, address common.Address) error {
	account := k.evmKeeper.GetAccount(ctx, address)
	if account != nil && account.HasCodeHash() {
		return errorsmod.Wrapf(types.ErrPrecompileAlreadyExists, "precompile already exists for address %s", address)
	}

	isAddrMapped, err := k.AddressPrecompilesMap.Has(ctx, address.Bytes())
	if err != nil {
		return err
	}
	if isAddrMapped {
		return errorsmod.Wrapf(types.ErrPrecompileAlreadyExists, "precompile already mapped for address %s", address)
	}

	isClientMapped, err := k.ClientPrecompilesMap.Has(ctx, clientID)
	if err != nil {
		return err
	}
	if isClientMapped {
		return errorsmod.Wrapf(types.ErrPrecompileAlreadyExists, "precompile already mapped for client ID %s", clientID)
	}

	return nil
}
