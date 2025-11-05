package evmd

import (
	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

type BankKeeper interface {
	evmtypes.BankKeeper
	cmn.BankKeeper
}
