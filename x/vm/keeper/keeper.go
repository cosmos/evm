package keeper

import (
	"encoding/binary"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/wrappers"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Keeper grants access to the EVM module state and implements the go-ethereum StateDB interface.
type Keeper struct {
	// Protobuf codec
	cdc codec.BinaryCodec
	// Store key required for the EVM Prefix KVStore. It is required by:
	// - storing account's Storage State
	// - storing account's Code
	// - storing transaction Logs
	// - storing Bloom filters by block height. Needed for the Web3 API.
	storeKey storetypes.StoreKey

	// key to access the object store, which is reset on every block during Commit
	objectKey storetypes.StoreKey

	// Store Keys for modules wired to app
	storeKeys map[string]storetypes.StoreKey

	// the address capable of executing a MsgUpdateParams message. Typically, this should be the x/gov module account.
	authority sdk.AccAddress

	// access to account state
	accountKeeper types.AccountKeeper

	// bankWrapper is used to convert the Cosmos SDK coin used in the EVM to the
	// proper decimal representation.
	bankWrapper types.BankWrapper

	// access historical headers for EVM state transition execution
	stakingKeeper types.StakingKeeper
	// fetch EIP1559 base fee and parameters
	feeMarketWrapper *wrappers.FeeMarketWrapper
	// optional erc20Keeper interface needed to instantiate erc20 precompiles
	erc20Keeper types.Erc20Keeper
	// consensusKeeper is used to get consensus params during query contexts.
	// This is needed as block.gasLimit is expected to be available in eth_call, which is routed through Cosmos SDK's
	// grpc query router. This query router builds a context WITHOUT consensus params, so we manually supply the context
	// with consensus params when not set in context.
	consensusKeeper types.ConsensusParamsKeeper

	// Tracer used to collect execution traces from the EVM transaction execution
	tracer string

	hooks types.EvmHooks
	// EVM Hooks for tx post-processing

	// precompiles defines the map of all available precompiled smart contracts.
	// Some of these precompiled contracts might not be active depending on the EVM
	// parameters.
	precompiles map[common.Address]vm.PrecompiledContract

	// evmMempool is the custom EVM appside mempool
	// if it is nil, the default comet mempool will be used
	evmMempool *evmmempool.ExperimentalEVMMempool

	// virtualFeeCollection enabling will use "Virtual" methods from the bank module to accumulate
	// fees to the fee collector module in the endBlocker instead of using regular sends during tx execution.
	virtualFeeCollection bool
}

// NewKeeper generates new evm module keeper
func NewKeeper(
	cdc codec.BinaryCodec,
	storeKey, objectKey storetypes.StoreKey,
	keys []storetypes.StoreKey,
	authority sdk.AccAddress,
	ak types.AccountKeeper,
	bankKeeper types.BankKeeper,
	sk types.StakingKeeper,
	fmk types.FeeMarketKeeper,
	consensusKeeper types.ConsensusParamsKeeper,
	erc20Keeper types.Erc20Keeper,
	evmChainID uint64,
	tracer string,
) *Keeper {
	// ensure evm module account is set
	if addr := ak.GetModuleAddress(types.ModuleName); addr == nil {
		panic("the EVM module account has not been set")
	}

	// ensure the authority account is correct
	if err := sdk.VerifyAddressFormat(authority); err != nil {
		panic(err)
	}

	bankWrapper := wrappers.NewBankWrapper(bankKeeper)
	feeMarketWrapper := wrappers.NewFeeMarketWrapper(fmk)
	storeKeys := make(map[string]storetypes.StoreKey)
	for _, k := range keys {
		storeKeys[k.Name()] = k
	}

	// set global chain config
	ethCfg := types.DefaultChainConfig(evmChainID)
	if err := types.SetChainConfig(ethCfg); err != nil {
		panic(err)
	}

	// NOTE: we pass in the parameter space to the CommitStateDB in order to use custom denominations for the EVM operations
	return &Keeper{
		cdc:              cdc,
		authority:        authority,
		accountKeeper:    ak,
		bankWrapper:      bankWrapper,
		stakingKeeper:    sk,
		feeMarketWrapper: feeMarketWrapper,
		storeKey:         storeKey,
		objectKey:        objectKey,
		tracer:           tracer,
		consensusKeeper:  consensusKeeper,
		erc20Keeper:      erc20Keeper,
		storeKeys:        storeKeys,
	}
}

// EnableVirtualFeeCollection switches fee deduction for evm transactions to use the virtual fee collection of the
// bank keeper via the object store.
// Note: Do NOT use this if your chain does not have an 18 decimal point precision gas token.
func (k *Keeper) EnableVirtualFeeCollection() {
	k.virtualFeeCollection = true
}

// Logger returns a module-specific logger.
func (k Keeper) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("module", types.ModuleName)
}

