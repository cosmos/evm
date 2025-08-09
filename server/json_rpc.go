package server

import (
	"context"
	"log/slog"
	"net/http"

	ethrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"golang.org/x/sync/errgroup"

	"github.com/cosmos/evm/rpc"
	serverconfig "github.com/cosmos/evm/server/config"
	cosmosevmtypes "github.com/cosmos/evm/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server"
)

// StartJSONRPC starts the JSON-RPC server
func StartJSONRPC(
	ctx context.Context,
	srvCtx *server.Context,
	clientCtx client.Context,
	g *errgroup.Group,
	tmRPCAddr, tmEndpoint string,
	config *serverconfig.Config,
	indexer cosmosevmtypes.EVMTxIndexer,
) (*http.Server, error) {
	logger := srvCtx.Logger.With("module", "geth")
	tmWsClient := ConnectTmWS(tmRPCAddr, tmEndpoint, logger)

	// Set Geth's global logger to use this handler
	handler := &CustomSlogHandler{logger: logger}
	slog.SetDefault(slog.New(handler))

	rpcServer := ethrpc.NewServer()

	rpcServer.SetBatchLimits(config.JSONRPC.BatchRequestLimit, config.JSONRPC.BatchResponseMaxSize)
	allowUnprotectedTxs := config.JSONRPC.AllowUnprotectedTxs
	rpcAPIArr := config.JSONRPC.API

	apis := rpc.GetRPCAPIs(srvCtx, clientCtx, tmWsClient, allowUnprotectedTxs, indexer, rpcAPIArr)

	for _, api := range apis {
		if err := rpcServer.RegisterName(api.Namespace, api.Service); err != nil {
			logger.Error(
				"failed to register service in JSON RPC namespace",
				"namespace", api.Namespace,
				"service", api.Service,
			)
			return nil, err
		}
	}

	r := mux.NewRouter()
	r.HandleFunc("/", rpcServer.ServeHTTP).Methods("POST")

	handlerWithCors := cors.Default()
	if config.API.EnableUnsafeCORS {
		handlerWithCors = cors.AllowAll()
	}

	httpSrv := &http.Server{
		Addr:              config.JSONRPC.Address,
		Handler:           handlerWithCors.Handler(r),
		ReadHeaderTimeout: config.JSONRPC.HTTPTimeout,
		ReadTimeout:       config.JSONRPC.HTTPTimeout,
		WriteTimeout:      config.JSONRPC.HTTPTimeout,
		IdleTimeout:       config.JSONRPC.HTTPIdleTimeout,
	}
	httpSrvDone := make(chan struct{}, 1)

	ln, err := Listen(httpSrv.Addr, config)
	if err != nil {
		return nil, err
	}

	g.Go(func() error {
		srvCtx.Logger.Info("Starting JSON-RPC server", "address", config.JSONRPC.Address)
		errCh := make(chan error)
		go func() {
			errCh <- httpSrv.Serve(ln)
		}()

		// Start a blocking select to wait for an indication to stop the server or that
		// the server failed to start properly.
		select {
		case <-ctx.Done():
			// The calling process canceled or closed the provided context, so we must
			// gracefully stop the JSON-RPC server.
			logger.Info("stopping JSON-RPC server...", "address", config.JSONRPC.Address)
			if err := httpSrv.Shutdown(context.Background()); err != nil {
				logger.Error("failed to shutdown JSON-RPC server", "error", err.Error())
			}
			return nil

		case err := <-errCh:
			if err == http.ErrServerClosed {
				close(httpSrvDone)
				return nil
			}

			srvCtx.Logger.Error("failed to start JSON-RPC server", "error", err.Error())
			return err
		}
	})

	srvCtx.Logger.Info("Starting JSON WebSocket server", "address", config.JSONRPC.WsAddress)

	// allocate separate WS connection to Tendermint
	tmWsClient = ConnectTmWS(tmRPCAddr, tmEndpoint, logger)
	wsSrv := rpc.NewWebsocketsServer(clientCtx, logger, tmWsClient, config)
	wsSrv.Start()
	return httpSrv, nil
}
