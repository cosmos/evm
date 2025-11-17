package evm

import (
	"encoding/json"

	erc20keeper "github.com/cosmos/evm/x/erc20/keeper"
	feemarketkeeper "github.com/cosmos/evm/x/feemarket/keeper"
	"github.com/cosmos/evm/x/ibc/callbacks/keeper"
	transferkeeper "github.com/cosmos/evm/x/ibc/transfer/keeper"
	precisebankkeeper "github.com/cosmos/evm/x/precisebank/keeper"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	storetypes "cosmossdk.io/store/types"
	evidencekeeper "cosmossdk.io/x/evidence/keeper"
	feegrantkeeper "cosmossdk.io/x/feegrant/keeper"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/mempool"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	consensusparamkeeper "github.com/cosmos/cosmos-sdk/x/consensus/keeper"
	distrkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

// TestApp captures the minimal functionality all test harnesses require.
type TestApp interface {
	servertypes.Application
	runtime.AppI
	InterfaceRegistry() types.InterfaceRegistry
	AppCodec() codec.Codec
	GetTxConfig() client.TxConfig
	LegacyAmino() *codec.LegacyAmino
	ChainID() string
	DefaultGenesis() map[string]json.RawMessage
	GetKey(storeKey string) *storetypes.KVStoreKey
	GetBaseApp() *baseapp.BaseApp
	LastCommitID() storetypes.CommitID
	LastBlockHeight() int64
	GetAnteHandler() sdk.AnteHandler
	MsgServiceRouter() *baseapp.MsgServiceRouter
	GetMempool() mempool.ExtMempool

	// keeper getters
	VMKeeperProvider
	BankKeeperProvider
	StakingKeeperProvider
}

// EvmApp defines the interface for an EVM application.
type EvmApp interface { //nolint:revive
	TestApp
	AccountKeeperProvider
	AnteHandlerProvider
	CallbackKeeperProvider
	ConsensusParamsKeeperProvider
	DistrKeeperProvider
	EvidenceKeeperProvider
	Erc20KeeperProvider
	Erc20KeeperSetter
	FeeGrantKeeperProvider
	FeeMarketKeeperProvider
	GovKeeperProvider
	KeyProvider
	MempoolProvider
	MintKeeperProvider
	MsgServiceRouterProvider
	PreciseBankKeeperProvider
	SlashingKeeperProvider
	TransferKeeperProvider
	TransferKeeperSetter
}

// Keeper provider interfaces allow tests to depend on the exact subset of
// keepers they need without requiring a fully fledged evmd application.
type (
	AccountKeeperProvider interface {
		GetAccountKeeper() authkeeper.AccountKeeper
	}
	AnteHandlerProvider interface {
		GetAnteHandler() sdk.AnteHandler
	}
	BankKeeperProvider interface {
		GetBankKeeper() bankkeeper.Keeper
	}
	CallbackKeeperProvider interface {
		GetCallbackKeeper() keeper.ContractKeeper
	}
	ChainIDProvider interface {
		ChainID() string
	}
	ConsensusParamsKeeperProvider interface {
		GetConsensusParamsKeeper() consensusparamkeeper.Keeper
	}
	DistrKeeperProvider interface {
		GetDistrKeeper() distrkeeper.Keeper
	}
	EvidenceKeeperProvider interface {
		GetEvidenceKeeper() *evidencekeeper.Keeper
	}
	VMKeeperProvider interface {
		GetEVMKeeper() *evmkeeper.Keeper
	}
	IBCKeeperProvider interface {
		GetIBCKeeper() *ibckeeper.Keeper
	}
	Erc20KeeperProvider interface {
		GetErc20Keeper() *erc20keeper.Keeper
	}
	Erc20KeeperSetter interface {
		SetErc20Keeper(erc20keeper.Keeper)
	}
	FeeGrantKeeperProvider interface {
		GetFeeGrantKeeper() feegrantkeeper.Keeper
	}
	FeeMarketKeeperProvider interface {
		GetFeeMarketKeeper() *feemarketkeeper.Keeper
	}
	GovKeeperProvider interface {
		GetGovKeeper() govkeeper.Keeper
	}
	KeyProvider interface {
		GetKey(storeKey string) *storetypes.KVStoreKey
	}
	MempoolProvider interface {
		GetMempool() mempool.ExtMempool
	}
	MintKeeperProvider interface {
		GetMintKeeper() mintkeeper.Keeper
	}
	MsgServiceRouterProvider interface {
		MsgServiceRouter() *baseapp.MsgServiceRouter
	}
	PreciseBankKeeperProvider interface {
		GetPreciseBankKeeper() *precisebankkeeper.Keeper
	}
	SlashingKeeperProvider interface {
		GetSlashingKeeper() slashingkeeper.Keeper
	}
	StakingKeeperProvider interface {
		GetStakingKeeper() *stakingkeeper.Keeper
	}
	TransferKeeperProvider interface {
		GetTransferKeeper() transferkeeper.Keeper
	}
	TransferKeeperSetter interface {
		SetTransferKeeper(transferkeeper.Keeper)
	}
)

type (
	IBCTestApp interface {
		TestApp
		ibctesting.TestingApp
	}
	IBCApp interface {
		EvmApp
		IBCKeeperProvider
	}
	// Precompile-focused application interfaces describe the exact keepers that a
	// given precompile test suite requires. External chains can implement only the
	// interfaces relevant to the suites they wish to run.
	BankPrecompileApp interface {
		TestApp
		BankKeeperProvider
		Erc20KeeperProvider
	}
	Bech32PrecompileApp interface {
		TestApp
	}
	DistributionPrecompileApp interface {
		TestApp
		DistrKeeperProvider
		StakingKeeperProvider
	}
	Erc20PrecompileApp interface {
		TestApp
		AccountKeeperProvider
		BankKeeperProvider
		Erc20KeeperProvider
		PreciseBankKeeperProvider
		TransferKeeperProvider
	}
	GovPrecompileApp interface {
		TestApp
		GovKeeperProvider
	}
	ICS20PrecompileApp interface {
		TestApp
		ChainIDProvider
		BankKeeperProvider
		StakingKeeperProvider
		TransferKeeperProvider
		IBCKeeperProvider
	}
	P256PrecompileApp interface {
		TestApp
	}
	SlashingPrecompileApp interface {
		TestApp
		SlashingKeeperProvider
		StakingKeeperProvider
	}
	StakingPrecompileApp interface {
		TestApp
		AccountKeeperProvider
		BankKeeperProvider
		StakingKeeperProvider
	}
	WERC20PrecompileApp interface {
		TestApp
		BankKeeperProvider
		Erc20KeeperProvider
		TransferKeeperProvider
	}

	// Base interface required by the integration network helpers. Any app used by
	// evm/testutil/integration must satisfy these keeper providers so the shared
	// network setup can access the necessary modules.
	IntegrationNetworkApp interface {
		TestApp
		AccountKeeperProvider
		DistrKeeperProvider
		Erc20KeeperProvider
		FeeMarketKeeperProvider
		GovKeeperProvider
		MintKeeperProvider
		PreciseBankKeeperProvider
		SlashingKeeperProvider
		EvidenceKeeperProvider
	}
	Erc20IntegrationApp interface {
		IntegrationNetworkApp
		TransferKeeperProvider
		IBCKeeperProvider
	}
	VMIntegrationApp interface {
		IntegrationNetworkApp
		ConsensusParamsKeeperProvider
	}
	AnteIntegrationApp interface {
		IntegrationNetworkApp
		FeeGrantKeeperProvider
		IBCKeeperProvider
	}
	IBCIntegrationApp interface {
		IntegrationNetworkApp
		TransferKeeperProvider
		IBCKeeperProvider
	}
	IBCCallbackIntegrationApp interface {
		IntegrationNetworkApp
		CallbackKeeperProvider
		IBCKeeperProvider
	}
)
