package app

import (
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/cosmos/ibc-go/v10/modules/core/keeper"
)

func (app App) GetIBCKeeper() *keeper.Keeper {
	return app.IBCKeeper
}

func (app App) GetTxConfig() client.TxConfig {
	return app.txConfig
}

func (app *App) GetMempool() mempool.ExtMempool {
	return app.EVMMempool
}

func (app *App) GetAnteHandler() types.AnteHandler {
	return app.AnteHandler()
}

// Keeper getters required by evm.EvmApp interface
func (app *App) GetEVMKeeper() *evmkeeper.Keeper {
	return app.EVMKeeper
}

func (app *App) GetFeeMarketKeeper() *feemarketkeeper.Keeper {
	return &app.FeeMarketKeeper
}

func (app *App) GetGovKeeper() govkeeper.Keeper {
	return app.GovKeeper
}

func (app *App) GetBankKeeper() bankkeeper.Keeper {
	return app.BankKeeper
}

func (app *App) GetAccountKeeper() authkeeper.AccountKeeper {
	return app.AccountKeeper
}

func (app *App) GetStakingKeeper() *stakingkeeper.Keeper {
	return app.StakingKeeper
}

func (app App) GetMintKeeper() mintkeeper.Keeper {
	return app.MintKeeper
}

func (app App) GetSlashingKeeper() slashingkeeper.Keeper {
	return app.SlashingKeeper
}

func (app App) GetDistrKeeper() distrkeeper.Keeper {
	return app.DistributionKeeper
}
