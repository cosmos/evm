package miner

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"go.opentelemetry.io/otel"

	"github.com/cosmos/evm/rpc/backend"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/server"
)

var tracer = otel.Tracer("evm/rpc/namespaces/ethereum/miner")

// API is the private miner prefixed set of APIs in the Miner JSON-RPC spec.
type API struct {
	ctx     *server.Context
	logger  log.Logger
	backend backend.EVMBackend
}

// NewPrivateAPI creates an instance of the Miner API.
func NewPrivateAPI(
	ctx *server.Context,
	backend backend.EVMBackend,
) *API {
	return &API{
		ctx:     ctx,
		logger:  ctx.Logger.With("api", "miner"),
		backend: backend,
	}
}

// SetEtherbase sets the etherbase of the miner
func (api *API) SetEtherbase(etherbase common.Address) bool {
	api.logger.Debug("miner_setEtherbase")
	ctx, span := tracer.Start(context.Background(), "miner_setEtherbase")
	defer span.End()
	return api.backend.SetEtherbase(ctx, etherbase)
}

// SetGasPrice sets the minimum accepted gas price for the miner.
func (api *API) SetGasPrice(gasPrice hexutil.Big) bool {
	api.logger.Info(api.ctx.Viper.ConfigFileUsed())
	ctx, span := tracer.Start(context.Background(), "miner_setGasPrice")
	defer span.End()
	return api.backend.SetGasPrice(ctx, gasPrice)
}