// ----------------------------------------------------------------------------
// Block Bloom
// Required by Web3 API.
// ----------------------------------------------------------------------------

// EmitBlockBloomEvent emit block bloom events
func (k Keeper) EmitBlockBloomEvent(ctx sdk.Context, bloom []byte) {
	ctx.EventManager().EmitEvent(
		sdk.NewEvent(
			types.EventTypeBlockBloom,
			sdk.NewAttribute(types.AttributeKeyEthereumBloom, string(bloom)),
		),
	)
}

// GetAuthority returns the x/evm module authority address
func (k Keeper) GetAuthority() sdk.AccAddress {
	return k.authority
}

// CollectTxBloom collects all tx blooms and emit a single block bloom event
func (k Keeper) CollectTxBloom(ctx sdk.Context) {
	store := prefix.NewObjStore(ctx.ObjectStore(k.objectKey), types.KeyPrefixObjectBloom)
	it := store.Iterator(nil, nil)
	defer it.Close()

	bloom := new(big.Int)
	for ; it.Valid(); it.Next() {
		bloom.Or(bloom, it.Value().(*big.Int))
		store.Delete(it.Key())
	}

	k.EmitBlockBloomEvent(ctx, bloom.Bytes())
}

// SetTxBloom sets the given bloom bytes to the object store. This value is reset on
// every block.
func (k Keeper) SetTxBloom(ctx sdk.Context, bloom *big.Int) {
	store := ctx.ObjectStore(k.objectKey)
	store.Set(types.ObjectBloomKey(ctx.TxIndex(), ctx.MsgIndex()), bloom)
}

// ----------------------------------------------------------------------------
// Hooks
// ----------------------------------------------------------------------------

// SetHooks sets the hooks for the EVM module
// Called only once during initialization, panics if called more than once.
func (k *Keeper) SetHooks(eh types.EvmHooks) *Keeper {
	if k.hooks != nil {
		panic("cannot set evm hooks twice")
	}

	k.hooks = eh
	return k
}

// PostTxProcessing delegates the call to the hooks.
// If no hook has been registered, this function returns with a `nil` error
func (k *Keeper) PostTxProcessing(
	ctx sdk.Context,
	sender common.Address,
	msg core.Message,
	receipt *ethtypes.Receipt,
) error {
	if k.hooks == nil {
		return nil
	}
	return k.hooks.PostTxProcessing(ctx, sender, msg, receipt)
}

// HasHooks returns true if hooks are set
func (k *Keeper) HasHooks() bool {
	return k.hooks != nil
}

// ----------------------------------------------------------------------------
// Storage
// ----------------------------------------------------------------------------

// GetAccountStorage return state storage associated with an account
func (k Keeper) GetAccountStorage(ctx sdk.Context, address common.Address) types.Storage {
	storage := types.Storage{}

	k.ForEachStorage(ctx, address, func(key, value common.Hash) bool {
		storage = append(storage, types.NewState(key, value))
		return true
	})

	return storage
}

