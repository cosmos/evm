//go:build !all_precompiles
// +build !all_precompiles

package app

import (
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	ibccallbackskeeper "github.com/cosmos/evm/x/ibc/callbacks/keeper"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	consensuskeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

// Getters of necessary keepers

func (app App) GetIBCKeeper() *ibckeeper.Keeper {
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

func (app App) GetConsensusParamsKeeper() consensuskeeper.Keeper {
	return app.ConsensusParamsKeeper
}

// Getters of optional keepers
//
// These getters expose keepers that the minimum EVM app does not include,
// and some of the existing tests may require one or more of these getters.
// In such cases, you need to add the required keeper(s) to the app first,
// and then implement the corresponding getter(s).
func (app App) GetErc20Keeper() *erc20keeper.Keeper {
	panic("implement me")
}

func (app App) SetErc20Keeper(keeper erc20keeper.Keeper) {
	panic("implement me")
}

func (app App) GetEvidenceKeeper() *evidencekeeper.Keeper {
	panic("implement me")
}

func (app App) GetPreciseBankKeeper() *precisebankkeeper.Keeper {
	panic("implement me")
}

func (app App) GetFeeGrantKeeper() feegrantkeeper.Keeper {
	panic("implement me")
}

func (app App) GetCallbackKeeper() ibccallbackskeeper.ContractKeeper {
	panic("implement me")
}

func (app App) GetTransferKeeper() transferkeeper.Keeper {
	panic("implement me")
}

func (app App) SetTransferKeeper(transferKeeper transferkeeper.Keeper) {
	panic("implement me")
}
