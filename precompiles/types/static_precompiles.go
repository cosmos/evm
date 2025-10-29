package types

import (
	"fmt"
	"maps"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	ibcutils "github.com/cosmos/evm/ibc"
	bankprecompile "github.com/cosmos/evm/precompiles/bank"
	"github.com/cosmos/evm/precompiles/bech32"
	cmn "github.com/cosmos/evm/precompiles/common"
	distprecompile "github.com/cosmos/evm/precompiles/distribution"
	govprecompile "github.com/cosmos/evm/precompiles/gov"
	ics02precompile "github.com/cosmos/evm/precompiles/ics02"
	ics20precompile "github.com/cosmos/evm/precompiles/ics20"
	"github.com/cosmos/evm/precompiles/p256"
	slashingprecompile "github.com/cosmos/evm/precompiles/slashing"
	stakingprecompile "github.com/cosmos/evm/precompiles/staking"
	erc20Keeper "github.com/cosmos/evm/x/erc20/keeper"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	channelkeeper "github.com/cosmos/ibc-go/v10/modules/core/04-channel/keeper"

	"github.com/cosmos/cosmos-sdk/codec"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

type StaticPrecompiles map[common.Address]vm.PrecompiledContract

func NewStaticPrecompiles() StaticPrecompiles {
	return make(StaticPrecompiles)
}

func (s StaticPrecompiles) WithPraguePrecompiles() StaticPrecompiles {
	maps.Copy(s, vm.PrecompiledContractsPrague)
	return s
}

func (s StaticPrecompiles) WithP256Precompile() StaticPrecompiles {
	p256Precompile := &p256.Precompile{}
	s[p256Precompile.Address()] = p256Precompile
	return s
}

func (s StaticPrecompiles) WithBech32Precompile() StaticPrecompiles {
	bech32Precompile, err := bech32.NewPrecompile(bech32PrecompileBaseGas)
	if err != nil {
		panic(fmt.Errorf("failed to instantiate bech32 precompile: %w", err))
	}
	s[bech32Precompile.Address()] = bech32Precompile
	return s
}

func (s StaticPrecompiles) WithStakingPrecompile(
	stakingKeeper stakingkeeper.Keeper,
	bankKeeper cmn.BankKeeper,
	opts ...Option,
) StaticPrecompiles {
	options := defaultOptionals()
	for _, opt := range opts {
		opt(&options)
	}

	stakingPrecompile := stakingprecompile.NewPrecompile(
		stakingKeeper,
		stakingkeeper.NewMsgServerImpl(&stakingKeeper),
		stakingkeeper.NewQuerier(&stakingKeeper),
		bankKeeper,
		options.AddressCodec,
	)

	s[stakingPrecompile.Address()] = stakingPrecompile
	return s
}

func (s StaticPrecompiles) WithDistributionPrecompile(
	distributionKeeper distributionkeeper.Keeper,
	stakingKeeper stakingkeeper.Keeper,
	bankKeeper cmn.BankKeeper,
	opts ...Option,
) StaticPrecompiles {
	options := defaultOptionals()
	for _, opt := range opts {
		opt(&options)
	}

	distributionPrecompile := distprecompile.NewPrecompile(
		distributionKeeper,
		distributionkeeper.NewMsgServerImpl(distributionKeeper),
		distributionkeeper.NewQuerier(distributionKeeper),
		stakingKeeper,
		bankKeeper,
		options.AddressCodec,
	)

	s[distributionPrecompile.Address()] = distributionPrecompile
	return s
}

func (s StaticPrecompiles) WithICS02Precompile(
	codec codec.Codec,
	clientKeeper ibcutils.ClientKeeper,
) StaticPrecompiles {
	ibcClientPrecompile := ics02precompile.NewPrecompile(
		codec,
		clientKeeper,
	)

	s[ibcClientPrecompile.Address()] = ibcClientPrecompile
	return s
}

func (s StaticPrecompiles) WithICS20Precompile(
	bankKeeper cmn.BankKeeper,
	stakingKeeper stakingkeeper.Keeper,
	transferKeeper *transferkeeper.Keeper,
	channelKeeper *channelkeeper.Keeper,
) StaticPrecompiles {
	ibcTransferPrecompile := ics20precompile.NewPrecompile(
		bankKeeper,
		stakingKeeper,
		transferKeeper,
		channelKeeper,
	)

	s[ibcTransferPrecompile.Address()] = ibcTransferPrecompile
	return s
}

func (s StaticPrecompiles) WithBankPrecompile(
	bankKeeper cmn.BankKeeper,
	erc20Keeper *erc20Keeper.Keeper,
) StaticPrecompiles {
	bankPrecompile := bankprecompile.NewPrecompile(bankKeeper, erc20Keeper)
	s[bankPrecompile.Address()] = bankPrecompile
	return s
}

func (s StaticPrecompiles) WithGovPrecompile(
	govKeeper govkeeper.Keeper,
	bankKeeper cmn.BankKeeper,
	codec codec.Codec,
	opts ...Option,
) StaticPrecompiles {
	options := defaultOptionals()
	for _, opt := range opts {
		opt(&options)
	}

	govPrecompile := govprecompile.NewPrecompile(
		govkeeper.NewMsgServerImpl(&govKeeper),
		govkeeper.NewQueryServer(&govKeeper),
		bankKeeper,
		codec,
		options.AddressCodec,
	)

	s[govPrecompile.Address()] = govPrecompile
	return s
}

func (s StaticPrecompiles) WithSlashingPrecompile(
	slashingKeeper slashingkeeper.Keeper,
	bankKeeper cmn.BankKeeper,
	opts ...Option,
) StaticPrecompiles {
	options := defaultOptionals()
	for _, opt := range opts {
		opt(&options)
	}

	slashingPrecompile := slashingprecompile.NewPrecompile(
		slashingKeeper,
		slashingkeeper.NewMsgServerImpl(slashingKeeper),
		bankKeeper,
		options.ValidatorAddrCodec,
		options.ConsensusAddrCodec,
	)

	s[slashingPrecompile.Address()] = slashingPrecompile
	return s
}
