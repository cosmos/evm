package network

import (
	"fmt"
	"math"
	"math/big"
	"time"

	gethparams "github.com/ethereum/go-ethereum/params"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmversion "github.com/cometbft/cometbft/proto/tendermint/version"
	cmttypes "github.com/cometbft/cometbft/types"
	"github.com/cometbft/cometbft/version"

	"github.com/cosmos/evm"
	"github.com/cosmos/evm/testutil/integration"
	basenetwork "github.com/cosmos/evm/testutil/integration/base/network"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// Network is the interface that wraps the methods to interact with integration test network.
//
// It was designed to avoid users to access module's keepers directly and force integration tests
// to be closer to the real user's behavior.
type Network interface {
	basenetwork.Network

	GetBaseDecimal() evmtypes.Decimals
	GetEIP155ChainID() *big.Int
	GetEVMChainConfig() *gethparams.ChainConfig

	// Clients
	GetERC20Client() erc20types.QueryClient
	GetEvmClient() evmtypes.QueryClient
	GetGovClient() govtypes.QueryClient
	GetFeeMarketClient() feemarkettypes.QueryClient
	GetMintClient() minttypes.QueryClient
}

type CreateEvmApp func(chainID string, evmChainID uint64, customBaseAppOptions ...func(*baseapp.BaseApp)) evm.EvmApp

var _ Network = (*IntegrationNetwork)(nil)

// IntegrationNetwork is the implementation of the Network interface for integration tests.
type IntegrationNetwork struct {
	cfg         Config
	ctx         sdktypes.Context
	validators  []stakingtypes.Validator
	app         evm.EvmApp
	baseDecimal evmtypes.Decimals

	// This is only needed for IBC chain testing setup
	valSet     *cmttypes.ValidatorSet
	valSigners map[string]cmttypes.PrivValidator
}

// New configures and initializes a new integration Network instance with
// the given configuration options. If no configuration options are provided
// it uses the default configuration.
//
// It panics if an error occurs.
func New(createEvmApp CreateEvmApp, opts ...ConfigOption) *IntegrationNetwork {
	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	cfg := DefaultConfig()
	// Modify the default config with the given options
	for _, opt := range opts {
		opt(&cfg)
	}

	ctx := sdktypes.Context{}
	network := &IntegrationNetwork{
		cfg:        cfg,
		ctx:        ctx,
		validators: []stakingtypes.Validator{},
	}

	// create a new testing app with the following params
	evmApp := createEvmApp(cfg.chainID, cfg.eip155ChainID.Uint64(), cfg.customBaseAppOpts...)
	err := network.configureAndInitChain(evmApp)
	if err != nil {
		panic(err)
	}
	return network
}

// PrefundedAccountInitialBalance is the amount of tokens that each
// prefunded account has at genesis. It represents a 100k amount expressed
// in the 18 decimals representation.
var PrefundedAccountInitialBalance, _ = sdkmath.NewIntFromString("100_000_000_000_000_000_000_000")

// configureAndInitChain initializes the network with the given configuration.
// It creates the genesis state and starts the network.
func (n *IntegrationNetwork) configureAndInitChain(evmApp evm.EvmApp) error {
	// --------------------------------------------------------------------------------------------
	// Apply changes deriving from possible config options
	// FIX: for sure there exists a better way to achieve that.
	// --------------------------------------------------------------------------------------------

	// The bonded amount should be updated to reflect the actual base denom
	baseDecimals := n.cfg.chainCoins.BaseDecimals()
	n.baseDecimal = baseDecimals
	bondedAmount := GetInitialBondedAmount(baseDecimals)

	// create validator set with the amount of validators specified in the config
	// with the default power of 1.
	valSet, valSigners := createValidatorSetAndSigners(n.cfg.amountOfValidators)
	totalBonded := bondedAmount.Mul(sdkmath.NewInt(int64(n.cfg.amountOfValidators)))

	// Build staking type validators and delegations
	validators, err := createStakingValidators(valSet.Validators, bondedAmount, n.cfg.operatorsAddrs)
	if err != nil {
		return err
	}

	// create genesis accounts and funded balances based on the config.
	genAccounts, fundedAccountBalances := getGenAccountsAndBalances(n.cfg, validators)

	fundedAccountBalances = addBondedModuleAccountToFundedBalances(
		fundedAccountBalances,
		sdktypes.NewCoin(n.cfg.chainCoins.BaseDenom(), totalBonded),
	)

	delegations := createDelegations(validators, genAccounts[0].GetAddress())

	stakingParams := StakingCustomGenesisState{
		denom:       n.cfg.chainCoins.BaseDenom(),
		validators:  validators,
		delegations: delegations,
	}
	govParams := GovCustomGenesisState{
		denom: n.cfg.chainCoins.BaseDenom(),
	}

	fmParams := FeeMarketCustomGenesisState{
		baseFee: GetInitialBaseFeeAmount(n.cfg.chainCoins.BaseDecimals()),
	}

	mintParams := MintCustomGenesisState{
		denom:        n.cfg.chainCoins.BaseDenom(),
		inflationMax: sdkmath.LegacyNewDecWithPrec(0, 1),
		inflationMin: sdkmath.LegacyNewDecWithPrec(0, 1),
	}

	totalSupply := calculateTotalSupply(fundedAccountBalances)
	bankParams := BankCustomGenesisState{
		totalSupply: totalSupply,
		balances:    fundedAccountBalances,
	}

	// Get the corresponding slashing info and missed block info
	// for the created validators
	slashingParams, err := getValidatorsSlashingGen(validators, evmApp.GetStakingKeeper())
	if err != nil {
		return err
	}

	// Configure Genesis state
	genesisState := newDefaultGenesisState(
		evmApp,
		n.cfg.eip155ChainID.Uint64(),
		defaultGenesisParams{
			genAccounts: genAccounts,
			staking:     stakingParams,
			bank:        bankParams,
			feemarket:   fmParams,
			slashing:    slashingParams,
			gov:         govParams,
			mint:        mintParams,
		},
	)

	// modify genesis state if there're any custom genesis state
	// for specific modules
	genesisState, err = customizeGenesis(evmApp, n.cfg.customGenesisState, genesisState)
	if err != nil {
		return err
	}

	// Init chain
	stateBytes, err := cmtjson.MarshalIndent(genesisState, "", " ")
	if err != nil {
		return err
	}

	consensusParams := integration.DefaultConsensusParams
	if n.cfg.customConsensusParams != nil {
		consensusParams = n.cfg.customConsensusParams
	}
	now := time.Now()

	if _, err = evmApp.InitChain(
		&abcitypes.RequestInitChain{
			Time:            now,
			ChainId:         n.cfg.chainID,
			Validators:      []abcitypes.ValidatorUpdate{},
			ConsensusParams: consensusParams,
			AppStateBytes:   stateBytes,
		},
	); err != nil {
		return err
	}

	header := cmtproto.Header{
		ChainID:            n.cfg.chainID,
		Height:             evmApp.LastBlockHeight() + 1,
		AppHash:            evmApp.LastCommitID().Hash,
		Time:               now,
		ValidatorsHash:     valSet.Hash(),
		NextValidatorsHash: valSet.Hash(),
		ProposerAddress:    valSet.Proposer.Address,
		Version: tmversion.Consensus{
			Block: version.BlockProtocol,
		},
	}

	req := buildFinalizeBlockReq(header, valSet.Validators)
	if _, err := evmApp.FinalizeBlock(req); err != nil {
		return err
	}

	// TODO - this might not be the best way to initilize the context
	n.ctx = evmApp.GetBaseApp().NewContextLegacy(false, header)

	// Commit genesis changes
	if _, err := evmApp.Commit(); err != nil {
		return err
	}

	// Set networks global parameters
	var blockMaxGas uint64 = math.MaxUint64
	if consensusParams.Block != nil && consensusParams.Block.MaxGas > 0 {
		blockMaxGas = uint64(consensusParams.Block.MaxGas) //#nosec G115 -- max gas will not exceed uint64
	}

	n.app = evmApp
	n.ctx = n.ctx.WithConsensusParams(*consensusParams)
	n.ctx = n.ctx.WithBlockGasMeter(evmtypes.NewInfiniteGasMeterWithLimit(blockMaxGas))

	n.validators = validators
	n.valSet = valSet
	n.valSigners = valSigners

	return nil
}

// GetConfig returns the network's configuration
func (n *IntegrationNetwork) GetBaseDecimal() evmtypes.Decimals {
	return n.baseDecimal
}

// GetContext returns the network's context
func (n *IntegrationNetwork) GetContext() sdktypes.Context {
	return n.ctx
}

// GetQueryContext returns the network's context, but only set the fields that Cosmos SDK sets when it creates a query context.
// ref: https://github.com/cosmos/cosmos-sdk/blob/fd170b51404b49bda767cf74727cd26329bfd115/baseapp/abci.go#L1298-L1314
func (n *IntegrationNetwork) GetQueryContext() sdktypes.Context {
	ctx := sdktypes.NewContext(n.ctx.MultiStore(), n.ctx.BlockHeader(), true, n.ctx.Logger()).
		WithMinGasPrices(n.ctx.MinGasPrices()).
		WithGasMeter(storetypes.NewGasMeter(n.ctx.GasMeter().Limit())).
		WithBlockHeader(n.ctx.BlockHeader()).
		WithBlockHeight(n.ctx.BlockHeight()).
		WithBlockTime(n.ctx.BlockTime())

	return ctx
}

// WithIsCheckTxCtx switches the network's checkTx property
func (n *IntegrationNetwork) WithIsCheckTxCtx(isCheckTx bool) sdktypes.Context {
	n.ctx = n.ctx.WithIsCheckTx(isCheckTx)
	return n.ctx
}

// GetChainID returns the network's chainID
func (n *IntegrationNetwork) GetChainID() string {
	return n.cfg.chainID
}

// GetEIP155ChainID returns the network EIP-155 chainID number
func (n *IntegrationNetwork) GetEIP155ChainID() *big.Int {
	return n.cfg.eip155ChainID
}

// GetEVMChainConfig returns the network's EVM chain config
func (n *IntegrationNetwork) GetEVMChainConfig() *gethparams.ChainConfig {
	return evmtypes.GetEthChainConfig()
}

// GetBaseDenom returns the network's base denom
func (n *IntegrationNetwork) GetBaseDenom() string {
	return n.cfg.chainCoins.baseCoin.Denom
}

// GetEVMDenom returns the network's evm denom
func (n *IntegrationNetwork) GetEVMDenom() string {
	return n.cfg.chainCoins.evmCoin.Denom
}

// GetOtherDenoms returns network's other supported denoms
func (n *IntegrationNetwork) GetOtherDenoms() []string {
	return n.cfg.otherCoinDenoms
}

// GetValidators returns the network's validators
func (n *IntegrationNetwork) GetValidators() []stakingtypes.Validator {
	return n.validators
}

// GetEncodingConfig returns the network's encoding configuration
func (n *IntegrationNetwork) GetEncodingConfig() sdktestutil.TestEncodingConfig {
	return sdktestutil.TestEncodingConfig{
		InterfaceRegistry: n.app.InterfaceRegistry(),
		Codec:             n.app.AppCodec(),
		TxConfig:          n.app.GetTxConfig(),
		Amino:             n.app.LegacyAmino(),
	}
}

// BroadcastTxSync broadcasts the given txBytes to the network and returns the response.
// TODO - this should be change to gRPC
func (n *IntegrationNetwork) BroadcastTxSync(txBytes []byte) (abcitypes.ExecTxResult, error) {
	header := n.ctx.BlockHeader()
	// Update block header and BeginBlock
	header.Height++
	header.AppHash = n.app.LastCommitID().Hash
	// Calculate new block time after duration
	newBlockTime := header.Time.Add(time.Second)
	header.Time = newBlockTime

	req := buildFinalizeBlockReq(header, n.valSet.Validators, txBytes)

	// dont include the DecidedLastCommit because we're not committing the changes
	// here, is just for broadcasting the tx. To persist the changes, use the
	// NextBlock or NextBlockAfter functions
	req.DecidedLastCommit = abcitypes.CommitInfo{}

	blockRes, err := n.app.GetBaseApp().FinalizeBlock(req)
	if err != nil {
		return abcitypes.ExecTxResult{}, err
	}
	if len(blockRes.TxResults) != 1 {
		return abcitypes.ExecTxResult{}, fmt.Errorf("unexpected number of tx results. Expected 1, got: %d", len(blockRes.TxResults))
	}
	return *blockRes.TxResults[0], nil
}

// Simulate simulates the given txBytes to the network and returns the simulated response.
// TODO - this should be change to gRPC
func (n *IntegrationNetwork) Simulate(txBytes []byte) (*txtypes.SimulateResponse, error) {
	gas, result, err := n.app.GetBaseApp().Simulate(txBytes)
	if err != nil {
		return nil, err
	}
	return &txtypes.SimulateResponse{
		GasInfo: &gas,
		Result:  result,
	}, nil
}

// CheckTx calls the BaseApp's CheckTx method with the given txBytes to the network and returns the response.
func (n *IntegrationNetwork) CheckTx(txBytes []byte) (*abcitypes.ResponseCheckTx, error) {
	req := &abcitypes.RequestCheckTx{Tx: txBytes}
	res, err := n.app.GetBaseApp().CheckTx(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}
