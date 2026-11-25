package types

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// RPCMsgEthereumTxI is a minimal read-only interface used by the RPC/query layer.
type RPCMsgEthereumTxI interface {
	GetGas() uint64
	AsTransaction() *ethtypes.Transaction
	GetSenderLegacy(signer ethtypes.Signer) (common.Address, error)
}