// ----------------------------------------------------------------------------
// Account
// ----------------------------------------------------------------------------

// Tracer return a default vm.Tracer based on current keeper state
func (k Keeper) Tracer(ctx sdk.Context, msg core.Message, ethCfg *ethparams.ChainConfig) *tracing.Hooks {
	return types.NewTracer(k.tracer, msg, ethCfg, ctx.BlockHeight(), uint64(ctx.BlockTime().Unix())) //#nosec G115 -- int overflow is not a concern here
}

// GetAccountWithoutBalance load nonce and codehash without balance,
// more efficient in cases where balance is not needed.
func (k *Keeper) GetAccountWithoutBalance(ctx sdk.Context, addr common.Address) *statedb.Account {
	cosmosAddr := sdk.AccAddress(addr.Bytes())
	acct := k.accountKeeper.GetAccount(ctx, cosmosAddr)
	if acct == nil {
		return nil
	}

	codeHashBz := k.GetCodeHash(ctx, addr).Bytes()

	return &statedb.Account{
		Nonce:    acct.GetSequence(),
		CodeHash: codeHashBz,
	}
}

// GetAccountOrEmpty returns empty account if not exist.
func (k *Keeper) GetAccountOrEmpty(ctx sdk.Context, addr common.Address) statedb.Account {
	acct := k.GetAccount(ctx, addr)
	if acct != nil {
		return *acct
	}

	// empty account
	return statedb.Account{
		Balance:  new(uint256.Int),
		CodeHash: types.EmptyCodeHash,
	}
}

// GetNonce returns the sequence number of an account, returns 0 if not exists.
func (k *Keeper) GetNonce(ctx sdk.Context, addr common.Address) uint64 {
	cosmosAddr := sdk.AccAddress(addr.Bytes())
	acct := k.accountKeeper.GetAccount(ctx, cosmosAddr)
	if acct == nil {
		return 0
	}

	return acct.GetSequence()
}

// SpendableCoin load account's balance of gas token.
func (k *Keeper) SpendableCoin(ctx sdk.Context, addr common.Address) *uint256.Int {
	cosmosAddr := sdk.AccAddress(addr.Bytes())

	// Get the balance via bank wrapper to convert it to 18 decimals if needed.
	coin := k.bankWrapper.SpendableCoin(ctx, cosmosAddr, types.GetEVMCoinDenom())

	result, err := utils.Uint256FromBigInt(coin.Amount.BigInt())
	if err != nil {
		return nil
	}

	return result
}

// GetBalance load account's balance of gas token.
func (k *Keeper) GetBalance(ctx sdk.Context, addr common.Address) *uint256.Int {
	cosmosAddr := sdk.AccAddress(addr.Bytes())

	// Get the balance via bank wrapper to convert it to 18 decimals if needed.
	coin := k.bankWrapper.GetBalance(ctx, cosmosAddr, types.GetEVMCoinDenom())

	result, err := utils.Uint256FromBigInt(coin.Amount.BigInt())
	if err != nil {
		return nil
	}

	return result
}

// GetBaseFee returns current base fee, return values:
// - `nil`: london hardfork not enabled.
// - `0`: london hardfork enabled but feemarket is not enabled.
// - `n`: both london hardfork and feemarket are enabled.
func (k Keeper) GetBaseFee(ctx sdk.Context) *big.Int {
	ethCfg := types.GetEthChainConfig()
	if !types.IsLondon(ethCfg, ctx.BlockHeight()) {
		return nil
	}
	coinInfo := k.GetEvmCoinInfo(ctx)
	baseFee := k.feeMarketWrapper.GetBaseFee(ctx, types.Decimals(coinInfo.Decimals))
	if baseFee == nil {
		// return 0 if feemarket not enabled.
		baseFee = big.NewInt(0)
	}
	return baseFee
}

