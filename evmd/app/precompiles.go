package app

import (
	"fmt"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	distributionprecompile "github.com/cosmos/evm/precompiles/distribution"
	"maps"

	evmibcutils "github.com/cosmos/evm/ibc"
	"github.com/cosmos/evm/precompiles/bech32"
	cmn "github.com/cosmos/evm/precompiles/common"
	govprecompile "github.com/cosmos/evm/precompiles/gov"
	ics02precompile "github.com/cosmos/evm/precompiles/ics02"
	"github.com/cosmos/evm/precompiles/p256"
	stakingprecompile "github.com/cosmos/evm/precompiles/staking"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

// NOTE: This is almost a full copy of the precompiles.go file in the evm repo, but we
// needed to remove some of the precompiles...
const (
	// defaultBech32BaseGas is the default gas cost for bech32 precompile operations.
	// This value provides sufficient gas for address conversion operations while
	// preventing excessive gas consumption attacks.
	defaultBech32BaseGas = 6_000
)

// StaticPrecompiles returns the list of all available static precompiled contracts from Cosmos EVM.
//
// NOTE: this should only be used during initialization of the Keeper.
func StaticPrecompiles(
	stakingKeeper stakingkeeper.Keeper,
	distributionKeeper distrkeeper.Keeper,
	bankKeeper cmn.BankKeeper,
	govKeeper govkeeper.Keeper,
	clientKeeper evmibcutils.ClientKeeper,
	cdc codec.Codec,
) map[common.Address]vm.PrecompiledContract {
	// Clone the mapping from the latest EVM fork.
	precompiles := maps.Clone(vm.PrecompiledContractsPrague)

	addrCodec := addresscodec.NewBech32Codec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	// Stateless precompiles
	bech32Precompile, err := bech32.NewPrecompile(defaultBech32BaseGas)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate bech32 precompile: %w", err))
	}
	precompiles[bech32Precompile.Address()] = bech32Precompile

	// secp256r1 precompile as per EIP-7212
	p256Precompile := &p256.Precompile{}
	precompiles[p256Precompile.Address()] = p256Precompile

	// Stateful precompiles
	stakingPrecompile := stakingprecompile.NewPrecompile(
		stakingKeeper,
		stakingkeeper.NewMsgServerImpl(&stakingKeeper),
		stakingkeeper.NewQuerier(&stakingKeeper),
		bankKeeper,
		addrCodec,
	)
	precompiles[stakingPrecompile.Address()] = stakingPrecompile

	distributionPrecompile := distributionprecompile.NewPrecompile(
		distributionKeeper,
		distrkeeper.NewMsgServerImpl(distributionKeeper),
		distrkeeper.NewQuerier(distributionKeeper),
		stakingKeeper,
		bankKeeper,
		addrCodec,
	)
	precompiles[distributionPrecompile.Address()] = distributionPrecompile

	govPrecompile := govprecompile.NewPrecompile(
		govkeeper.NewMsgServerImpl(&govKeeper),
		govkeeper.NewQueryServer(&govKeeper),
		bankKeeper,
		cdc,
		addrCodec,
	)
	precompiles[govPrecompile.Address()] = govPrecompile

	ics02Precompile := ics02precompile.NewPrecompile(cdc, clientKeeper)
	precompiles[ics02Precompile.Address()] = ics02Precompile

	return precompiles
}
