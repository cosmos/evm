package types

import (
	"errors"
	"math/big"

	gethparams "github.com/ethereum/go-ethereum/params"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
)

// testChainID represents the ChainID used for the purpose of testing.
const testChainID uint64 = 262144

// chainConfig is the chain configuration used in the EVM to defined which
// opcodes are active based on Ethereum upgrades.
var chainConfig *ChainConfig

// EthereumConfig returns an Ethereum ChainConfig for EVM state transitions.
// All the negative or nil values are converted to nil
func (cc ChainConfig) EthereumConfig(chainID *big.Int) *gethparams.ChainConfig {
	cID := new(big.Int).SetUint64(cc.ChainId)
	if chainID != nil {
		cID = chainID
	}
	return &gethparams.ChainConfig{
		ChainID:                 cID,
		HomesteadBlock:          getBlockValue(cc.HomesteadBlock),
		DAOForkBlock:            getBlockValue(cc.DAOForkBlock),
		DAOForkSupport:          cc.DAOForkSupport,
		EIP150Block:             getBlockValue(cc.EIP150Block),
		EIP155Block:             getBlockValue(cc.EIP155Block),
		EIP158Block:             getBlockValue(cc.EIP158Block),
		ByzantiumBlock:          getBlockValue(cc.ByzantiumBlock),
		ConstantinopleBlock:     getBlockValue(cc.ConstantinopleBlock),
		PetersburgBlock:         getBlockValue(cc.PetersburgBlock),
		IstanbulBlock:           getBlockValue(cc.IstanbulBlock),
		MuirGlacierBlock:        getBlockValue(cc.MuirGlacierBlock),
		BerlinBlock:             getBlockValue(cc.BerlinBlock),
		LondonBlock:             getBlockValue(cc.LondonBlock),
		ArrowGlacierBlock:       getBlockValue(cc.ArrowGlacierBlock),
		GrayGlacierBlock:        getBlockValue(cc.GrayGlacierBlock),
		MergeNetsplitBlock:      getBlockValue(cc.MergeNetsplitBlock),
		ShanghaiTime:            getTimestampValue(cc.ShanghaiTime),
		CancunTime:              getTimestampValue(cc.CancunTime),
		PragueTime:              getTimestampValue(cc.PragueTime),
		OsakaTime:               getTimestampValue(cc.OsakaTime),
		VerkleTime:              getTimestampValue(cc.VerkleTime),
		TerminalTotalDifficulty: nil,
		Ethash:                  nil,
		Clique:                  nil,
		BlobScheduleConfig: &gethparams.BlobScheduleConfig{
			Cancun: gethparams.DefaultCancunBlobConfig,
			Prague: gethparams.DefaultPragueBlobConfig,
			Osaka:  gethparams.DefaultOsakaBlobConfig,
		},
	}
}

func DefaultChainConfig(evmChainID uint64) *ChainConfig {
	if evmChainID == 0 {
		evmChainID = testChainID
	}

	homesteadBlock := sdkmath.ZeroInt()
	daoForkBlock := sdkmath.ZeroInt()
	eip150Block := sdkmath.ZeroInt()
	eip155Block := sdkmath.ZeroInt()
	eip158Block := sdkmath.ZeroInt()
	byzantiumBlock := sdkmath.ZeroInt()
	constantinopleBlock := sdkmath.ZeroInt()
	petersburgBlock := sdkmath.ZeroInt()
	istanbulBlock := sdkmath.ZeroInt()
	muirGlacierBlock := sdkmath.ZeroInt()
	berlinBlock := sdkmath.ZeroInt()
	londonBlock := sdkmath.ZeroInt()
	arrowGlacierBlock := sdkmath.ZeroInt()
	grayGlacierBlock := sdkmath.ZeroInt()
	mergeNetsplitBlock := sdkmath.ZeroInt()
	shanghaiTime := sdkmath.ZeroInt()
	cancunTime := sdkmath.ZeroInt()
	pragueTime := sdkmath.ZeroInt()

	cfg := &ChainConfig{
		ChainId:             evmChainID,
		Denom:               DefaultEVMDenom,
		Decimals:            DefaultEVMDecimals,
		HomesteadBlock:      &homesteadBlock,
		DAOForkBlock:        &daoForkBlock,
		DAOForkSupport:      true,
		EIP150Block:         &eip150Block,
		EIP155Block:         &eip155Block,
		EIP158Block:         &eip158Block,
		ByzantiumBlock:      &byzantiumBlock,
		ConstantinopleBlock: &constantinopleBlock,
		PetersburgBlock:     &petersburgBlock,
		IstanbulBlock:       &istanbulBlock,
		MuirGlacierBlock:    &muirGlacierBlock,
		BerlinBlock:         &berlinBlock,
		LondonBlock:         &londonBlock,
		ArrowGlacierBlock:   &arrowGlacierBlock,
		GrayGlacierBlock:    &grayGlacierBlock,
		MergeNetsplitBlock:  &mergeNetsplitBlock,
		ShanghaiTime:        &shanghaiTime,
		CancunTime:          &cancunTime,
		PragueTime:          &pragueTime,
		OsakaTime:           nil,
		VerkleTime:          nil,
	}
	return cfg
}

