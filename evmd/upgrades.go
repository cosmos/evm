package evmd

import (
	"context"
	"fmt"

	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	"github.com/cosmos/evm/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// UpgradeName defines the on-chain upgrade name for the EpixChain upgrade
// from v0.5.1 to v0.5.2.
//
// This upgrade fixes the IBC channel state that was not properly migrated in v0.5.1:
// - Initializes missing IBC channel sequence counters
// - Ensures EVM params and coin info are properly set
const UpgradeName = "v0.5.2"

// Previous upgrade name for reference
const previousUpgradeName = "v0.5.1"

func (app EVMD) RegisterUpgradeHandlers() {
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeName,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			sdkCtx.Logger().Info("Starting EpixChain v0.5.1 to v0.5.2 recovery upgrade...")

			// Set denom metadata for EpixChain's native token (aepix/epix)
			// This ensures the metadata is correct even if v0.5.1 upgrade had issues
			app.BankKeeper.SetDenomMetaData(ctx, banktypes.Metadata{
				Description: "The native staking and governance token of the EpixChain",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    config.EpixChainDenom,
						Exponent: 0,
						Aliases:  nil,
					},
					{
						Denom:    config.EpixDisplayDenom,
						Exponent: 18,
						Aliases:  nil,
					},
				},
				Base:    config.EpixChainDenom,
				Display: config.EpixDisplayDenom,
				Name:    "EpixChain",
				Symbol:  "EPIX",
				URI:     "https://epix.zone/",
			})
			sdkCtx.Logger().Info("EpixChain denom metadata set successfully")

			// Update EVM params to set the EvmDenom and ExtendedDenomOptions
			evmParams := app.EVMKeeper.GetParams(sdkCtx)
			evmParams.EvmDenom = config.EpixChainDenom
			evmParams.ExtendedDenomOptions = &evmtypes.ExtendedDenomOptions{
				ExtendedDenom: config.EpixChainDenom,
			}
			if err := app.EVMKeeper.SetParams(sdkCtx, evmParams); err != nil {
				return nil, fmt.Errorf("failed to set EVM params: %w", err)
			}
			sdkCtx.Logger().Info("EVM params updated successfully")

			// Initialize EVM coin info from the bank metadata
			if err := app.EVMKeeper.InitEvmCoinInfo(sdkCtx); err != nil {
				return nil, fmt.Errorf("failed to initialize EVM coin info: %w", err)
			}
			sdkCtx.Logger().Info("EVM coin info initialized successfully")

			// Fix IBC channel sequence counters
			// Query all channels and ensure their sequence counters are initialized
			channels := app.IBCKeeper.ChannelKeeper.GetAllChannels(sdkCtx)
			sdkCtx.Logger().Info(fmt.Sprintf("Found %d IBC channels to check", len(channels)))

			for _, channel := range channels {
				portID := channel.PortId
				channelID := channel.ChannelId

				// Check if NextSequenceSend exists
				_, found := app.IBCKeeper.ChannelKeeper.GetNextSequenceSend(sdkCtx, portID, channelID)
				if !found {
					// Initialize to 1 if not found
					app.IBCKeeper.ChannelKeeper.SetNextSequenceSend(sdkCtx, portID, channelID, 1)
					sdkCtx.Logger().Info(fmt.Sprintf("Initialized NextSequenceSend for port %s, channel %s to 1", portID, channelID))
				}

				// Check if NextSequenceRecv exists
				_, found = app.IBCKeeper.ChannelKeeper.GetNextSequenceRecv(sdkCtx, portID, channelID)
				if !found {
					// Initialize to 1 if not found
					app.IBCKeeper.ChannelKeeper.SetNextSequenceRecv(sdkCtx, portID, channelID, 1)
					sdkCtx.Logger().Info(fmt.Sprintf("Initialized NextSequenceRecv for port %s, channel %s to 1", portID, channelID))
				}

				// For unordered channels, check NextSequenceAck
				if channel.Ordering == channeltypes.UNORDERED {
					_, found = app.IBCKeeper.ChannelKeeper.GetNextSequenceAck(sdkCtx, portID, channelID)
					if !found {
						app.IBCKeeper.ChannelKeeper.SetNextSequenceAck(sdkCtx, portID, channelID, 1)
						sdkCtx.Logger().Info(fmt.Sprintf("Initialized NextSequenceAck for port %s, channel %s to 1", portID, channelID))
					}
				}
			}

			sdkCtx.Logger().Info("IBC channel sequence counters recovery complete")

			// Run module migrations
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
