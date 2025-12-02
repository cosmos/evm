package rpc

import (
	"fmt"

	"github.com/ethereum/go-ethereum/rpc"

	"github.com/cosmos/evm/rpc/backend"
	"github.com/cosmos/evm/rpc/namespaces/ethereum/debug"
	"github.com/cosmos/evm/rpc/namespaces/ethereum/eth"
	"github.com/cosmos/evm/rpc/namespaces/ethereum/eth/filters"
	"github.com/cosmos/evm/rpc/namespaces/ethereum/miner"
	"github.com/cosmos/evm/rpc/namespaces/ethereum/net"
	"github.com/cosmos/evm/rpc/namespaces/ethereum/personal"
	"github.com/cosmos/evm/rpc/namespaces/ethereum/txpool"
	"github.com/cosmos/evm/rpc/namespaces/ethereum/web3"
	"github.com/cosmos/evm/rpc/stream"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server"
)

// RPC namespaces and API version
const (
	// Cosmos namespaces

	CosmosNamespace = "cosmos"

	// Ethereum namespaces

	Web3Namespace     = "web3"
	EthNamespace      = "eth"
	PersonalNamespace = "personal"
	NetNamespace      = "net"
	TxPoolNamespace   = "txpool"
	DebugNamespace    = "debug"
	MinerNamespace    = "miner"

	apiVersion = "1.0"
)

// APICreator creates the JSON-RPC API implementations.
type APICreator = func(
	ctx *server.Context,
	clientCtx client.Context,
	stream *stream.RPCStream,
	backendProvider func() backend.BackendI,
) []rpc.API

// apiCreators defines the JSON-RPC API namespaces.
var apiCreators map[string]APICreator

func init() {
	apiCreators = map[string]APICreator{
		EthNamespace: func(
			ctx *server.Context,
			clientCtx client.Context,
			stream *stream.RPCStream,
			backendProvider func() backend.BackendI,
		) []rpc.API {
			evmBackend := backendProvider()

			// should not happen, but just in case
			filterBackend, ok := evmBackend.(filters.Backend)
			if !ok {
				panic("backend does not implement filter.Backend")
			}

			return []rpc.API{
				{
					Namespace: EthNamespace,
					Version:   apiVersion,
					Service:   eth.NewPublicAPI(ctx.Logger, evmBackend),
					Public:    true,
				},
				{
					Namespace: EthNamespace,
					Version:   apiVersion,
					Service:   filters.NewPublicAPI(ctx.Logger, clientCtx, stream, filterBackend),
					Public:    true,
				},
			}
		},
		Web3Namespace: func(*server.Context, client.Context, *stream.RPCStream, func() backend.BackendI) []rpc.API {
			return []rpc.API{
				{
					Namespace: Web3Namespace,
					Version:   apiVersion,
					Service:   web3.NewPublicAPI(),
					Public:    true,
				},
			}
		},
		NetNamespace: func(
			ctx *server.Context,
			clientCtx client.Context,
			_ *stream.RPCStream,
			_ func() backend.BackendI,
		) []rpc.API {
			return []rpc.API{
				{
					Namespace: NetNamespace,
					Version:   apiVersion,
					Service:   net.NewPublicAPI(ctx, clientCtx),
					Public:    true,
				},
			}
		},
		PersonalNamespace: func(
			ctx *server.Context,
			_ client.Context,
			_ *stream.RPCStream,
			backendProvider func() backend.BackendI,
		) []rpc.API {
			return []rpc.API{
				{
					Namespace: PersonalNamespace,
					Version:   apiVersion,
					Service:   personal.NewAPI(ctx.Logger, backendProvider()),
					Public:    false,
				},
			}
		},
		TxPoolNamespace: func(
			ctx *server.Context,
			_ client.Context,
			_ *stream.RPCStream,
			backendProvider func() backend.BackendI,
		) []rpc.API {
			return []rpc.API{
				{
					Namespace: TxPoolNamespace,
					Version:   apiVersion,
					Service:   txpool.NewPublicAPI(ctx.Logger, backendProvider()),
					Public:    true,
				},
			}
		},
		DebugNamespace: func(
			ctx *server.Context,
			_ client.Context,
			_ *stream.RPCStream,
			backendProvider func() backend.BackendI,
		) []rpc.API {
			evmBackend := backendProvider()
			return []rpc.API{
				{
					Namespace: DebugNamespace,
					Version:   apiVersion,
					Service:   debug.NewAPI(ctx, evmBackend, evmBackend.GetConfig().JSONRPC.EnableProfiling),
					Public:    true,
				},
			}
		},
		MinerNamespace: func(
			ctx *server.Context,
			_ client.Context,
			_ *stream.RPCStream,
			backendProvider func() backend.BackendI,
		) []rpc.API {
			return []rpc.API{
				{
					Namespace: MinerNamespace,
					Version:   apiVersion,
					Service:   miner.NewPrivateAPI(ctx, backendProvider()),
					Public:    false,
				},
			}
		},
	}
}

// BuildRPCs builds the JSON-RPC APIs for the given namespaces.
func BuildRPCs(
	selectedAPIs []string,
	ctx *server.Context,
	clientCtx client.Context,
	stream *stream.RPCStream,
	backendProvider func() backend.BackendI,
) []rpc.API {
	var apis []rpc.API

	for _, ns := range selectedAPIs {
		creator, ok := apiCreators[ns]
		if !ok {
			ctx.Logger.Error("invalid namespace value", "namespace", ns)
			continue
		}

		api := creator(ctx, clientCtx, stream, backendProvider)

		apis = append(apis, api...)
	}

	return apis
}

// RegisterAPINamespace registers a new API namespace with the API creator.
// This function fails if the namespace is already registered.
func RegisterAPINamespace(ns string, creator APICreator) error {
	if _, ok := apiCreators[ns]; ok {
		return fmt.Errorf("duplicated api namespace %s", ns)
	}
	apiCreators[ns] = creator
	return nil
}
