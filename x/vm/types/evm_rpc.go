package types

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RPCMsgEthereumTxI is a minimal read-only interface used by the RPC/query layer.
type RPCMsgEthereumTxI interface {
	GetFrom() sdk.AccAddress
	GetGas() uint64
	AsTransaction() *ethtypes.Transaction
	GetSenderLegacy(signer ethtypes.Signer) (common.Address, error)
}
