package evmd

import (
	"context"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// UpgradeName defines the on-chain upgrade name for the EpixChain upgrade
// from v0.5.0 to v0.5.1.
//
// This upgrade migrates EpixChain from cosmos/evm v0.5.0 to v0.5.1, which includes:
// - Updated function signatures and import paths
// - Improved EVM configuration handling
// - Enhanced denom metadata for aepix/epix tokens
const UpgradeName = "v0.5.1"

func (app EVMD) RegisterUpgradeHandlers() {
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			sdkCtx.Logger().Info("Starting EpixChain v0.5.0 to v0.5.1 upgrade...")

			// Set denom metadata for EpixChain's native token (aepix/epix)
			app.BankKeeper.SetDenomMetaData(ctx, banktypes.Metadata{
				Description: "The native staking and governance token of the EpixChain",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    BaseDenom,
						Exponent: 0,
						Aliases:  nil,
					},
					{
						Denom:    DisplayDenom,
						Exponent: Decimals,
						Aliases:  nil,
					},
				},
				Base:    BaseDenom,
				Display: DisplayDenom,
				Name:    "EpixChain",
				Symbol:  "EPIX",
				URI:     "https://epix.zone/",
			})

			sdkCtx.Logger().Info("EpixChain denom metadata updated successfully")

			// Update EVM params to set the EvmDenom and ExtendedDenomOptions
			evmParams := app.EVMKeeper.GetParams(sdkCtx)
			evmParams.EvmDenom = BaseDenom
			evmParams.ExtendedDenomOptions = &evmtypes.ExtendedDenomOptions{
				ExtendedDenom: BaseDenom,
			}
			if err := app.EVMKeeper.SetParams(sdkCtx, evmParams); err != nil {
				return nil, err
			}
			sdkCtx.Logger().Info("EVM params updated successfully")

			// Initialize EVM coin info from the bank metadata
			if err := app.EVMKeeper.InitEvmCoinInfo(sdkCtx); err != nil {
				return nil, err
			}
			sdkCtx.Logger().Info("EVM coin info initialized successfully")

			return app.ModuleManager.RunMigrations(ctx, app.Configurator(), fromVM)
		},
	)

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(err)
	}

	if upgradeInfo.Name == UpgradeName && !app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		storeUpgrades := storetypes.StoreUpgrades{
			Added: []string{},
		}
		// configure store loader that checks if version == upgradeHeight and applies store upgrades
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}
