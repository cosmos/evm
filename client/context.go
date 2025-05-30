package client

import (
	"github.com/cosmos/cosmos-sdk/client" // import the original package
)

// EVMContext embeds the original Context and adds your own field
type EVMContext struct {
	client.Context        // Embedding the original Context
	EVMChainID     uint64 `json:"evm_chain_id"`
}

func (ctx EVMContext) WithEVMChainID(evmChainID uint64) EVMContext {
	return EVMContext{
		Context:    ctx.Context,
		EVMChainID: evmChainID,
	}
}
