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
	evmtrace "github.com/cosmos/evm/trace"

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
func (api *PublicAPI) Content() (_ map[string]map[string]map[string]*types.RPCTransaction, err error) {
	api.logger.Debug("txpool_content")
	ctx, span := tracer.Start(context.Background(), "Content")
	defer func() { evmtrace.EndSpanErr(span, err) }()
	return api.backend.Content(ctx)
}

// ContentFrom returns the transactions contained within the transaction pool
func (api *PublicAPI) ContentFrom(address common.Address) (_ map[string]map[string]*types.RPCTransaction, err error) {
	api.logger.Debug("txpool_contentFrom")
	ctx, span := tracer.Start(context.Background(), "ContentFrom", trace.WithAttributes(attribute.String("address", address.Hex())))
	defer func() { evmtrace.EndSpanErr(span, err) }()
	return api.backend.ContentFrom(ctx, address)
}

// Inspect returns the content of the transaction pool and flattens it into an easily inspectable list
func (api *PublicAPI) Inspect() (_ map[string]map[string]map[string]string, err error) {
	api.logger.Debug("txpool_inspect")
	ctx, span := tracer.Start(context.Background(), "Inspect")
	defer func() { evmtrace.EndSpanErr(span, err) }()
	return api.backend.Inspect(ctx)
}

// Status returns the number of pending and queued transaction in the pool
func (api *PublicAPI) Status() (_ map[string]hexutil.Uint, err error) {
	api.logger.Debug("txpool_status")
	ctx, span := tracer.Start(context.Background(), "Status")
	defer func() { evmtrace.EndSpanErr(span, err) }()
	return api.backend.Status(ctx)
}
