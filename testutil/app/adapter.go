package testapp

import (
	"encoding/json"
	"fmt"

	evm "github.com/cosmos/evm"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	"github.com/cosmos/evm/x/ibc/callbacks/keeper"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	storetypes "cosmossdk.io/store/types"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"

	"github.com/cosmos/cosmos-sdk/baseapp"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

// NewEvmAppAdapter wraps a specialized TestApp (one that implements the keeper
// provider interfaces) into a full evm.EvmApp so shared testing helpers can
// keep using the broader interface.
func NewEvmAppAdapter(app evm.TestApp) *EvmAppAdapter {
	return &EvmAppAdapter{TestApp: app}
}

// ToEvmAppCreator validates that the provided factory returns an app
// implementing the desired interface T and then wraps it behind the keeper
// adapter so downstream helpers can keep using evm.EvmApp.
func ToEvmAppCreator[T any](create func(string, uint64, ...func(*baseapp.BaseApp)) evm.EvmApp, ifaceName string) func(string, uint64, ...func(*baseapp.BaseApp)) evm.EvmApp {
	return func(chainID string, evmChainID uint64, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.EvmApp {
		app := create(chainID, evmChainID, customBaseAppOptions...)
		if _, ok := app.(T); !ok {
			panic(fmt.Sprintf("CreateEvmApp must implement %s", ifaceName))
		}
		return NewEvmAppAdapter(app)
	}
}

// ToIBCAppCreator adapts an ibctesting.AppCreator into one that
// guarantees the returned app implements the desired interface T and exposes
// the evm.EvmApp API via the testing adapter.
func ToIBCAppCreator[T any](creator ibctesting.AppCreator, ifaceName string) ibctesting.AppCreator {
	return func() (ibctesting.TestingApp, map[string]json.RawMessage) {
		app, genesis := creator()
		typedApp, ok := app.(evm.TestApp)
		if !ok {
			panic("AppCreator must return an app implementing evm.TestApp")
		}
		if _, ok := app.(T); !ok {
			panic(fmt.Sprintf("AppCreator must implement %s", ifaceName))
		}
		if _, ok := app.(evm.IBCTestApp); !ok {
			panic("AppCreator must return an app implementing evm.IBCTestApp")
		}
		return NewEvmAppAdapter(typedApp), genesis
	}
}

type EvmAppAdapter struct {
	evm.TestApp
}

var _ evm.EvmApp = (*EvmAppAdapter)(nil)

func (a *EvmAppAdapter) GetEVMKeeper() *evmkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.VMKeeperProvider); ok {
		return provider.GetEVMKeeper()
	}
	panicMissingProvider("EVMKeeperProvider")
	return nil
}

func (a *EvmAppAdapter) GetErc20Keeper() *erc20keeper.Keeper {
	if provider, ok := a.TestApp.(evm.Erc20KeeperProvider); ok {
		return provider.GetErc20Keeper()
	}
	panicMissingProvider("Erc20KeeperProvider")
	return nil
}

func (a *EvmAppAdapter) SetErc20Keeper(k erc20keeper.Keeper) {
	if setter, ok := a.TestApp.(evm.Erc20KeeperSetter); ok {
		setter.SetErc20Keeper(k)
		return
	}
	panicMissingProvider("Erc20KeeperSetter")
}

func (a *EvmAppAdapter) GetGovKeeper() govkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.GovKeeperProvider); ok {
		return provider.GetGovKeeper()
	}
	panicMissingProvider("GovKeeperProvider")
	return govkeeper.Keeper{}
}

func (a *EvmAppAdapter) GetSlashingKeeper() slashingkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.SlashingKeeperProvider); ok {
		return provider.GetSlashingKeeper()
	}
	panicMissingProvider("SlashingKeeperProvider")
	return slashingkeeper.Keeper{}
}

func (a *EvmAppAdapter) GetIBCKeeper() *ibckeeper.Keeper {
	if provider, ok := a.TestApp.(evm.IBCKeeperProvider); ok {
		return provider.GetIBCKeeper()
	}
	panicMissingProvider("IBCKeeperProvider")
	return nil
}

