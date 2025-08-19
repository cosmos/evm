package namespaces

import "github.com/cosmos/evm/tests/jsonrpc/simulator/types"

const (
	// Engine API namespace (not applicable for Cosmos chains)
	MethodNameEngineNewPayloadV1        types.RpcName = "engine_newPayloadV1"
	MethodNameEngineNewPayloadV2        types.RpcName = "engine_newPayloadV2"
	MethodNameEngineNewPayloadV3        types.RpcName = "engine_newPayloadV3"
	MethodNameEngineForkchoiceUpdatedV1 types.RpcName = "engine_forkchoiceUpdatedV1"
	MethodNameEngineForkchoiceUpdatedV2 types.RpcName = "engine_forkchoiceUpdatedV2"
	MethodNameEngineForkchoiceUpdatedV3 types.RpcName = "engine_forkchoiceUpdatedV3"
	MethodNameEngineGetPayloadV1        types.RpcName = "engine_getPayloadV1"
	MethodNameEngineGetPayloadV2        types.RpcName = "engine_getPayloadV2"
	MethodNameEngineGetPayloadV3        types.RpcName = "engine_getPayloadV3"
)
