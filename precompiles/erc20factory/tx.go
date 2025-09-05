// Copyright Tharsis Labs Ltd.(Evmos)
// SPDX-License-Identifier:ENCL-1.0(https://github.com/evmos/evmos/blob/main/LICENSE)

package erc20factory

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"

	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	// CreateMethod defines the ABI method name to create a new ERC20 Token Pair
	CreateMethod = "create"
)

// Create CreateERC20Precompile creates a new ERC20 TokenPair
func (p Precompile) Create(
	ctx sdk.Context,
	stateDB vm.StateDB,
	method *abi.Method,
	caller common.Address,
	args []interface{},
) ([]byte, error) {
	salt, name, symbol, decimals, minter, premintedSupply, err := ParseCreateArgs(args)
	if err != nil {
		return nil, err
	}

	address := crypto.CreateAddress2(caller, salt, []byte{})

	hash := p.evmKeeper.GetCodeHash(ctx, address)
	if hash.Cmp(common.BytesToHash(evmtypes.EmptyCodeHash)) != 0 {
		return nil, errors.Wrapf(
			erc20types.ErrContractAlreadyExists,
			"contract already exists at address %s", address.String(),
		)
	}

	metadata, err := p.createCoinMetadata(ctx, address, name, symbol, decimals)
	if err != nil {
		return nil, errors.Wrap(
			err, "failed to create wrapped coin denom metadata for ERC20",
		)
	}

	if err := metadata.Validate(); err != nil {
		return nil, errors.Wrapf(
			err, "ERC20 token data is invalid for contract %s", address.String(),
		)
	}

	p.bankKeeper.SetDenomMetaData(ctx, *metadata)

	pair := erc20types.NewTokenPair(address, metadata.Name, erc20types.OWNER_EXTERNAL)

	err = p.erc20Keeper.SetToken(ctx, pair)
	if err != nil {
		return nil, err
	}

	err = p.erc20Keeper.EnableDynamicPrecompile(ctx, pair.GetERC20Contract())
	if err != nil {
		return nil, err
	}

	coins := sdk.NewCoins(sdk.NewCoin(metadata.Base, math.NewIntFromBigInt(premintedSupply)))
	if err := p.bankKeeper.MintCoins(ctx, erc20types.ModuleName, coins); err != nil {
		return nil, err
	}
	if err := p.bankKeeper.SendCoinsFromModuleToAccount(ctx, erc20types.ModuleName, sdk.AccAddress(minter.Bytes()), coins); err != nil {
		return nil, err
	}

	if err = p.EmitCreateEvent(ctx, stateDB, address, salt, name, symbol, decimals, minter, premintedSupply); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(address)
}

func (p Precompile) createCoinMetadata(ctx sdk.Context, address common.Address, name string, symbol string, decimals uint8) (*banktypes.Metadata, error) {
	addressString := address.String()
	denom := erc20types.CreateDenom(addressString)

	_, found := p.bankKeeper.GetDenomMetaData(ctx, denom)
	if found {
		return nil, errors.Wrap(
			erc20types.ErrInternalTokenPair, "denom metadata already registered",
		)
	}

	if p.erc20Keeper.IsDenomRegistered(ctx, denom) {
		return nil, errors.Wrapf(
			erc20types.ErrInternalTokenPair, "coin denomination already registered: %s", name,
		)
	}

	// base denomination
	base := erc20types.CreateDenom(addressString)

	// create a bank denom metadata based on the ERC20 token ABI details
	// metadata name is should always be the contract since it's the key
	// to the bank store
	metadata := banktypes.Metadata{
		Description: erc20types.CreateDenomDescription(addressString),
		Base:        base,
		// NOTE: Denom units MUST be increasing
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    base,
				Exponent: 0,
			},
		},
		Name:    base,
		Symbol:  symbol,
		Display: base,
	}

	// only append metadata if decimals > 0, otherwise validation fails
	if decimals > 0 {
		nameSanitized := erc20types.SanitizeERC20Name(name)
		metadata.DenomUnits = append(
			metadata.DenomUnits,
			&banktypes.DenomUnit{
				Denom:    nameSanitized,
				Exponent: uint32(decimals), //#nosec G115
			},
		)
		metadata.Display = nameSanitized
	}

	return &metadata, nil
}
