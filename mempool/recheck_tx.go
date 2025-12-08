package mempool

import (
	"fmt"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/vm/statedb"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

type TxConverter interface {
	EVMTxToCosmosTx(tx *ethtypes.Transaction) (sdk.Tx, error)
}

func NewRecheckTx(converter TxConverter, anteHandler sdk.AnteHandler) legacypool.RecheckFn {
	return func(ctx sdk.Context, tx *ethtypes.Transaction) (sdk.Context, error) {
		cosmosTx, err := converter.EVMTxToCosmosTx(tx)
		if err != nil {
			return sdk.Context{}, fmt.Errorf("converting evm tx %s to cosmos tx: %w", tx.Hash(), err)
		}

		return anteHandler(ctx, cosmosTx, false)
	}
}

func RecheckContext(db vm.StateDB, header *ethtypes.Header) sdk.Context {
	statedb, ok := db.(*statedb.StateDB)
	if !ok {
		panic("expected *statedb.StateDB for implementation of vm.StateDB")
	}

	// NOTE: The statedb's context is cached at height H, and will not be
	// updated to height H+1 out from under us, so we do not need to lock when
	// reading or updating. We branch this context again in order to get a
	// context at height H that will be isolated for recheck.
	//
	// We will never write the recheck changes back to the vm's original
	// context, so we disregard the given write function.
	ctx, _ := statedb.GetContext().CacheContext()
	if ctx.ConsensusParams().Block == nil {
		// set the latest blocks gas limit as the max gas in cp. this is
		// necessary to validate each tx's gas wanted
		maxGas, err := utils.SafeInt64(header.GasLimit)
		if err != nil {
			panic(fmt.Errorf("converting evm block gas limit to int64: %w", err))
		}
		cp := cmtproto.ConsensusParams{Block: &cmtproto.BlockParams{MaxGas: maxGas}}
		ctx = ctx.WithConsensusParams(cp)
	}

	return ctx
}
