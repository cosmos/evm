package interfaces

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type IMsgEthereumTx interface {
	FromEthereumTx(tx *ethtypes.Transaction)
	FromSignedEthereumTx(tx *ethtypes.Transaction, signer ethtypes.Signer) error
	GetFrom() sdk.AccAddress
	GetGas() uint64
	GetEffectiveFee(baseFee *big.Int) *big.Int
	AsTransaction() *ethtypes.Transaction
	GetSenderLegacy(signer ethtypes.Signer) (common.Address, error)
	AsMessage(baseFee *big.Int) *core.Message
	UnmarshalBinary(b []byte, signer ethtypes.Signer) error
}
