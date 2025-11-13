package testapp

import (
	"fmt"

	evm "github.com/cosmos/evm"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	"github.com/cosmos/evm/x/ibc/callbacks/keeper"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"

	storetypes "cosmossdk.io/store/types"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"

	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

// keeperAdapter wraps a TestApp and implements EvmApp by delegating keeper
// access to optional provider interfaces.
type keeperAdapter struct {
	evm.TestApp
}

var _ evm.EvmApp = (*keeperAdapter)(nil)

// NewKeeperAdapter converts a TestApp that implements only the keepers it
// actually needs into a full EvmApp by delegating keeper access via the
// provider interfaces. Methods panic with a descriptive error if the underlying
// app does not expose the requested keeper.
func NewKeeperAdapter(app evm.TestApp) evm.EvmApp {
	return &keeperAdapter{TestApp: app}
}

func (a *keeperAdapter) GetEVMKeeper() *evmkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.EVMKeeperProvider); ok {
		return provider.GetEVMKeeper()
	}
	panicMissingProvider("EVMKeeperProvider")
	return nil
}

func (a *keeperAdapter) GetErc20Keeper() *erc20keeper.Keeper {
	if provider, ok := a.TestApp.(evm.Erc20KeeperProvider); ok {
		return provider.GetErc20Keeper()
	}
	panicMissingProvider("Erc20KeeperProvider")
	return nil
}

func (a *keeperAdapter) SetErc20Keeper(k erc20keeper.Keeper) {
	if setter, ok := a.TestApp.(evm.Erc20KeeperSetter); ok {
		setter.SetErc20Keeper(k)
		return
	}
	panicMissingProvider("Erc20KeeperSetter")
}

func (a *keeperAdapter) GetGovKeeper() govkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.GovKeeperProvider); ok {
		return provider.GetGovKeeper()
	}
	panicMissingProvider("GovKeeperProvider")
	return govkeeper.Keeper{}
}

func (a *keeperAdapter) GetSlashingKeeper() slashingkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.SlashingKeeperProvider); ok {
		return provider.GetSlashingKeeper()
	}
	panicMissingProvider("SlashingKeeperProvider")
	return slashingkeeper.Keeper{}
}

func (a *keeperAdapter) GetEvidenceKeeper() *evidencekeeper.Keeper {
	if provider, ok := a.TestApp.(evm.EvidenceKeeperProvider); ok {
		return provider.GetEvidenceKeeper()
	}
	panicMissingProvider("EvidenceKeeperProvider")
	return nil
}

func (a *keeperAdapter) GetBankKeeper() bankkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.BankKeeperProvider); ok {
		return provider.GetBankKeeper()
	}
	panicMissingProvider("BankKeeperProvider")
	return bankkeeper.BaseKeeper{}
}

func (a *keeperAdapter) GetFeeMarketKeeper() *feemarketkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.FeeMarketKeeperProvider); ok {
		return provider.GetFeeMarketKeeper()
	}
	panicMissingProvider("FeeMarketKeeperProvider")
	return nil
}

func (a *keeperAdapter) GetAccountKeeper() authkeeper.AccountKeeper {
	if provider, ok := a.TestApp.(evm.AccountKeeperProvider); ok {
		return provider.GetAccountKeeper()
	}
	panicMissingProvider("AccountKeeperProvider")
	return authkeeper.AccountKeeper{}
}

func (a *keeperAdapter) GetDistrKeeper() distrkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.DistrKeeperProvider); ok {
		return provider.GetDistrKeeper()
	}
	panicMissingProvider("DistrKeeperProvider")
	return distrkeeper.Keeper{}
}

func (a *keeperAdapter) GetStakingKeeper() *stakingkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.StakingKeeperProvider); ok {
		return provider.GetStakingKeeper()
	}
	panicMissingProvider("StakingKeeperProvider")
	return nil
}

func (a *keeperAdapter) GetMintKeeper() mintkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.MintKeeperProvider); ok {
		return provider.GetMintKeeper()
	}
	panicMissingProvider("MintKeeperProvider")
	return mintkeeper.Keeper{}
}

func (a *keeperAdapter) GetPreciseBankKeeper() *precisebankkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.PreciseBankKeeperProvider); ok {
		return provider.GetPreciseBankKeeper()
	}
	panicMissingProvider("PreciseBankKeeperProvider")
	return nil
}

func (a *keeperAdapter) GetFeeGrantKeeper() feegrantkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.FeeGrantKeeperProvider); ok {
		return provider.GetFeeGrantKeeper()
	}
	panicMissingProvider("FeeGrantKeeperProvider")
	return feegrantkeeper.Keeper{}
}

func (a *keeperAdapter) GetConsensusParamsKeeper() consensusparamkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.ConsensusParamsKeeperProvider); ok {
		return provider.GetConsensusParamsKeeper()
	}
	panicMissingProvider("ConsensusParamsKeeperProvider")
	return consensusparamkeeper.Keeper{}
}

func (a *keeperAdapter) GetCallbackKeeper() keeper.ContractKeeper {
	if provider, ok := a.TestApp.(evm.CallbackKeeperProvider); ok {
		return provider.GetCallbackKeeper()
	}
	panicMissingProvider("CallbackKeeperProvider")
	return keeper.ContractKeeper{}
}

func (a *keeperAdapter) GetTransferKeeper() transferkeeper.Keeper {
	if provider, ok := a.TestApp.(evm.TransferKeeperProvider); ok {
		return provider.GetTransferKeeper()
	}
	panicMissingProvider("TransferKeeperProvider")
	return transferkeeper.Keeper{}
}

func (a *keeperAdapter) SetTransferKeeper(k transferkeeper.Keeper) {
	if setter, ok := a.TestApp.(evm.TransferKeeperSetter); ok {
		setter.SetTransferKeeper(k)
		return
	}
	panicMissingProvider("TransferKeeperSetter")
}

func (a *keeperAdapter) GetKey(storeKey string) *storetypes.KVStoreKey {
	if provider, ok := a.TestApp.(evm.KeyProvider); ok {
		return provider.GetKey(storeKey)
	}
	return a.TestApp.GetKey(storeKey)
}

func panicMissingProvider(name string) {
	panic(fmt.Sprintf("keeper adapter: app does not implement %s", name))
}
