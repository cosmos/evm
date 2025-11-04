package debug

import (
	"math/big"

	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/yihuang/go-abi"
)

type EVMKeeper interface {
	CallEVM(ctx sdk.Context, method abi.Method, from, contract common.Address, commit bool, gasCap *big.Int) (*evmtypes.MsgEthereumTxResponse, error)
	CallEVMWithData(
		ctx sdk.Context,
		from common.Address,
		contract *common.Address,
		data []byte,
		commit bool,
		gasCap *big.Int,
	) (*evmtypes.MsgEthereumTxResponse, error)
}
