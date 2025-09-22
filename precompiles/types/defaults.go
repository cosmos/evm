package types

import (
	"fmt"
	"maps"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	bankprecompile "github.com/cosmos/evm/precompiles/bank"
	bank2precompile "github.com/cosmos/evm/precompiles/bank2"
	"github.com/cosmos/evm/precompiles/bech32"
	cmn "github.com/cosmos/evm/precompiles/common"
	distprecompile "github.com/cosmos/evm/precompiles/distribution"
	govprecompile "github.com/cosmos/evm/precompiles/gov"
	ics20precompile "github.com/cosmos/evm/precompiles/ics20"
	"github.com/cosmos/evm/precompiles/p256"
	slashingprecompile "github.com/cosmos/evm/precompiles/slashing"
	stakingprecompile "github.com/cosmos/evm/precompiles/staking"
	erc20Keeper "github.com/cosmos/evm/x/erc20/keeper"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	channelkeeper "github.com/cosmos/ibc-go/v10/modules/core/04-channel/keeper"

	"cosmossdk.io/core/address"

	"github.com/cosmos/cosmos-sdk/codec"
	addresscodec "github.com/cosmos/cosmos-sdk/codec/address"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

// Optionals define some optional params that can be applied to _some_ precompiles.
// Extend this struct, add a sane default to defaultOptionals, and an Option function to provide users with a non-breaking
// way to provide custom args to certain precompiles.
type Optionals struct {
	AddressCodec       address.Codec // used by gov/staking
	ValidatorAddrCodec address.Codec // used by slashing
	ConsensusAddrCodec address.Codec // used by slashing
}

func defaultOptionals() Optionals {
	return Optionals{
		AddressCodec:       addresscodec.NewBech32Codec(sdktypes.GetConfig().GetBech32AccountAddrPrefix()),
		ValidatorAddrCodec: addresscodec.NewBech32Codec(sdktypes.GetConfig().GetBech32ValidatorAddrPrefix()),
		ConsensusAddrCodec: addresscodec.NewBech32Codec(sdktypes.GetConfig().GetBech32ConsensusAddrPrefix()),
	}
}

type Option func(opts *Optionals)

func WithAddressCodec(codec address.Codec) Option {
	return func(opts *Optionals) {
		opts.AddressCodec = codec
	}
}

func WithValidatorAddrCodec(codec address.Codec) Option {
	return func(opts *Optionals) {
		opts.ValidatorAddrCodec = codec
	}
}

func WithConsensusAddrCodec(codec address.Codec) Option {
	return func(opts *Optionals) {
		opts.ConsensusAddrCodec = codec
	}
}

const bech32PrecompileBaseGas = 6_000

// DefaultStaticPrecompiles returns the list of all available static precompiled contracts from Cosmos EVM.
//
// NOTE: this should only be used during initialization of the Keeper.
func DefaultStaticPrecompiles(
	stakingKeeper stakingkeeper.Keeper,
	distributionKeeper distributionkeeper.Keeper,
	bankKeeper2 bankkeeper.Keeper,
	bankKeeper cmn.BankKeeper,
	erc20Keeper *erc20Keeper.Keeper,
	transferKeeper *transferkeeper.Keeper,
	channelKeeper *channelkeeper.Keeper,
	govKeeper govkeeper.Keeper,
	slashingKeeper slashingkeeper.Keeper,
	codec codec.Codec,
	opts ...Option,
) map[common.Address]vm.PrecompiledContract {
	options := defaultOptionals()
	for _, opt := range opts {
		opt(&options)
	}
	// Clone the mapping from the latest EVM fork.
	precompiles := maps.Clone(vm.PrecompiledContractsPrague)

	// secp256r1 precompile as per EIP-7212
	p256Precompile := &p256.Precompile{}

	bech32Precompile, err := bech32.NewPrecompile(bech32PrecompileBaseGas)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate bech32 precompile: %w", err))
	}

	stakingPrecompile := stakingprecompile.NewPrecompile(
		stakingKeeper,
		stakingkeeper.NewMsgServerImpl(&stakingKeeper),
		stakingkeeper.NewQuerier(&stakingKeeper),
		bankKeeper,
		options.AddressCodec,
	)

	distributionPrecompile := distprecompile.NewPrecompile(
		distributionKeeper,
		distributionkeeper.NewMsgServerImpl(distributionKeeper),
		distributionkeeper.NewQuerier(distributionKeeper),
		stakingKeeper,
		bankKeeper,
		options.AddressCodec,
	)

	ibcTransferPrecompile := ics20precompile.NewPrecompile(
		bankKeeper,
		stakingKeeper,
		transferKeeper,
		channelKeeper,
	)

	bankPrecompile := bankprecompile.NewPrecompile(bankKeeper, erc20Keeper)

	govPrecompile := govprecompile.NewPrecompile(
		govkeeper.NewMsgServerImpl(&govKeeper),
		govkeeper.NewQueryServer(&govKeeper),
		bankKeeper,
		codec,
		options.AddressCodec,
	)

	slashingPrecompile := slashingprecompile.NewPrecompile(
		slashingKeeper,
		slashingkeeper.NewMsgServerImpl(slashingKeeper),
		bankKeeper,
		options.ValidatorAddrCodec,
		options.ConsensusAddrCodec,
	)

	bank2Precompile := bank2precompile.NewPrecompile(bankkeeper.NewMsgServerImpl(bankKeeper2), bankKeeper2)

	// Stateless precompiles
	precompiles[bech32Precompile.Address()] = bech32Precompile
	precompiles[p256Precompile.Address()] = p256Precompile

	// Stateful precompiles
	precompiles[stakingPrecompile.Address()] = stakingPrecompile
	precompiles[distributionPrecompile.Address()] = distributionPrecompile
	precompiles[ibcTransferPrecompile.Address()] = ibcTransferPrecompile
	precompiles[bankPrecompile.Address()] = bankPrecompile
	precompiles[govPrecompile.Address()] = govPrecompile
	precompiles[slashingPrecompile.Address()] = slashingPrecompile
	precompiles[bank2Precompile.Address()] = bank2Precompile

	return precompiles
}
