package config

import (
	"math"
	"path/filepath"
	"time"

	"github.com/holiman/uint256"
	"github.com/spf13/cast"
	"github.com/spf13/viper"

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
func GetBlockGasLimit(v *viper.Viper, logger log.Logger) uint64 {
	if v == nil {
		logger.Error("viper instance is nil, using zero block gas limit")
		return math.MaxUint64
	}

	homeDir := v.GetString(flags.FlagHome)
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

// GetMinTip reads the min tip from the viper flags, set from app.toml
// This field is also known as the minimum priority fee
func GetMinTip(v *viper.Viper, logger log.Logger) *uint256.Int {
	if v == nil {
		logger.Error("viper instance is nil, using zero min tip")
		return nil
	}

	minTipUint64 := v.GetUint64(srvflags.EVMMinTip)
	minTip := uint256.NewInt(minTipUint64)

	if minTip.Cmp(uint256.NewInt(0)) >= 0 { // zero or positive
		return minTip
	}

	logger.Error("invalid min tip value in app.toml or flag, falling back to nil", "min_tip", minTipUint64)
	return nil
}

// GetMempoolConfig reads the mempool configuration from viper
func GetMempoolConfig(v *viper.Viper, logger log.Logger) (*evmmempool.EVMMempoolConfig, error) {
	if v == nil {
		logger.Error("viper instance is nil, using default mempool config")
		return &evmmempool.EVMMempoolConfig{}, nil
	}

	// Create LegacyPool config from viper settings
	legacyConfig := &legacypool.Config{
		PriceLimit:   v.GetUint64("evm.mempool.price-limit"),
		PriceBump:    v.GetUint64("evm.mempool.price-bump"),
		AccountSlots: v.GetUint64("evm.mempool.account-slots"),
		GlobalSlots:  v.GetUint64("evm.mempool.global-slots"),
		AccountQueue: v.GetUint64("evm.mempool.account-queue"),
		GlobalQueue:  v.GetUint64("evm.mempool.global-queue"),
		Lifetime:     v.GetDuration("evm.mempool.lifetime"),
		Journal:      "transactions.rlp", // Fixed journal filename
		Rejournal:    time.Hour,          // Fixed rejournal interval
	}

	// Validate and sanitize the config values (similar to sanitize() method)
	sanitizedConfig := *legacyConfig
	if sanitizedConfig.PriceLimit < 1 {
		logger.Warn("Invalid txpool price limit, using default", "provided", sanitizedConfig.PriceLimit, "default", legacypool.DefaultConfig.PriceLimit)
		sanitizedConfig.PriceLimit = legacypool.DefaultConfig.PriceLimit
	}
	if sanitizedConfig.PriceBump < 1 {
		logger.Warn("Invalid txpool price bump, using default", "provided", sanitizedConfig.PriceBump, "default", legacypool.DefaultConfig.PriceBump)
		sanitizedConfig.PriceBump = legacypool.DefaultConfig.PriceBump
	}
	if sanitizedConfig.AccountSlots < 1 {
		logger.Warn("Invalid txpool account slots, using default", "provided", sanitizedConfig.AccountSlots, "default", legacypool.DefaultConfig.AccountSlots)
		sanitizedConfig.AccountSlots = legacypool.DefaultConfig.AccountSlots
	}
	if sanitizedConfig.GlobalSlots < 1 {
		logger.Warn("Invalid txpool global slots, using default", "provided", sanitizedConfig.GlobalSlots, "default", legacypool.DefaultConfig.GlobalSlots)
		sanitizedConfig.GlobalSlots = legacypool.DefaultConfig.GlobalSlots
	}
	if sanitizedConfig.AccountQueue < 1 {
		logger.Warn("Invalid txpool account queue, using default", "provided", sanitizedConfig.AccountQueue, "default", legacypool.DefaultConfig.AccountQueue)
		sanitizedConfig.AccountQueue = legacypool.DefaultConfig.AccountQueue
	}
	if sanitizedConfig.GlobalQueue < 1 {
		logger.Warn("Invalid txpool global queue, using default", "provided", sanitizedConfig.GlobalQueue, "default", legacypool.DefaultConfig.GlobalQueue)
		sanitizedConfig.GlobalQueue = legacypool.DefaultConfig.GlobalQueue
	}
	if sanitizedConfig.Lifetime < 1 {
		logger.Warn("Invalid txpool lifetime, using default", "provided", sanitizedConfig.Lifetime, "default", legacypool.DefaultConfig.Lifetime)
		sanitizedConfig.Lifetime = legacypool.DefaultConfig.Lifetime
	}

	mempoolConfig := &evmmempool.EVMMempoolConfig{
		LegacyPoolConfig: &sanitizedConfig,
	}

	return mempoolConfig, nil
}