func (a *EvmAppAdapter) GetEvidenceKeeper() *evidencekeeper.Keeper {
	if provider, ok := a.TestApp.(evm.EvidenceKeeperProvider); ok {
		return provider.GetEvidenceKeeper()
	}
	panicMissingProvider("EvidenceKeeperProvider")
	return nil
}

func (a *EvmAppAdapter) GetBankKeeper() bankkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.BankKeeperProvider); ok {
		return provider.GetBankKeeper()
	}
	panicMissingProvider("BankKeeperProvider")
	return bankkeeper.BaseKeeper{}
}

func (a *EvmAppAdapter) GetFeeMarketKeeper() *feemarketkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.FeeMarketKeeperProvider); ok {
		return provider.GetFeeMarketKeeper()
	}
	panicMissingProvider("FeeMarketKeeperProvider")
	return nil
}

func (a *EvmAppAdapter) GetAccountKeeper() authkeeper.AccountKeeper {
	if provider, ok := a.TestApp.(evm.AccountKeeperProvider); ok {
		return provider.GetAccountKeeper()
	}
	panicMissingProvider("AccountKeeperProvider")
	return authkeeper.AccountKeeper{}
}

func (a *EvmAppAdapter) GetDistrKeeper() distrkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.DistrKeeperProvider); ok {
		return provider.GetDistrKeeper()
	}
	panicMissingProvider("DistrKeeperProvider")
	return distrkeeper.Keeper{}
}

func (a *EvmAppAdapter) GetStakingKeeper() *stakingkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.StakingKeeperProvider); ok {
		return provider.GetStakingKeeper()
	}
	panicMissingProvider("StakingKeeperProvider")
	return nil
}

func (a *EvmAppAdapter) GetMintKeeper() mintkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.MintKeeperProvider); ok {
		return provider.GetMintKeeper()
	}
	panicMissingProvider("MintKeeperProvider")
	return mintkeeper.Keeper{}
}

func (a *EvmAppAdapter) GetPreciseBankKeeper() *precisebankkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.PreciseBankKeeperProvider); ok {
		return provider.GetPreciseBankKeeper()
	}
	panicMissingProvider("PreciseBankKeeperProvider")
	return nil
}

func (a *EvmAppAdapter) GetFeeGrantKeeper() feegrantkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.FeeGrantKeeperProvider); ok {
		return provider.GetFeeGrantKeeper()
	}
	panicMissingProvider("FeeGrantKeeperProvider")
	return feegrantkeeper.Keeper{}
}

func (a *EvmAppAdapter) GetConsensusParamsKeeper() consensusparamkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.ConsensusParamsKeeperProvider); ok {
		return provider.GetConsensusParamsKeeper()
	}
	panicMissingProvider("ConsensusParamsKeeperProvider")
	return consensusparamkeeper.Keeper{}
}

func (a *EvmAppAdapter) GetCallbackKeeper() keeper.ContractKeeper {
	if provider, ok := a.TestApp.(evm.CallbackKeeperProvider); ok {
		return provider.GetCallbackKeeper()
	}
	panicMissingProvider("CallbackKeeperProvider")
	return keeper.ContractKeeper{}
}

func (a *EvmAppAdapter) GetTransferKeeper() transferkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.TransferKeeperProvider); ok {
		return provider.GetTransferKeeper()
	}
	panicMissingProvider("TransferKeeperProvider")
	return transferkeeper.Keeper{}
}

func (a *EvmAppAdapter) SetTransferKeeper(k transferkeeper.Keeper) {
	if setter, ok := a.TestApp.(evm.TransferKeeperSetter); ok {
		setter.SetTransferKeeper(k)
		return
	}
	panicMissingProvider("TransferKeeperSetter")
}

func (a *EvmAppAdapter) GetKey(storeKey string) *storetypes.KVStoreKey {
	if provider, ok := a.TestApp.(evm.KeyProvider); ok {
		return provider.GetKey(storeKey)
	}
	return a.TestApp.GetKey(storeKey)
}

func panicMissingProvider(name string) {
	panic(fmt.Sprintf("keeper adapter: app does not implement %s", name))
}
