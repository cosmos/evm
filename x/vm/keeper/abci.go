package keeper

import (
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	ethparams "github.com/ethereum/go-ethereum/params"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// BeginBlock emits a base fee event which will be adjusted to the evm decimals
func (k *Keeper) BeginBlock(ctx sdk.Context) error {
	logger := ctx.Logger().With("begin_block", "evm")

	// Base fee is already set on FeeMarket BeginBlock
	// that runs before this one
	// We emit this event on the EVM and FeeMarket modules
	// because they can be different if the evm denom has 6 decimals
	res, err := k.BaseFee(ctx, &evmtypes.QueryBaseFeeRequest{})
	if err != nil {
		logger.Error("error when getting base fee", "error", err.Error())
	}
	if res != nil && res.BaseFee != nil && !res.BaseFee.IsNil() {
		// Store current base fee in event
		ctx.EventManager().EmitEvents(sdk.Events{
			sdk.NewEvent(
				evmtypes.EventTypeFeeMarket,
				sdk.NewAttribute(evmtypes.AttributeKeyBaseFee, res.BaseFee.String()),
			),
		})
	}

	ethCfg := evmtypes.GetEthChainConfig()
	if ethCfg.IsPrague(big.NewInt(ctx.BlockHeight()), uint64(ctx.BlockTime().Unix())) {
		stateDB := statedb.New(ctx, k, statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash())))
		// can't get current coinbase address here
		cfg := &statedb.EVMConfig{
			Params:  k.GetParams(ctx),
			BaseFee: res.BaseFee.BigInt(),
		}
		vmConfig := k.VMConfig(ctx, cfg, nil)
		blockCtx := k.BlockContext(ctx, cfg, stateDB)
		evm := vm.NewEVM(blockCtx, stateDB, ethCfg, vmConfig)
		parentHash := ctx.BlockHeader().LastBlockId.Hash
		if err := ProcessParentBlockHash(common.BytesToHash(parentHash), evm); err != nil {
			// must not happen
			return err
		}
	}

	return nil
}

// EndBlock also retrieves the bloom filter value from the transient store and commits it to the
// KVStore. The EVM end block logic doesn't update the validator set, thus it returns
// an empty slice.
func (k *Keeper) EndBlock(ctx sdk.Context) error {
	// Gas costs are handled within msg handler so costs should be ignored
	infCtx := ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())

	bloom := ethtypes.BytesToBloom(k.GetBlockBloomTransient(infCtx).Bytes())
	k.EmitBlockBloomEvent(infCtx, bloom)

	return nil
}

// ProcessParentBlockHash stores the parent block hash in the history storage contract
// as per EIP-2935/7709.
func ProcessParentBlockHash(prevHash common.Hash, evm *vm.EVM) error {
	msg := &core.Message{
		From:      ethparams.SystemAddress,
		GasLimit:  30_000_000,
		GasPrice:  common.Big0,
		GasFeeCap: common.Big0,
		GasTipCap: common.Big0,
		To:        &ethparams.HistoryStorageAddress,
		Data:      prevHash.Bytes(),
	}
	evm.SetTxContext(core.NewEVMTxContext(msg))
	evm.StateDB.AddAddressToAccessList(ethparams.HistoryStorageAddress)
	_, _, err := evm.Call(msg.From, *msg.To, msg.Data, 30_000_000, common.U2560)
	if err != nil {
		return err
	}
	if evm.StateDB.AccessEvents() != nil {
		evm.StateDB.AccessEvents().Merge(evm.AccessEvents)
	}
	evm.StateDB.Finalise(true)
	return nil
}
