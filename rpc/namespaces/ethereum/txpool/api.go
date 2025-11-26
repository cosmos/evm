package txpool

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/cosmos/evm/rpc/backend"
	"github.com/cosmos/evm/rpc/types"

	"cosmossdk.io/log"
)

var tracer = otel.Tracer("evm/rpc/namespaces/ethereum/txpool")

// PublicAPI offers and API for the transaction pool. It only operates on data that is non-confidential.
// NOTE: For more info about the current status of this endpoints see https://github.com/evmos/ethermint/issues/124
type PublicAPI struct {
	logger  log.Logger
	backend backend.EVMBackend
}

// NewPublicAPI creates a new tx pool service that gives information about the transaction pool.
func NewPublicAPI(logger log.Logger, backend backend.EVMBackend) *PublicAPI {
	return &PublicAPI{
		logger:  logger.With("module", "txpool"),
		backend: backend,
	}
}

// Content returns the transactions contained within the transaction pool
func (api *PublicAPI) Content() (map[string]map[string]map[string]*types.RPCTransaction, error) {
	api.logger.Debug("txpool_content")
	ctx, span := tracer.Start(context.Background(), "Content")
	defer span.End()
	return api.backend.Content(ctx)
}

// ContentFrom returns the transactions contained within the transaction pool
func (api *PublicAPI) ContentFrom(address common.Address) (map[string]map[string]*types.RPCTransaction, error) {
	api.logger.Debug("txpool_contentFrom")
	ctx, span := tracer.Start(context.Background(), "ContentFrom", trace.WithAttributes(attribute.String("address", address.Hex())))
	defer span.End()
	return api.backend.ContentFrom(ctx, address)
}

// Inspect returns the content of the transaction pool and flattens it into an easily inspectable list
func (api *PublicAPI) Inspect() (map[string]map[string]map[string]string, error) {
	api.logger.Debug("txpool_inspect")
	ctx, span := tracer.Start(context.Background(), "Inspect")
	defer span.End()
	return api.backend.Inspect(ctx)
}

// Status returns the number of pending and queued transaction in the pool
func (api *PublicAPI) Status() (map[string]hexutil.Uint, error) {
	api.logger.Debug("txpool_status")
	ctx, span := tracer.Start(context.Background(), "Status")
	defer span.End()
	return api.backend.Status(ctx)
}
