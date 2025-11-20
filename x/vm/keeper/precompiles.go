package keeper

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/x/vm/types"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
)

type Precompiles struct {
	Map       map[common.Address]vm.PrecompiledContract
	Addresses []common.Address
}

// GetPrecompileInstance returns the address and instance of the static or dynamic precompile associated with the
// given address, or return nil if not found.
func (k *Keeper) GetPrecompileInstance(
	ctx sdktypes.Context,
	address common.Address,
) (*Precompiles, bool, error) {
	params := k.GetParams(ctx)
	// Get the precompile from the static precompiles
	if precompile, found, err := k.GetStaticPrecompileInstance(&params, address); err != nil {
		return nil, false, err
	} else if found {
		addressMap := make(map[common.Address]vm.PrecompiledContract)
		addressMap[address] = precompile
		return &Precompiles{
			Map:       addressMap,
			Addresses: []common.Address{precompile.Address()},
		}, found, nil
	}

	// Since erc20Keeper is optional, we check if it is nil, in which case we just return that we didn't find the precompile
	if k.erc20Keeper == nil {
		return nil, false, nil
	}

	// Get the precompile from the dynamic precompiles
	precompile, found, err := k.erc20Keeper.GetERC20PrecompileInstance(ctx, address)
	if err != nil || !found {
		return nil, false, err
	}
	addressMap := make(map[common.Address]vm.PrecompiledContract)
	addressMap[address] = precompile
	return &Precompiles{
		Map:       addressMap,
		Addresses: []common.Address{precompile.Address()},
	}, found, nil
}

// GetPrecompilesCallHook returns a closure that can be used to instantiate the EVM with a specific
// precompile instance.
func (k *Keeper) GetPrecompilesCallHook(ctx sdktypes.Context) types.CallHook {
	return func(evm *vm.EVM, _ common.Address, recipient common.Address) error {
		// Check if the recipient is a precompile contract and if so, load the precompile instance
		precompiles, found, err := k.GetPrecompileInstance(ctx, recipient)
		if err != nil {
			return err
		}

		// If the precompile instance is created, we have to update the EVM with
		// only the recipient precompile and add it's address to the access list.
		if found {
			evm.WithPrecompiles(precompiles.Map)
			evm.StateDB.AddAddressToAccessList(recipient)
		}

		return nil
	}
}

// GetPrecompileRecipientCallHook returns a closure that can be used to instantiate the EVM with a specific
// recipient from precompiles.
func (k *Keeper) GetPrecompileRecipientCallHook(ctx sdktypes.Context) types.CallHook {
	return func(evm *vm.EVM, _ common.Address, recipient common.Address) error {
		// Check if the recipient is a precompile contract and if so, load the precompile instance
		_, found, err := k.GetPrecompileInstance(ctx, recipient)
		if err != nil {
			return err
		}

		// If the precompile instance is created, we have to update the EVM with
		// only the recipient precompile and add it's address to the access list.
		if found {
			evm.StateDB.AddAddressToAccessList(recipient)
		}

		return nil
	}
}
