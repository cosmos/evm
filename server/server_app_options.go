package server

import (
	"fmt"
	"math"
	"path/filepath"
	"time"

	"github.com/holiman/uint256"
	"github.com/spf13/cast"

	cmtcfg "github.com/cometbft/cometbft/config"

	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	srvflags "github.com/cosmos/evm/server/flags"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/client/flags"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
)

const (
	cmtMempoolMaxTxBytesKey   = "mempool.max_tx_bytes"
	cmtMempoolReapMaxBytesKey = "mempool.reap_max_bytes"
	cmtMempoolReapMaxGasKey   = "mempool.reap_max_gas"
)

// ValidateReapBounds errors when an admission cap exceeds the matching reap
// cap. A tx admitted via CheckTx that's larger than reap_max_bytes (or whose
// gas exceeds reap_max_gas) would wedge the head of the reap list. A zero reap
// cap means "no limit" on Comet's side — skip.
func ValidateReapBounds(appOpts servertypes.AppOptions, blockGasLimit uint64) error {
	if appOpts == nil {
		return nil
	}

	// Fall back to Comet's default when max_tx_bytes is missing or 0 — viper
	// returns nil for an absent key, cast.ToUint64(nil) is 0, and a 0 cap
	// would silently bypass the comparison even though Comet's effective
	// admission limit is 1 MiB.
	maxTxBytes := cast.ToUint64(appOpts.Get(cmtMempoolMaxTxBytesKey))
	if maxTxBytes == 0 {
		maxTxBytes = uint64(cmtcfg.DefaultMempoolConfig().MaxTxBytes) // #nosec G115 -- comet default is positive (1 MiB)
	}
	reapMaxBytes := cast.ToUint64(appOpts.Get(cmtMempoolReapMaxBytesKey))
	reapMaxGas := cast.ToUint64(appOpts.Get(cmtMempoolReapMaxGasKey))

	if reapMaxBytes > 0 && maxTxBytes > reapMaxBytes {
		return fmt.Errorf(
			"mempool.max_tx_bytes (%d) must be <= mempool.reap_max_bytes (%d): "+
				"a tx admitted via CheckTx that exceeds reap_max_bytes would wedge the reap list",
			maxTxBytes, reapMaxBytes,
		)
	}
	if reapMaxGas > 0 && blockGasLimit > reapMaxGas {
		return fmt.Errorf(
			"genesis consensus block.max_gas (%d) must be <= mempool.reap_max_gas (%d): "+
				"a tx admitted with gas up to block.max_gas that exceeds reap_max_gas would wedge the reap list",
			blockGasLimit, reapMaxGas,
		)
	}
	return nil
}

// ResolveMempoolConfig resolves the mempool configuration from the app options.
func ResolveMempoolConfig(anteHandler sdk.AnteHandler, appOpts servertypes.AppOptions, logger log.Logger) *evmmempool.Config {
	return &evmmempool.Config{
		AnteHandler:              anteHandler,
		LegacyPoolConfig:         GetLegacyPoolConfig(appOpts, logger),
		BlockGasLimit:            GetBlockGasLimit(appOpts, logger),
		MinTip:                   GetMinTip(appOpts, logger),
		PendingTxProposalTimeout: GetPendingTxProposalTimeout(appOpts, logger),
		InsertQueueSize:          GetMempoolInsertQueueSize(appOpts, logger),
		EnableTxTracker:          GetMempoolEnableTxTracker(appOpts),
	}
}

