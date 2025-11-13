package network

import (
	"fmt"

	"github.com/cosmos/evm"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"

	evidencekeeper "cosmossdk.io/x/evidence/keeper"

	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

func mustGetEVMKeeper(app evm.TestApp) *evmkeeper.Keeper {
	if provider, ok := app.(evm.EVMKeeperProvider); ok {
		return provider.GetEVMKeeper()
	}
	panicMissing("EVMKeeperProvider")
	return nil
}

func mustGetErc20Keeper(app evm.TestApp) *erc20keeper.Keeper {
	if provider, ok := app.(evm.Erc20KeeperProvider); ok {
		return provider.GetErc20Keeper()
	}
	panicMissing("Erc20KeeperProvider")
	return nil
}

func mustGetGovKeeper(app evm.TestApp) govkeeper.Keeper {
	if provider, ok := app.(evm.GovKeeperProvider); ok {
		return provider.GetGovKeeper()
	}
	panicMissing("GovKeeperProvider")
	return govkeeper.Keeper{}
}

func mustGetBankKeeper(app evm.TestApp) bankkeeper.Keeper {
	if provider, ok := app.(evm.BankKeeperProvider); ok {
		return provider.GetBankKeeper()
	}
	panicMissing("BankKeeperProvider")
	return bankkeeper.BaseKeeper{}
}

func mustGetFeeMarketKeeper(app evm.TestApp) *feemarketkeeper.Keeper {
	if provider, ok := app.(evm.FeeMarketKeeperProvider); ok {
		return provider.GetFeeMarketKeeper()
	}
	panicMissing("FeeMarketKeeperProvider")
	return nil
}

func mustGetAccountKeeper(app evm.TestApp) authkeeper.AccountKeeper {
	if provider, ok := app.(evm.AccountKeeperProvider); ok {
		return provider.GetAccountKeeper()
	}
	panicMissing("AccountKeeperProvider")
	return authkeeper.AccountKeeper{}
}

func mustGetStakingKeeper(app evm.TestApp) *stakingkeeper.Keeper {
	if provider, ok := app.(evm.StakingKeeperProvider); ok {
		return provider.GetStakingKeeper()
	}
	panicMissing("StakingKeeperProvider")
	return nil
}

func mustGetDistrKeeper(app evm.TestApp) distrkeeper.Keeper {
	if provider, ok := app.(evm.DistrKeeperProvider); ok {
		return provider.GetDistrKeeper()
	}
	panicMissing("DistrKeeperProvider")
	return distrkeeper.Keeper{}
}

func mustGetMintKeeper(app evm.TestApp) mintkeeper.Keeper {
	if provider, ok := app.(evm.MintKeeperProvider); ok {
		return provider.GetMintKeeper()
	}
	panicMissing("MintKeeperProvider")
	return mintkeeper.Keeper{}
}

func mustGetPreciseBankKeeper(app evm.TestApp) *precisebankkeeper.Keeper {
	if provider, ok := app.(evm.PreciseBankKeeperProvider); ok {
		return provider.GetPreciseBankKeeper()
	}
	panicMissing("PreciseBankKeeperProvider")
	return nil
}

func mustGetEvidenceKeeper(app evm.TestApp) *evidencekeeper.Keeper {
	if provider, ok := app.(evm.EvidenceKeeperProvider); ok {
		return provider.GetEvidenceKeeper()
	}
	panicMissing("EvidenceKeeperProvider")
	return nil
}

func mustGetSlashingKeeper(app evm.TestApp) slashingkeeper.Keeper {
	if provider, ok := app.(evm.SlashingKeeperProvider); ok {
		return provider.GetSlashingKeeper()
	}
	panicMissing("SlashingKeeperProvider")
	return slashingkeeper.Keeper{}
}

func panicMissing(name string) {
	panic(fmt.Sprintf("network: application does not implement %s", name))
}
