package types

import (
	autocliv1 "cosmossdk.io/api/cosmos/autocli/v1"
)

// AutoCLIOptions implements the autocli.HasAutoCLIConfig interface.
func AutoCLIOptions() *autocliv1.ModuleOptions {
	return &autocliv1.ModuleOptions{
		Query: &autocliv1.ServiceCommandDescriptor{
			Service: _Query_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "ClientPrecompile",
					Use:       "get-precompile [client]",
					Short:     "Get the precompile information for a given client ID or light client precompile hex address",
					PositionalArgs: []*autocliv1.PositionalArgDescriptor{
						{ProtoField: "client"},
					},
				},
				{
					RpcMethod: "Params",
					Use:       "params",
					Short:     "Get the current module parameters",
				},
			},
		},
		Tx: &autocliv1.ServiceCommandDescriptor{
			Service: _Msg_serviceDesc.ServiceName,
			RpcCommandOptions: []*autocliv1.RpcCommandOptions{
				{
					RpcMethod: "RegisterClientPrecompile",
					// Use:       "register-precompile [client_id] [address] [sender]",
					// Short:     "Register a new light client precompile address for a given IBC client ID. Requires authority permissions.",
					// PositionalArgs: []*autocliv1.PositionalArgDescriptor{
					// 	{ProtoField: "client_id"},
					// 	{ProtoField: "address"},
					// 	{ProtoField: "sender"},
					// },
					Skip: true, // This is a authority gated tx, so we skip it.
				},
				{
					RpcMethod: "UpdateParams",
					Skip:      true, // This is a authority gated tx, so we skip it.
				},
			},
		},
	}
}
