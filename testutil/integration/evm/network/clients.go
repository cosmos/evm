package network

import (
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module/testutil"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func getQueryHelper(ctx sdktypes.Context, encCfg testutil.TestEncodingConfig) *baseapp.QueryServiceTestHelper {
	interfaceRegistry := encCfg.InterfaceRegistry
	// This is needed so that state changes are not committed in precompiles
	// simulations.
	cacheCtx, _ := ctx.CacheContext()
	return baseapp.NewQueryServerTestHelper(cacheCtx, interfaceRegistry)
}

func (n *IntegrationNetwork) GetERC20Client() erc20types.QueryClient {
	queryHelper := getQueryHelper(n.GetContext(), n.GetEncodingConfig())
	erc20types.RegisterQueryServer(queryHelper, mustGetErc20Keeper(n.app))
	return erc20types.NewQueryClient(queryHelper)
}

func (n *IntegrationNetwork) GetEvmClient() evmtypes.QueryClient {
	queryHelper := getQueryHelper(n.GetContext(), n.GetEncodingConfig())
	evmtypes.RegisterQueryServer(queryHelper, mustGetEVMKeeper(n.app))
	return evmtypes.NewQueryClient(queryHelper)
}

func (n *IntegrationNetwork) GetGovClient() govtypes.QueryClient {
	queryHelper := getQueryHelper(n.GetContext(), n.GetEncodingConfig())
	govKeeper := mustGetGovKeeper(n.app)
	govtypes.RegisterQueryServer(queryHelper, govkeeper.NewQueryServer(&govKeeper))
	return govtypes.NewQueryClient(queryHelper)
}

func (n *IntegrationNetwork) GetBankClient() banktypes.QueryClient {
	queryHelper := getQueryHelper(n.GetContext(), n.GetEncodingConfig())
	banktypes.RegisterQueryServer(queryHelper, mustGetBankKeeper(n.app))
	return banktypes.NewQueryClient(queryHelper)
}

func (n *IntegrationNetwork) GetFeeMarketClient() feemarkettypes.QueryClient {
	queryHelper := getQueryHelper(n.GetContext(), n.GetEncodingConfig())
	feemarkettypes.RegisterQueryServer(queryHelper, mustGetFeeMarketKeeper(n.app))
	return feemarkettypes.NewQueryClient(queryHelper)
}

func (n *IntegrationNetwork) GetAuthClient() authtypes.QueryClient {
	queryHelper := getQueryHelper(n.GetContext(), n.GetEncodingConfig())
	authtypes.RegisterQueryServer(queryHelper, authkeeper.NewQueryServer(mustGetAccountKeeper(n.app)))
	return authtypes.NewQueryClient(queryHelper)
}

func (n *IntegrationNetwork) GetStakingClient() stakingtypes.QueryClient {
	queryHelper := getQueryHelper(n.GetContext(), n.GetEncodingConfig())
	stakingtypes.RegisterQueryServer(queryHelper, stakingkeeper.Querier{Keeper: mustGetStakingKeeper(n.app)})
	return stakingtypes.NewQueryClient(queryHelper)
}

func (n *IntegrationNetwork) GetDistrClient() distrtypes.QueryClient {
	queryHelper := getQueryHelper(n.GetContext(), n.GetEncodingConfig())
	distrtypes.RegisterQueryServer(queryHelper, distrkeeper.Querier{Keeper: mustGetDistrKeeper(n.app)})
	return distrtypes.NewQueryClient(queryHelper)
}

func (n *IntegrationNetwork) GetMintClient() minttypes.QueryClient {
	queryHelper := getQueryHelper(n.GetContext(), n.GetEncodingConfig())
	minttypes.RegisterQueryServer(queryHelper, mintkeeper.NewQueryServerImpl(mustGetMintKeeper(n.app)))
	return minttypes.NewQueryClient(queryHelper)
}

func (n *IntegrationNetwork) GetPreciseBankClient() precisebanktypes.QueryClient {
	queryHelper := getQueryHelper(n.GetContext(), n.GetEncodingConfig())
	precisebanktypes.RegisterQueryServer(queryHelper, precisebankkeeper.NewQueryServerImpl(*mustGetPreciseBankKeeper(n.app)))
	return precisebanktypes.NewQueryClient(queryHelper)
}