// setChainConfig allows to set the `chainConfig` variable modifying the
// default values. The method is private because it should only be called once
// in the EVMConfigurator.
func setChainConfig(cc *ChainConfig) error {
	if chainConfig != nil {
		return errors.New("chainConfig already set. Cannot set again the chainConfig")
	}
	config := DefaultChainConfig(0)
	if cc != nil {
		config = cc
	}
	if err := config.Validate(); err != nil {
		return err
	}
	chainConfig = config

	return nil
}

func getBlockValue(block *sdkmath.Int) *big.Int {
	if block == nil || block.IsNegative() {
		return nil
	}

	return block.BigInt()
}

func getTimestampValue(ts *sdkmath.Int) *uint64 {
	if ts == nil || ts.IsNegative() {
		return nil
	}
	res := ts.Uint64()
	return &res
}

// Validate performs a basic validation of the ChainConfig params. The function will return an error
// if any of the block values is uninitialized (i.e nil) or if the EIP150Hash is an invalid hash.
func (cc ChainConfig) Validate() error {
	if err := validateBlockOrTimestamp(cc.HomesteadBlock); err != nil {
		return errorsmod.Wrap(err, "homesteadBlock")
	}
	if err := validateBlockOrTimestamp(cc.DAOForkBlock); err != nil {
		return errorsmod.Wrap(err, "daoForkBlock")
	}
	if err := validateBlockOrTimestamp(cc.EIP150Block); err != nil {
		return errorsmod.Wrap(err, "eip150Block")
	}
	if err := validateBlockOrTimestamp(cc.EIP155Block); err != nil {
		return errorsmod.Wrap(err, "eip155Block")
	}
	if err := validateBlockOrTimestamp(cc.EIP158Block); err != nil {
		return errorsmod.Wrap(err, "eip158Block")
	}
	if err := validateBlockOrTimestamp(cc.ByzantiumBlock); err != nil {
		return errorsmod.Wrap(err, "byzantiumBlock")
	}
	if err := validateBlockOrTimestamp(cc.ConstantinopleBlock); err != nil {
		return errorsmod.Wrap(err, "constantinopleBlock")
	}
	if err := validateBlockOrTimestamp(cc.PetersburgBlock); err != nil {
		return errorsmod.Wrap(err, "petersburgBlock")
	}
	if err := validateBlockOrTimestamp(cc.IstanbulBlock); err != nil {
		return errorsmod.Wrap(err, "istanbulBlock")
	}
	if err := validateBlockOrTimestamp(cc.MuirGlacierBlock); err != nil {
		return errorsmod.Wrap(err, "muirGlacierBlock")
	}
	if err := validateBlockOrTimestamp(cc.BerlinBlock); err != nil {
		return errorsmod.Wrap(err, "berlinBlock")
	}
	if err := validateBlockOrTimestamp(cc.LondonBlock); err != nil {
		return errorsmod.Wrap(err, "londonBlock")
	}
	if err := validateBlockOrTimestamp(cc.ArrowGlacierBlock); err != nil {
		return errorsmod.Wrap(err, "arrowGlacierBlock")
	}
	if err := validateBlockOrTimestamp(cc.GrayGlacierBlock); err != nil {
		return errorsmod.Wrap(err, "GrayGlacierBlock")
	}
	if err := validateBlockOrTimestamp(cc.MergeNetsplitBlock); err != nil {
		return errorsmod.Wrap(err, "MergeNetsplitBlock")
	}
	if err := validateBlockOrTimestamp(cc.ShanghaiTime); err != nil {
		return errorsmod.Wrap(err, "ShanghaiTime")
	}
	if err := validateBlockOrTimestamp(cc.CancunTime); err != nil {
		return errorsmod.Wrap(err, "CancunTime")
	}
	if err := validateBlockOrTimestamp(cc.PragueTime); err != nil {
		return errorsmod.Wrap(err, "PragueTime")
	}
	if err := validateBlockOrTimestamp(cc.OsakaTime); err != nil {
		return errorsmod.Wrap(err, "OsakaTime")
	}
	if err := validateBlockOrTimestamp(cc.VerkleTime); err != nil {
		return errorsmod.Wrap(err, "VerkleTime")
	}
	// NOTE: chain ID is not needed to check config order
	if err := cc.EthereumConfig(nil).CheckConfigForkOrder(); err != nil {
		return errorsmod.Wrap(err, "invalid config fork order")
	}
	return nil
}

func validateBlockOrTimestamp(value *sdkmath.Int) error {
	// nil value means that the fork has not yet been applied
	if value == nil {
		return nil
	}

	if value.IsNegative() {
		return errorsmod.Wrapf(
			ErrInvalidChainConfig, "block or timestamp value cannot be negative: %s", value,
		)
	}

	return nil
}