// GetMinGasPrice returns the MinGasPrice param from the fee market module
// adapted according to the evm denom decimals
func (k Keeper) GetMinGasPrice(ctx sdk.Context) math.LegacyDec {
	return k.feeMarketWrapper.GetParams(ctx).MinGasPrice
}

// ResetTransientGasUsed reset gas used to prepare for execution of cosmos tx, called in EndBlocker.
func (k Keeper) ResetTransientGasUsed(ctx sdk.Context) {
	store := prefix.NewObjStore(ctx.ObjectStore(k.objectKey),
		types.KeyPrefixObjectGasUsed)
	it := store.Iterator(nil, nil)

	defer it.Close()

	for ; it.Valid(); it.Next() {
		store.Delete(it.Key())
	}
}

// GetTransientGasUsed returns the gas used by current cosmos tx.
func (k Keeper) GetTransientGasUsed(ctx sdk.Context) uint64 {
	store := ctx.ObjectStore(k.objectKey)
	v := store.Get(types.ObjectGasUsedKey(ctx.TxIndex()))
	if v == nil {
		return 0
	}
	return v.(uint64)
}

// SetTransientGasUsed sets the gas used by current cosmos tx.
func (k Keeper) SetTransientGasUsed(ctx sdk.Context, gasUsed uint64) {
	store := ctx.ObjectStore(k.objectKey)
	store.Set(types.ObjectGasUsedKey(ctx.TxIndex()), gasUsed)
}

// AddTransientGasUsed accumulate gas used by each eth msgs included in current cosmos tx.
func (k Keeper) AddTransientGasUsed(ctx sdk.Context, gasUsed uint64) (uint64, error) {
	result := k.GetTransientGasUsed(ctx) + gasUsed
	if result < gasUsed {
		return 0, errorsmod.Wrap(types.ErrGasOverflow, "transient gas used")
	}
	k.SetTransientGasUsed(ctx, result)
	return result, nil
}

// KVStoreKeys returns KVStore keys injected to keeper
func (k Keeper) KVStoreKeys() map[string]storetypes.StoreKey {
	return k.storeKeys
}

// SetEvmMempool sets the evm mempool
func (k *Keeper) SetEvmMempool(evmMempool *evmmempool.ExperimentalEVMMempool) {
	k.evmMempool = evmMempool
}

// GetEvmMempool returns the evm mempool
func (k Keeper) GetEvmMempool() *evmmempool.ExperimentalEVMMempool {
	return k.evmMempool
}

// SetHeaderHash sets current block hash into EIP-2935 compatible storage contract.
func (k Keeper) SetHeaderHash(ctx sdk.Context) {
	window := uint64(types.DefaultHistoryServeWindow)
	params := k.GetParams(ctx)
	if params.HistoryServeWindow > 0 {
		window = params.HistoryServeWindow
	}

	acct := k.GetAccount(ctx, ethparams.HistoryStorageAddress)
	if acct != nil && k.IsContract(ctx, ethparams.HistoryStorageAddress) {
		// set current block hash in the contract storage, compatible with EIP-2935
		ringIndex := uint64(ctx.BlockHeight()) % window //nolint:gosec // G115 // won't exceed uint64
		var key common.Hash
		binary.BigEndian.PutUint64(key[24:], ringIndex)
		k.SetState(ctx, ethparams.HistoryStorageAddress, key, ctx.HeaderHash())
	}
}

// GetHeaderHash sets block hash into EIP-2935 compatible storage contract.
func (k Keeper) GetHeaderHash(ctx sdk.Context, height uint64) common.Hash {
	window := uint64(types.DefaultHistoryServeWindow)
	params := k.GetParams(ctx)
	if params.HistoryServeWindow > 0 {
		window = params.HistoryServeWindow
	}

	ringIndex := height % window
	var key common.Hash
	binary.BigEndian.PutUint64(key[24:], ringIndex)
	return k.GetState(ctx, ethparams.HistoryStorageAddress, key)
}
