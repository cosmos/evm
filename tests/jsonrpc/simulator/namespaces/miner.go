package namespaces

import "github.com/cosmos/evm/tests/jsonrpc/simulator/types"

const (
	// Miner namespace (deprecated)
	MethodNameMinerStart        types.RpcName = "miner_start"
	MethodNameMinerStop         types.RpcName = "miner_stop"
	MethodNameMinerSetEtherbase types.RpcName = "miner_setEtherbase"
	MethodNameMinerSetExtra     types.RpcName = "miner_setExtra"
	MethodNameMinerSetGasPrice  types.RpcName = "miner_setGasPrice"
	MethodNameMinerSetGasLimit  types.RpcName = "miner_setGasLimit"
	MethodNameMinerGetHashrate  types.RpcName = "miner_getHashrate"
)
