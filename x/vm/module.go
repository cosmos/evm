package vm

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/x/vm/client/cli"
	"github.com/cosmos/evm/x/vm/keeper"
	"github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/core/address"
	"cosmossdk.io/core/appmodule"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

// consensusVersion defines the current x/evm module consensus version.
const consensusVersion = 1

var (
	_ module.AppModuleBasic = AppModuleBasic{}
	_ module.HasABCIGenesis = AppModule{}

	_ appmodule.HasBeginBlocker = AppModule{}
	_ appmodule.HasEndBlocker   = AppModule{}
	_ appmodule.HasPreBlocker   = AppModule{}
)

// AppModuleBasic defines the basic application module used by the evm module.
type AppModuleBasic struct {
	ac address.Codec
}

// Name returns the evm module's name.
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers the module's types with the given codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	types.RegisterLegacyAminoCodec(cdc)
}

// ConsensusVersion returns the consensus state-breaking version for the module.
func (AppModuleBasic) ConsensusVersion() uint64 {
	return consensusVersion
}

// DefaultGenesis returns default genesis state as raw bytes for the evm
// module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	return cdc.MustMarshalJSON(types.DefaultGenesisState())
}

// ValidateGenesis is the validation check of the Genesis
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, _ client.TxEncodingConfig, bz json.RawMessage) error {
	var genesisState types.GenesisState
	if err := cdc.UnmarshalJSON(bz, &genesisState); err != nil {
		return fmt.Errorf("failed to unmarshal %s genesis state: %w", types.ModuleName, err)
	}

	return genesisState.Validate()
}

// RegisterRESTRoutes performs a no-op as the EVM module doesn't expose REST
// endpoints
func (AppModuleBasic) RegisterRESTRoutes(_ client.Context, _ *mux.Router) {
}

func (b AppModuleBasic) RegisterGRPCGatewayRoutes(c client.Context, serveMux *runtime.ServeMux) {
	if err := types.RegisterQueryHandlerClient(context.Background(), serveMux, types.NewQueryClient(c)); err != nil {
		panic(err)
	}
}

// RegisterInterfaces registers interfaces and implementations of the evm module.
func (AppModuleBasic) RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	types.RegisterInterfaces(registry)
}

// GetTxCmd returns the root tx command for the erc20 module.
func (b AppModuleBasic) GetTxCmd() *cobra.Command {
	return cli.NewTxCmd(b.ac)
}

// GetQueryCmd returns the root query command for the erc20 module.
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return cli.GetQueryCmd()
}

// ____________________________________________________________________________

// AppModule implements an application module for the evm module.
type AppModule struct {
	AppModuleBasic
	keeper      *keeper.Keeper
	ak          types.AccountKeeper
	bankKeeper  types.BankKeeper
	initializer *sync.Once
}

// NewAppModule creates a new AppModule object
func NewAppModule(k *keeper.Keeper, ak types.AccountKeeper, bankKeeper types.BankKeeper, ac address.Codec) AppModule {
	return AppModule{
		AppModuleBasic: AppModuleBasic{ac: ac},
		keeper:         k,
		ak:             ak,
		bankKeeper:     bankKeeper,
		initializer:    &sync.Once{},
	}
}

// Name returns the evm module's name.
func (AppModule) Name() string {
	return types.ModuleName
}

// RegisterServices registers a GRPC query service to respond to the
// module-specific GRPC queries.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterMsgServer(cfg.MsgServer(), am.keeper)
	types.RegisterQueryServer(cfg.QueryServer(), am.keeper)
}

func (am AppModule) PreBlock(goCtx context.Context) (appmodule.ResponsePreBlock, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)
	coinInfo := am.keeper.GetEvmCoinInfo(ctx)
	am.initializer.Do(func() {
		SetGlobalConfigVariables(coinInfo)
	})
	return &sdk.ResponsePreBlock{ConsensusParamsChanged: false}, nil
}

// BeginBlock returns the begin blocker for the evm module.
func (am AppModule) BeginBlock(ctx context.Context) error {
	c := sdk.UnwrapSDKContext(ctx)
	return am.keeper.BeginBlock(c)
}

// EndBlock returns the end blocker for the evm module. It returns no validator
// updates.
func (am AppModule) EndBlock(ctx context.Context) error {
	c := sdk.UnwrapSDKContext(ctx)
	return am.keeper.EndBlock(c)
}

// InitGenesis performs genesis initialization for the evm module. It returns
// no validator updates.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, data json.RawMessage) []abci.ValidatorUpdate {
	var genesisState types.GenesisState
	cdc.MustUnmarshalJSON(data, &genesisState)
	InitGenesis(ctx, am.keeper, am.ak, am.bankKeeper, genesisState, am.initializer)
	return []abci.ValidatorUpdate{}
}

// ExportGenesis returns the exported genesis state as raw bytes for the evm
// module.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := ExportGenesis(ctx, am.keeper)
	return cdc.MustMarshalJSON(gs)
}

// RegisterStoreDecoder registers a decoder for evm module's types
func (am AppModule) RegisterStoreDecoder(_ simtypes.StoreDecoderRegistry) {
}

// GenerateGenesisState creates a randomized GenState of the evm module.
func (AppModule) GenerateGenesisState(_ *module.SimulationState) {
}

// WeightedOperations returns the all the evm module operations with their respective weights.
func (am AppModule) WeightedOperations(_ module.SimulationState) []simtypes.WeightedOperation {
	return nil
}

// IsAppModule implements the appmodule.AppModule interface.
func (am AppModule) IsAppModule() {}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (am AppModule) IsOnePerModuleType() {}

// setBaseDenom registers the display denom and base denom and sets the
// base denom for the chain. The function registered different values based on
// the EvmCoinInfo to allow different configurations in mainnet and testnet.
func setBaseDenom(ci types.EvmCoinInfo) (err error) {
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

func SetGlobalConfigVariables(coinInfo types.EvmCoinInfo) {
	// set the denom info for the chain
	if err := setBaseDenom(coinInfo); err != nil {
		panic(err)
	}

	configurator := types.NewEVMConfigurator()
	err := configurator.
		WithExtendedEips(types.DefaultCosmosEVMActivators).
		// NOTE: we're using the 18 decimals default for the example chain
		WithEVMCoinInfo(coinInfo).
		Configure()
	if err != nil {
		panic(err)
	}
}