// GetBlockGasLimit reads the genesis json file using AppGenesisFromFile
// to extract the consensus block gas limit before InitChain is called.
func GetBlockGasLimit(appOpts servertypes.AppOptions, logger log.Logger) uint64 {
	if appOpts == nil {
		logger.Error("app options is nil, using max int64 block gas limit")
		return math.MaxInt64
	}

	homeDir := cast.ToString(appOpts.Get(flags.FlagHome))
	if homeDir == "" {
		logger.Error("home directory not found in app options, using max int64 block gas limit")
		return math.MaxInt64
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
		logger.Warn("genesis max_gas is unlimited (-1), using max int64 block gas limit")
		return math.MaxInt64
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
	if appOpts == nil {
		logger.Error("app options is nil, using empty DecCoins")
		return sdk.DecCoins{}
	}

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

// GetLegacyPoolConfig reads the legacy pool configuration from appOpts and overrides
// default values with values from app.toml if they exist and are non-zero.
func GetLegacyPoolConfig(appOpts servertypes.AppOptions, logger log.Logger) *legacypool.Config {
	if appOpts == nil {
		logger.Error("app options is nil, using default mempool config")
		return &legacypool.DefaultConfig
	}

	legacyConfig := legacypool.DefaultConfig
	if priceLimit := cast.ToUint64(appOpts.Get(srvflags.EVMMempoolPriceLimit)); priceLimit != 0 {
		legacyConfig.PriceLimit = priceLimit
	}
	if priceBump := cast.ToUint64(appOpts.Get(srvflags.EVMMempoolPriceBump)); priceBump != 0 {
		legacyConfig.PriceBump = priceBump
	}
	if accountSlots := cast.ToUint64(appOpts.Get(srvflags.EVMMempoolAccountSlots)); accountSlots != 0 {
		legacyConfig.AccountSlots = accountSlots
	}
	if globalSlots := cast.ToUint64(appOpts.Get(srvflags.EVMMempoolGlobalSlots)); globalSlots != 0 {
		legacyConfig.GlobalSlots = globalSlots
	}
	if accountQueue := cast.ToUint64(appOpts.Get(srvflags.EVMMempoolAccountQueue)); accountQueue != 0 {
		legacyConfig.AccountQueue = accountQueue
	}
	if globalQueue := cast.ToUint64(appOpts.Get(srvflags.EVMMempoolGlobalQueue)); globalQueue != 0 {
		legacyConfig.GlobalQueue = globalQueue
	}
	if includedNonceCacheSize := cast.ToInt(appOpts.Get(srvflags.EVMMempoolIncludedNonceCacheSize)); includedNonceCacheSize != 0 {
		legacyConfig.IncludedNonceCacheSize = includedNonceCacheSize
	}
	if lifetime := cast.ToDuration(appOpts.Get(srvflags.EVMMempoolLifetime)); lifetime != 0 {
		legacyConfig.Lifetime = lifetime
	}

	return &legacyConfig
}

func GetPendingTxProposalTimeout(appOpts servertypes.AppOptions, logger log.Logger) time.Duration {
	if appOpts == nil {
		logger.Error("app options is nil, using pending tx proposal timeout of 0 (unlimited)")
		return 0
	}

	return cast.ToDuration(appOpts.Get(srvflags.EVMMempoolPendingTxProposalTimeout))
}

func GetMempoolInsertQueueSize(appOpts servertypes.AppOptions, logger log.Logger) int {
	if appOpts == nil {
		logger.Error("app options is nil, using insert queue size of 5000")
		return 5000
	}

	return cast.ToInt(appOpts.Get(srvflags.EVMMempoolInsertQueueSize))
}

// GetMempoolEnableTxTracker reads whether per-tx lifecycle telemetry should be
// recorded by the mempool. Defaults to false when not set or appOpts is nil.
func GetMempoolEnableTxTracker(appOpts servertypes.AppOptions) bool {
	if appOpts == nil {
		return false
	}
	return cast.ToBool(appOpts.Get(srvflags.EVMMempoolEnableTxTracker))
}

func GetMempoolCheckTxTimeout(appOpts servertypes.AppOptions, logger log.Logger) time.Duration {
	if appOpts == nil {
		logger.Error("app options is nil, using check tx timeout of 5 seconds")
		return 5 * time.Second
	}

	dur := cast.ToDuration(appOpts.Get(srvflags.EVMMempoolCheckTxTimeout))
	if dur <= 0 {
		logger.Error("check tx timeout must be greater than 0, using 5 seconds")
		return 5 * time.Second
	}

	return dur
}

func GetCosmosPoolMaxTx(appOpts servertypes.AppOptions, logger log.Logger) int {
	if appOpts == nil {
		// we don't want to return 0 here, as then appOpts.Get() will return nil and that will be
		// "accidentally" cast to the correct evm max tx default of 0, thereby hiding the error
		logger.Error("app options is nil, using sdk max tx default of -1 (no-op)")
		return sdkmempool.DefaultMaxTx
	}

	return cast.ToInt(appOpts.Get(sdkserver.FlagMempoolMaxTxs))
}
