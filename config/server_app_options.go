package config

import (
	"errors"
	"math"
	"path/filepath"

	"github.com/holiman/uint256"
	"github.com/spf13/cast"

	"cosmossdk.io/log"
	"github.com/cosmos/evm/config/eips"
	srvflags "github.com/cosmos/evm/server/flags"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/config"
	"github.com/cosmos/cosmos-sdk/client/flags"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
)

// GetBlockGasLimit reads the genesis json file using AppGenesisFromFile
// to extract the consensus block gas limit before InitChain is called.
func GetBlockGasLimit(appOpts servertypes.AppOptions, logger log.Logger) uint64 {
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
	minTipUint64 := cast.ToUint64(appOpts.Get(srvflags.EVMMinTip))
	minTip := uint256.NewInt(minTipUint64)

	if minTip.Cmp(uint256.NewInt(0)) >= 0 { // zero or positive
		return minTip
	}

	logger.Error("invalid min tip value in app.toml or flag, falling back to nil", "min_tip", minTipUint64)
	return nil
}

// GetChainID returns the EVM chain ID from the app options, set from
// If not available, it will load from client.toml
func GetChainID(appOpts servertypes.AppOptions) (string, error) {
	chainID := cast.ToString(appOpts.Get(flags.FlagChainID))
	if chainID == "" {
		// if not available, load from client.toml
		homeDir := cast.ToString(appOpts.Get(flags.FlagHome))
		if homeDir == "" {
			return "", errors.New("home directory flag not found in app options")
		}
		clientCtx := client.Context{}.WithHomeDir(homeDir)
		clientCtx, err := config.ReadFromClientConfig(clientCtx)
		if err != nil {
			return "", err
		}
		chainID = clientCtx.ChainID
	}

	return chainID, nil
}

// GetEvmChainID returns the EVM chain ID from the app options, set from flags or app.toml
func GetEvmChainID(appOpts servertypes.AppOptions) (uint64, error) {
	evmChainID := cast.ToUint64(appOpts.Get(srvflags.EVMChainID))
	if evmChainID == 0 {
		return 0, errors.New("evm chain id flag not found in app options")
	}
	return evmChainID, nil
}

func GetEvmCoinInfo(appOpts servertypes.AppOptions) (*evmtypes.EvmCoinInfo, error) {
	displayDenom := cast.ToString(appOpts.Get(srvflags.EVMDisplayDenom))
	if displayDenom == "" {
		return nil, errors.New("display denom flag not found in app options")
	}
	decimals := cast.ToUint8(appOpts.Get(srvflags.EVMDecimals))
	if decimals == 0 {
		return nil, errors.New("decimals flag not found in app options")
	}
	extendedDecimals := cast.ToUint8(appOpts.Get(srvflags.EVMExtendedDecimals))
	if extendedDecimals == 0 {
		return nil, errors.New("extended decimals flag not found in app options")
	}

	evmCoinInfo := evmtypes.EvmCoinInfo{
		DisplayDenom:     displayDenom,
		Decimals:         evmtypes.Decimals(decimals),
		ExtendedDecimals: evmtypes.Decimals(extendedDecimals),
	}
	if err := evmCoinInfo.Validate(); err != nil {
		return nil, err
	}

	return &evmCoinInfo, nil
}

func CreateChainConfig(appOpts servertypes.AppOptions) (*ChainConfig, error) {
	chainID, err := GetChainID(appOpts)
	if err != nil {
		return nil, err
	}
	evmChainID, err := GetEvmChainID(appOpts)
	if err != nil {
		return nil, err
	}
	evmCoinInfo, err := GetEvmCoinInfo(appOpts)
	if err != nil {
		return nil, err
	}

	chainConfig := NewChainConfig(
		chainID,
		evmChainID,
		eips.CosmosEVMActivators,
		nil,
		nil,
		*evmCoinInfo,
		false,
	)

	return &chainConfig, nil
}
