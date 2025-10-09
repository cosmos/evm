package config

import (
	"math"
	"path/filepath"

	"github.com/holiman/uint256"
	"github.com/spf13/cast"

	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	srvflags "github.com/cosmos/evm/server/flags"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
)

// GetBlockGasLimit reads the genesis json file using AppGenesisFromFile
// to extract the consensus block gas limit before InitChain is called.
func GetBlockGasLimit(appOpts servertypes.AppOptions, logger log.Logger) uint64 {
	if appOpts == nil {
		logger.Error("app options is nil, using zero block gas limit")
		return math.MaxUint64
	}

	homeDir := cast.ToString(appOpts.Get(flags.FlagHome))
	if homeDir == "" {
		logger.Error("home directory not found in app options, using zero block gas limit")
		return math.MaxUint64
	}
	genesisPath := filepath.Join(homeDir, "config", "genesis.json")

	appGenesis, err := genutiltypes.AppGenesisFromFile(genesisPath)
	if err != nil {
		logger.Error("failed to load genesis using SDK AppGenesisFromFile, using zero block gas limit", "path", genesisPath, "error", err)
		return 0
	}
	genDoc, err := appGenesis.ToGenesisDoc()
	if err != nil {
		logger.Error("failed to convert AppGenesis to GenesisDoc, using zero block gas limit", "path", genesisPath, "error", err)
		return 0
	}

	if genDoc.ConsensusParams == nil {
		logger.Error("consensus parameters not found in genesis (nil), using zero block gas limit")
		return 0
	}

	maxGas := genDoc.ConsensusParams.Block.MaxGas
	if maxGas == -1 {
		logger.Warn("genesis max_gas is unlimited (-1), using max uint64")
		return math.MaxUint64
	}
	if maxGas < -1 {
		logger.Error("invalid max_gas value in genesis, using zero block gas limit")
		return 0
	}
	blockGasLimit := uint64(maxGas) // #nosec G115 -- maxGas >= 0 checked above

	logger.Debug(
		"extracted block gas limit from genesis using SDK AppGenesisFromFile",
		"genesis_path", genesisPath,
		"max_gas", maxGas,
		"block_gas_limit", blockGasLimit,
	)

	return blockGasLimit
}

// GetMinGasPrices reads the min gas prices from the app options, set from app.toml
// This is currently not used, but is kept in case this is useful for the mempool,
// in addition to the min tip flag
func GetMinGasPrices(appOpts servertypes.AppOptions, logger log.Logger) sdk.DecCoins {
	minGasPricesStr := cast.ToString(appOpts.Get(sdkserver.FlagMinGasPrices))
	minGasPrices, err := sdk.ParseDecCoins(minGasPricesStr)
	if err != nil {
		logger.With("error", err).Info("failed to parse min gas prices, using empty DecCoins")
		minGasPrices = sdk.DecCoins{}
	}

	return minGasPrices
}

// GetMinTip reads the min tip from the app options, set from app.toml
// This field is also known as the minimum priority fee
func GetMinTip(appOpts servertypes.AppOptions, logger log.Logger) *uint256.Int {
	if appOpts == nil {
		logger.Error("app options is nil, using zero min tip")
		return nil
	}

	minTipUint64 := cast.ToUint64(appOpts.Get(srvflags.EVMMinTip))
	minTip := uint256.NewInt(minTipUint64)

	if minTip.Cmp(uint256.NewInt(0)) >= 0 { // zero or positive
		return minTip
	}

	logger.Error("invalid min tip value in app.toml or flag, falling back to nil", "min_tip", minTipUint64)
	return nil
}

// GetMempoolConfig reads the mempool configuration from appOpts
func GetMempoolConfig(appOpts servertypes.AppOptions, logger log.Logger) (*evmmempool.EVMMempoolConfig, error) {
	if appOpts == nil {
		logger.Error("app options is nil, using default mempool config")
		return &evmmempool.EVMMempoolConfig{
			LegacyPoolConfig: &legacypool.DefaultConfig,
		}, nil
	}

	// Start with default configuration
	legacyConfig := legacypool.DefaultConfig

	// Override with values from app.toml if they exist and are non-zero
	if priceLimit := cast.ToUint64(appOpts.Get("evm.mempool.price-limit")); priceLimit != 0 {
		legacyConfig.PriceLimit = priceLimit
	}
	if priceBump := cast.ToUint64(appOpts.Get("evm.mempool.price-bump")); priceBump != 0 {
		legacyConfig.PriceBump = priceBump
	}
	if accountSlots := cast.ToUint64(appOpts.Get("evm.mempool.account-slots")); accountSlots != 0 {
		legacyConfig.AccountSlots = accountSlots
	}
	if globalSlots := cast.ToUint64(appOpts.Get("evm.mempool.global-slots")); globalSlots != 0 {
		legacyConfig.GlobalSlots = globalSlots
	}
	if accountQueue := cast.ToUint64(appOpts.Get("evm.mempool.account-queue")); accountQueue != 0 {
		legacyConfig.AccountQueue = accountQueue
	}
	if globalQueue := cast.ToUint64(appOpts.Get("evm.mempool.global-queue")); globalQueue != 0 {
		legacyConfig.GlobalQueue = globalQueue
	}
	if lifetime := cast.ToDuration(appOpts.Get("evm.mempool.lifetime")); lifetime != 0 {
		legacyConfig.Lifetime = lifetime
	}

	// Journal and Rejournal are not configurable via app.toml - use defaults

	mempoolConfig := &evmmempool.EVMMempoolConfig{
		LegacyPoolConfig: &legacyConfig,
	}

	return mempoolConfig, nil
}
