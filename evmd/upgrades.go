package evmd

import (
	"context"

	"github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// UpgradeName defines the on-chain upgrade name for the sample EVMD upgrade
// from v0.4.0 to v0.5.0.
//
// NOTE: This upgrade defines a reference implementation of what an upgrade
// could look like when an application is migrating from EVMD version
// v0.4.0 to v0.5.x
const UpgradeName = "v0.4.0-to-v0.5.0"

func (app EVMD) RegisterUpgradeHandlers() {
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			sdkCtx.Logger().Debug("this is a debug level message to test that verbose logging mode has properly been enabled during a chain upgrade")

			app.BankKeeper.SetDenomMetaData(ctx, banktypes.Metadata{
				Description: "Example description",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    "atest",
						Exponent: 0,
						Aliases:  nil,
					},
					{
						Denom:    "test",
						Exponent: 18,
						Aliases:  nil,
					},
				},
				Base:    "atest",
				Display: "test",
				Name:    "Test Token",
				Symbol:  "TEST",
				URI:     "example_uri",
				URIHash: "example_uri_hash",
			})

			// (Required for NON-18 denom chains *only)
			// Update EVM params to add Extended denom options
			// Ensure that this corresponds to the EVM denom
			// (tyically the bond denom)
			evmParams := app.EVMKeeper.GetParams(sdkCtx)
			evmParams.ExtendedDenomOptions = &types.ExtendedDenomOptions{ExtendedDenom: "atest"}
			err := app.EVMKeeper.SetParams(sdkCtx, evmParams)
			if err != nil {
				return nil, err
			}
			// Initialize EvmCoinInfo in the module store. Chains bootstrapped before v0.5.0
			// binaries never stored this information (it lived only in process globals),
			// so migrating nodes would otherwise see an empty EvmCoinInfo on upgrade.
			if err := app.EVMKeeper.InitEvmCoinInfo(sdkCtx); err != nil {
				return nil, err
			}
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
