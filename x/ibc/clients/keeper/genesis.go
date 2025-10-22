package keeper

import (
	"context"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/ibc/clients/types"
)

// InitGenesis initializes the module state from a genesis state.
func (k *Keeper) InitGenesis(ctx context.Context, data *types.GenesisState) error {
	if err := k.Params.Set(ctx, data.Params); err != nil {
		return err
	}

	for _, precompile := range data.ClientPrecompiles {
		if !common.IsHexAddress(precompile.Address) {
			return types.ErrInvalidPrecompileAddress.Wrapf("precompile address %s is not a valid hex address", precompile.Address)
		}

		addressBz := common.HexToAddress(precompile.Address).Bytes()

		if err := k.ClientPrecompiles.Set(ctx, precompile.ClientId, precompile); err != nil {
			return err
		}
		if err := k.Precompiles.Set(ctx, addressBz, precompile); err != nil {
			return err
		}
	}

	return nil
}

// ExportGenesis exports the module state to a genesis state.
func (k *Keeper) ExportGenesis(ctx context.Context) (*types.GenesisState, error) {
	params, err := k.Params.Get(ctx)
	if err != nil {
		return nil, err
	}

	var precompiles []types.ClientPrecompile
	if err := k.ClientPrecompiles.Walk(ctx, nil, func(address string, precompile types.ClientPrecompile) (bool, error) {
		precompiles = append(precompiles, precompile)

		return false, nil
	}); err != nil {
		return nil, err
	}

	return &types.GenesisState{
		Params:            params,
		ClientPrecompiles: precompiles,
	}, nil
}
