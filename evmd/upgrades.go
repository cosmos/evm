package evmd

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"

	"github.com/cosmos/evm/config"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// Upgrade names
const UpgradeName_v0_5_1 = "v0.5.1"
const UpgradeName_v0_5_2 = "v0.5.2"
const UpgradeName_v0_5_3 = "v0.5.3"
const UpgradeName_v0_5_4 = "v0.5.4"

// UpgradeName is the current upgrade (for store upgrades)
const UpgradeName = UpgradeName_v0_5_4

// RegisterUpgradeHandlers registers upgrade handlers for v0.5.1 and v0.5.2
func (app EVMD) RegisterUpgradeHandlers() {
	// Register v0.5.1 upgrade handler (the one that's currently stuck)
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeName_v0_5_1,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			sdkCtx.Logger().Info("Starting EpixChain v0.5.1 upgrade with recovery fix...")

			// Apply the recovery fix
			if err := app.applyRecoveryFix(ctx); err != nil {
				return nil, err
			}

			// Run module migrations
			return app.ModuleManager.RunMigrations(ctx, app.Configurator(), fromVM)
		},
	)

	// Register v0.5.2 upgrade handler (for future use)
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeName_v0_5_2,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			sdkCtx.Logger().Info("Starting EpixChain v0.5.2 upgrade...")

			// Apply the recovery fix (in case v0.5.1 didn't run it)
			if err := app.applyRecoveryFix(ctx); err != nil {
				return nil, err
			}

			// Run module migrations
			return app.ModuleManager.RunMigrations(ctx, app.Configurator(), fromVM)
		},
	)

	// Register v0.5.3 upgrade handler - Deterministic Emission Fix + Min Validator Self-Delegation
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeName_v0_5_3,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			sdkCtx.Logger().Info("Starting EpixChain v0.5.3 upgrade - Deterministic Emission Fix + Min Validator Self-Delegation...")

			// Log the upgrade details for transparency
			sdkCtx.Logger().Info("This upgrade fixes a consensus bug in token emission calculations")
			sdkCtx.Logger().Info("This upgrade also sets minimum validator self-delegation to 1,000,000 EPIX")

			// Set minimum validator self-delegation to 1M EPIX
			// 1M EPIX in aepix (18 decimals): 1 * 10^6 * 10^18 = 10^24
			minValidatorSelfDelegation, ok := math.NewIntFromString("1000000000000000000000000")
			if !ok {
				return nil, fmt.Errorf("failed to parse min validator self delegation")
			}

			epixmintParams := app.EpixMintKeeper.GetParams(sdkCtx)
			epixmintParams.MinValidatorSelfDelegation = minValidatorSelfDelegation
			if err := app.EpixMintKeeper.SetParams(sdkCtx, epixmintParams); err != nil {
				return nil, fmt.Errorf("failed to set epixmint params: %w", err)
			}
			sdkCtx.Logger().Info("Minimum validator self-delegation set to 1,000,000 EPIX")

			// Run module migrations
			return app.ModuleManager.RunMigrations(ctx, app.Configurator(), fromVM)
		},
	)

	// Register v0.5.4 upgrade handler - IBC Wallet Compatibility Fix + Min Validator Self-Delegation
	app.UpgradeKeeper.SetUpgradeHandler(
		UpgradeName_v0_5_4,
		func(ctx context.Context, _ upgradetypes.Plan, fromVM module.VersionMap) (module.VersionMap, error) {
			sdkCtx := sdk.UnwrapSDKContext(ctx)
			sdkCtx.Logger().Info("Starting EpixChain v0.5.4 upgrade - IBC Wallet Compatibility Fix + Min Validator Self-Delegation...")
			sdkCtx.Logger().Info("This upgrade fixes IBC transfer signature verification for Keplr/Leap wallets")
			sdkCtx.Logger().Info("This upgrade also sets minimum validator self-delegation to 1,000,000 EPIX")

			// Set minimum validator self-delegation to 1M EPIX
			// 1M EPIX in aepix (18 decimals): 1 * 10^6 * 10^18 = 10^24
			minValidatorSelfDelegation, ok := math.NewIntFromString("1000000000000000000000000")
			if !ok {
				return nil, fmt.Errorf("failed to parse min validator self delegation")
			}

			epixmintParams := app.EpixMintKeeper.GetParams(sdkCtx)
			epixmintParams.MinValidatorSelfDelegation = minValidatorSelfDelegation
			if err := app.EpixMintKeeper.SetParams(sdkCtx, epixmintParams); err != nil {
				return nil, fmt.Errorf("failed to set epixmint params: %w", err)
			}
			sdkCtx.Logger().Info("Minimum validator self-delegation set to 1,000,000 EPIX")

			// IBC signature fix requires no state migration - the fix is in signature
			// verification logic (crypto/ethsecp256k1/ethsecp256k1.go) which strips
			// extra amino fields added by IBC v10 (use_aliasing, encoding) that
			// wallets don't include.

			// Run module migrations
			return app.ModuleManager.RunMigrations(ctx, app.Configurator(), fromVM)
		},
	)

	upgradeInfo, err := app.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	if err != nil {
		panic(err)
	}

	// Handle v0.5.1, v0.5.2, and v0.5.3 upgrades
	if (upgradeInfo.Name == UpgradeName_v0_5_1 ||
		upgradeInfo.Name == UpgradeName_v0_5_2 ||
		upgradeInfo.Name == UpgradeName_v0_5_3 ||
		upgradeInfo.Name == UpgradeName_v0_5_4) &&
		!app.UpgradeKeeper.IsSkipHeight(upgradeInfo.Height) {
		storeUpgrades := storetypes.StoreUpgrades{
			Added: []string{},
		}
		// configure store loader that checks if version == upgradeHeight and applies store upgrades
		app.SetStoreLoader(upgradetypes.UpgradeStoreLoader(upgradeInfo.Height, &storeUpgrades))
	}
}

