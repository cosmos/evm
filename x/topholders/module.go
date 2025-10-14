package topholders

import (
	"context"
	"encoding/json"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/spf13/cobra"

	"cosmossdk.io/core/appmodule"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"

	"github.com/cosmos/evm/x/topholders/keeper"
	"github.com/cosmos/evm/x/topholders/types"
)

// ConsensusVersion defines the current module consensus version.
const ConsensusVersion = 1

var (
	_ module.AppModule      = AppModule{}
	_ module.AppModuleBasic = AppModuleBasic{}
	_ module.HasABCIGenesis = AppModule{}

	_ appmodule.AppModule   = AppModule{}
	_ module.HasABCIGenesis = AppModule{}
)

// ----------------------------------------------------------------------------
// AppModuleBasic
// ----------------------------------------------------------------------------

// AppModuleBasic app module basics object
type AppModuleBasic struct{}

func NewAppModuleBasic() AppModuleBasic {
	return AppModuleBasic{}
}

// Name get module name
func (AppModuleBasic) Name() string {
	return types.ModuleName
}

// RegisterLegacyAminoCodec registers the module's types with the given codec.
func (AppModuleBasic) RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {}

// RegisterInterfaces registers the module's interface types
func (a AppModuleBasic) RegisterInterfaces(reg cdctypes.InterfaceRegistry) {}

// DefaultGenesis returns default genesis state as raw bytes for the topholders
// module.
func (AppModuleBasic) DefaultGenesis(cdc codec.JSONCodec) json.RawMessage {
	gs := DefaultGenesis()
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(err)
	}
	return bz
}

// ValidateGenesis module validate genesis
func (AppModuleBasic) ValidateGenesis(cdc codec.JSONCodec, config client.TxEncodingConfig, bz json.RawMessage) error {
	var gs GenesisState
	err := json.Unmarshal(bz, &gs)
	if err != nil {
		return err
	}
	return gs.Validate()
}

// RegisterGRPCGatewayRoutes registers the gRPC Gateway routes for topholders module.
func (AppModuleBasic) RegisterGRPCGatewayRoutes(clientCtx client.Context, mux *runtime.ServeMux) {
	if err := types.RegisterQueryHandlerClient(context.Background(), mux, types.NewQueryClient(clientCtx)); err != nil {
		panic(err)
	}
}

// GetTxCmd returns topholders module's root tx command.
func (a AppModuleBasic) GetTxCmd() *cobra.Command {
	return nil
}

// GetQueryCmd returns topholders module's root query command.
func (AppModuleBasic) GetQueryCmd() *cobra.Command {
	return nil
}

// ----------------------------------------------------------------------------
// AppModule
// ----------------------------------------------------------------------------

// AppModule implements the AppModule interface for topholders module.
type AppModule struct {
	AppModuleBasic

	keeper keeper.Keeper
}

// NewAppModule creates a new AppModule object
func NewAppModule(
	keeper keeper.Keeper,
) AppModule {
	return AppModule{
		AppModuleBasic: NewAppModuleBasic(),
		keeper:         keeper,
	}
}

// Name returns topholders module's name.
func (am AppModule) Name() string {
	return am.AppModuleBasic.Name()
}

// RegisterServices registers a GRPC query service to respond to the
// module-specific GRPC queries.
func (am AppModule) RegisterServices(cfg module.Configurator) {
	types.RegisterQueryServer(cfg.QueryServer(), &am.keeper)
}

// InitGenesis performs topholders module's genesis initialization.
func (am AppModule) InitGenesis(ctx sdk.Context, cdc codec.JSONCodec, gs json.RawMessage) []abci.ValidatorUpdate {
	var genState GenesisState
	if err := json.Unmarshal(gs, &genState); err != nil {
		panic(err)
	}

	InitGenesis(ctx, am.keeper, &genState)
	return []abci.ValidatorUpdate{}
}

// ExportGenesis returns topholders module's exported genesis state as raw JSON bytes.
func (am AppModule) ExportGenesis(ctx sdk.Context, cdc codec.JSONCodec) json.RawMessage {
	gs := ExportGenesis(ctx, am.keeper)
	bz, err := json.Marshal(gs)
	if err != nil {
		panic(err)
	}
	return bz
}

// BeginBlock executes all ABCI BeginBlock logic respective to the topholders module.
func (am AppModule) BeginBlock(ctx context.Context) error {
	return am.keeper.BeginBlocker(ctx)
}

// IsAppModule implements the appmodule.AppModule interface.
func (AppModule) IsAppModule() {}

// IsOnePerModuleType implements the depinject.OnePerModuleType interface.
func (AppModule) IsOnePerModuleType() {}

// ConsensusVersion implements AppModule/ConsensusVersion.
func (AppModule) ConsensusVersion() uint64 { return ConsensusVersion }
