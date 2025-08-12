package config

import (
	"fmt"

	"github.com/ethereum/go-ethereum/core/vm"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EVMOptionsFn defines a function type for setting app options specifically for
// the Cosmos EVM app. The function should receive the chainID and return an error if
// any.
type EVMOptionsFn func(uint64) error

var sealed = false

func EvmAppOptionsWithConfig(
	chainID uint64,
	chainsCoinInfo map[uint64]evmtypes.EvmCoinInfo,
	cosmosEVMActivators map[int]func(*vm.JumpTable),
) error {
	if sealed {
		return nil
	}

	if err := EvmAppOptionsWithConfigWithReset(chainID, chainsCoinInfo, cosmosEVMActivators, false); err != nil {
		return err
	}

	sealed = true
	return nil
}

func EvmAppOptionsWithConfigWithReset(
	chainID uint64,
	chainsCoinInfo map[uint64]evmtypes.EvmCoinInfo,
	cosmosEVMActivators map[int]func(*vm.JumpTable),
	withReset bool,
) error {
	coinInfo, found := chainsCoinInfo[chainID]
	if !found {
		return fmt.Errorf("unknown chain id: %d", chainID)
	}

	// set the denom info for the chain
	if err := setBaseDenom(coinInfo); err != nil {
		return err
	}

	ethCfg := evmtypes.DefaultChainConfig(chainID)
	configurator := evmtypes.NewEVMConfigurator()
	if withReset {
		// reset configuration to set the new one
		configurator.ResetTestConfig()
	}
	err := configurator.
		WithExtendedEips(cosmosEVMActivators).
		WithChainConfig(ethCfg).
		// NOTE: we're using the 18 decimals default for the example chain
		WithEVMCoinInfo(coinInfo).
		Configure()
	if err != nil {
		return err
	}

	return nil
}

// setBaseDenom registers the display denom and base denom and sets the
// base denom for the chain. The function registered different values based on
// the EvmCoinInfo to allow different configurations in mainnet and testnet.
func setBaseDenom(ci evmtypes.EvmCoinInfo) (err error) {
	// Defer setting the base denom, and capture any potential error from it.
	// So when failing because the denom was already registered, we ignore it and set
	// the corresponding denom to be base denom
	defer func() {
		err = sdk.SetBaseDenom(ci.Denom)
	}()
	if err := sdk.RegisterDenom(ci.DisplayDenom, math.LegacyOneDec()); err != nil {
		return err
	}

	// sdk.RegisterDenom will automatically overwrite the base denom when the
	// new setBaseDenom() units are lower than the current base denom's units.
	return sdk.RegisterDenom(ci.Denom, math.LegacyNewDecWithPrec(1, int64(ci.Decimals)))
}
