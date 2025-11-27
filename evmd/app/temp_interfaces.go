package app

import (
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/ibc/callbacks/keeper"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
)

func (app App) GetErc20Keeper() *erc20keeper.Keeper {
	//TODO implement me
	panic("implement me")
}

func (app App) SetErc20Keeper(keeper erc20keeper.Keeper) {
	//TODO implement me
	panic("implement me")
}

func (app App) GetEvidenceKeeper() *evidencekeeper.Keeper {
	//TODO implement me
	panic("implement me")
}

func (app App) GetAuthzKeeper() authzkeeper.Keeper {
	//TODO implement me
	panic("implement me")
}

func (app App) GetPreciseBankKeeper() *precisebankkeeper.Keeper {
	//TODO implement me
	panic("implement me")
}

func (app App) GetFeeGrantKeeper() feegrantkeeper.Keeper {
	//TODO implement me
	panic("implement me")
}

func (app App) GetCallbackKeeper() keeper.ContractKeeper {
	//TODO implement me
	panic("implement me")
}

func (app App) GetTransferKeeper() transferkeeper.Keeper {
	//TODO implement me
	panic("implement me")
}

func (app App) SetTransferKeeper(transferKeeper transferkeeper.Keeper) {
	//TODO implement me
	panic("implement me")
}
