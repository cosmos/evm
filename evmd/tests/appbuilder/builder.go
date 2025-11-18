package appbuilder

import (
	"os"

	evm "github.com/cosmos/evm"
	evmdapp "github.com/cosmos/evm/evmd/app"
	"github.com/cosmos/evm/evmd/tests/integration"

	"cosmossdk.io/log"

	dbm "github.com/cosmos/cosmos-db"
	sdkbaseapp "github.com/cosmos/cosmos-sdk/baseapp"
)

// CreateEvmdWithProfile builds an EVM app using the requested profile.
// For now, only the Base profile is implemented; other profiles will be added
// incrementally.
func CreateEvmdWithProfile(profile Profile, chainID string, evmChainID uint64, opts ...func(*sdkbaseapp.BaseApp)) evm.EvmApp {
	switch profile {
	case Base:
		return createBase(chainID, evmChainID, opts...)
	default:
		panic("profile not implemented yet: " + string(profile))
	}
}

func createBase(chainID string, evmChainID uint64, opts ...func(*sdkbaseapp.BaseApp)) evm.EvmApp {
	defaultNodeHome, err := os.MkdirTemp("", "evmd-temp-homedir")
	if err != nil {
		panic(err)
	}

	db := dbm.NewMemDB()
	logger := log.NewNopLogger()
	loadLatest := true
	appOptions := integration.NewAppOptionsWithFlagHomeAndChainID(defaultNodeHome, evmChainID)

	baseOpts := append(opts, sdkbaseapp.SetChainID(chainID))

	return evmdapp.New(
		logger,
		db,
		nil,
		loadLatest,
		appOptions,
		baseOpts...,
	)
}
