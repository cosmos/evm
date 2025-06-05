//
// This file contains all utility function that require the access to the unit
// test network and should only be used in unit tests.

package utils

import (
	"fmt"

	"github.com/cosmos/evm/testutil/integration/evm/network"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

const (
	TokenToMint = 1e18
)

// RegisterEvmosERC20Coins uses the UnitNetwork to register the evmos token as an
// ERC20 token. The function performs all the required steps for the registration
// like registering the denom in the transfer keeper and minting the token
// with the bank. Returns the TokenPair or an error.
func RegisterEvmosERC20Coins(
	network network.UnitTestNetwork,
	tokenReceiver sdk.AccAddress,
) (erc20types.TokenPair, error) {
	bondDenom, err := network.App.GetStakingKeeper().BondDenom(network.GetContext())
	if err != nil {
		return erc20types.TokenPair{}, err
	}

	coin := sdk.NewCoin(bondDenom, math.NewInt(TokenToMint))
	err = network.App.GetBankKeeper().MintCoins(
		network.GetContext(),
		minttypes.ModuleName,
		sdk.NewCoins(coin),
	)
	if err != nil {
		return erc20types.TokenPair{}, err
	}
	err = network.App.GetBankKeeper().SendCoinsFromModuleToAccount(
		network.GetContext(),
		minttypes.ModuleName,
		tokenReceiver,
		sdk.NewCoins(coin),
	)
	if err != nil {
		return erc20types.TokenPair{}, err
	}

	cosmosEVMMetadata, found := network.App.GetBankKeeper().GetDenomMetaData(network.GetContext(), bondDenom)
	if !found {
		return erc20types.TokenPair{}, fmt.Errorf("expected evmos denom metadata")
	}

	_, err = network.App.GetErc20Keeper().RegisterERC20Extension(network.GetContext(), cosmosEVMMetadata.Base)
	if err != nil {
		return erc20types.TokenPair{}, err
	}

	cosmosEVMDenomID := network.App.GetErc20Keeper().GetDenomMap(network.GetContext(), bondDenom)
	tokenPair, ok := network.App.GetErc20Keeper().GetTokenPair(network.GetContext(), cosmosEVMDenomID)
	if !ok {
		return erc20types.TokenPair{}, fmt.Errorf("expected evmos erc20 token pair")
	}

	return tokenPair, nil
}

// RegisterIBCERC20Coins uses the UnitNetwork to register the denom as an
// ERC20 token. The function performs all the required steps for the registration
// like registering the denom in the transfer keeper and minting the token
// with the bank. Returns the TokenPair or an error.
//
// TODO: why is this not yet used
func RegisterIBCERC20Coins(
	network *network.UnitTestNetwork,
	tokenReceiver sdk.AccAddress,
	denom transfertypes.Denom,
) (erc20types.TokenPair, error) {
	ibcDenom := denom.IBCDenom()
	network.App.GetTransferKeeper().SetDenom(network.GetContext(), denom)
	ibcMetadata := banktypes.Metadata{
		Name:        "Generic IBC name",
		Symbol:      "IBC",
		Description: "Generic IBC token description",
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    ibcDenom,
				Exponent: 0,
				Aliases:  []string{ibcDenom},
			},
			{
				Denom:    ibcDenom,
				Exponent: 18,
			},
		},
		Display: ibcDenom,
		Base:    ibcDenom,
	}

	coin := sdk.NewCoin(ibcMetadata.Base, math.NewInt(TokenToMint))
	err := network.App.GetBankKeeper().MintCoins(
		network.GetContext(),
		minttypes.ModuleName,
		sdk.NewCoins(coin),
	)
	if err != nil {
		return erc20types.TokenPair{}, err
	}

	err = network.App.GetBankKeeper().SendCoinsFromModuleToAccount(
		network.GetContext(),
		minttypes.ModuleName,
		tokenReceiver,
		sdk.NewCoins(coin),
	)
	if err != nil {
		return erc20types.TokenPair{}, err
	}

	_, err = network.App.GetErc20Keeper().RegisterERC20Extension(network.GetContext(), ibcMetadata.Base)
	if err != nil {
		return erc20types.TokenPair{}, err
	}

	ibcDenomID := network.App.GetErc20Keeper().GetDenomMap(
		network.GetContext(),
		denom.IBCDenom(),
	)
	tokenPair, ok := network.App.GetErc20Keeper().GetTokenPair(network.GetContext(), ibcDenomID)
	if !ok {
		return erc20types.TokenPair{}, fmt.Errorf("expected %s erc20 token pair", ibcDenom)
	}

	return tokenPair, nil
}