// applyRecoveryFix contains the actual recovery logic for v0.5.1/v0.5.2 upgrades
// This fixes the IBC channel state that was not properly migrated:
// - Sets denom metadata for aepix/epix
// - Updates EVM params and coin info
// - Initializes missing IBC channel sequence counters
func (app EVMD) applyRecoveryFix(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)

	// Set denom metadata for EpixChain's native token (aepix/epix)
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
		return fmt.Errorf("failed to set EVM params: %w", err)
	}
	sdkCtx.Logger().Info("EVM params updated successfully")

	// Initialize EVM coin info from the bank metadata
	if err := app.EVMKeeper.InitEvmCoinInfo(sdkCtx); err != nil {
		return fmt.Errorf("failed to initialize EVM coin info: %w", err)
	}
	sdkCtx.Logger().Info("EVM coin info initialized successfully")

	// Fix IBC channel sequence counters
	channels := app.IBCKeeper.ChannelKeeper.GetAllChannels(sdkCtx)
	sdkCtx.Logger().Info(fmt.Sprintf("Found %d IBC channels to check", len(channels)))

	for _, channel := range channels {
		portID := channel.PortId
		channelID := channel.ChannelId

		// Check if NextSequenceSend exists
		_, found := app.IBCKeeper.ChannelKeeper.GetNextSequenceSend(sdkCtx, portID, channelID)
		if !found {
			app.IBCKeeper.ChannelKeeper.SetNextSequenceSend(sdkCtx, portID, channelID, 1)
			sdkCtx.Logger().Info(fmt.Sprintf("Initialized NextSequenceSend for port %s, channel %s to 1", portID, channelID))
		}

		// Check if NextSequenceRecv exists
		_, found = app.IBCKeeper.ChannelKeeper.GetNextSequenceRecv(sdkCtx, portID, channelID)
		if !found {
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
	return nil
}
