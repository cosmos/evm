package evmd

import (
	"context"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"
)

// UpgradeName defines the on-chain upgrade name for the EpixChain upgrade
// from v0.5.0 to v0.6.0.
//
// This upgrade migrates EpixChain from cosmos/evm v0.4.0 to v0.5.0, which includes:
// - Updated function signatures and import paths
// - Improved EVM configuration handling
// - Enhanced denom metadata for aepix/epix tokens
const UpgradeName = "v0.6.0"

func (app EVMD) RegisterUpgradeHandlers() {
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			sdkCtx.Logger().Info("Starting EpixChain v0.4.0 to v0.5.0 upgrade...")

			// Set denom metadata for EpixChain's native token (aepix/epix)
			app.BankKeeper.SetDenomMetaData(ctx, banktypes.Metadata{
				Description: "The native staking and governance token of the EpixChain",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    "aepix",
						Exponent: 0,
						Aliases:  nil,
					},
					{
						Denom:    "epix",
						Exponent: 18,
						Aliases:  nil,
					},
				},
				Base:    "aepix",
				Display: "epix",
				Name:    "EpixChain",
				Symbol:  "EPIX",
				URI:     "https://epix.zone/",
				URIHash: "8c574bb30f45242cd3058a6608d63fe49ba64eebbc158b93f5bfc61bb92f002c",
			})

			// EpixChain uses 18 decimals, so ExtendedDenomOptions is NOT required
			// This section is only needed for NON-18 decimal chains
			// Since EpixChain is an 18-decimal chain, we skip this step

			sdkCtx.Logger().Info("EpixChain denom metadata updated successfully")

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
