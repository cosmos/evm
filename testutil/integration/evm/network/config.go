package network

import (
	"fmt"
	"math/big"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	testconstants "github.com/cosmos/evm/testutil/constants"
	testtx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// defaultChain represents the default chain ID used in the suite setup.
var defaultChain = testconstants.ExampleChainID

// Config defines the configuration for a chain.
// It allows for customization of the network to adjust to
// testing needs.
type Config struct {
	chainID       string
	eip155ChainID *big.Int

	customGenesisState    CustomGenesisState
	customConsensusParams *cmtproto.ConsensusParams

	customBaseAppOpts []func(*baseapp.BaseApp)

	amountOfValidators  int
	operatorsAddrs      []sdktypes.AccAddress
	initialBondedAmount math.Int

	chainCoins     ChainCoins
	initialAmounts InitialAmounts
	// otherCoinDenoms represents the other possible coin denominations that can be passed during
	// test suite intialization to provide other coins initial balances.
	otherCoinDenoms   []string
	preFundedAccounts []sdktypes.AccAddress
	balances          []banktypes.Balance
}

type CustomGenesisState map[string]interface{}

// DefaultConfig returns the default configuration for a chain.
func DefaultConfig() Config {
	account, _ := testtx.NewAccAddressAndKey()

	return Config{
		chainID:             testconstants.ExampleChainID.ChainID,
		eip155ChainID:       big.NewInt(testconstants.EighteenDecimalsChainID),
		chainCoins:          DefaultChainCoins(),
		initialAmounts:      DefaultInitialAmounts(),
		initialBondedAmount: DefaultInitialBondedAmount(),
		amountOfValidators:  3,

		// Only one account besides the validators
		preFundedAccounts: []sdktypes.AccAddress{account},

		// NOTE: Per default, the balances are left empty, and the pre-funded accounts are used.
		balances:           nil,
		customGenesisState: nil,
	}
}

func (cfg Config) GetChainCoins() ChainCoins {
	return cfg.chainCoins
}

// getGenAccountsAndBalances takes the network configuration and returns the used
// genesis accounts and balances.
//
// NOTE: If the balances are set, the pre-funded accounts are ignored.
func getGenAccountsAndBalances(cfg Config, validators []stakingtypes.Validator) (genAccounts []authtypes.GenesisAccount, balances []banktypes.Balance) {
	if len(cfg.balances) > 0 {
		balances = cfg.balances
		accounts := getAccAddrsFromBalances(balances)
		genAccounts = createGenesisAccounts(accounts)
	} else {
		genAccounts = createGenesisAccounts(cfg.preFundedAccounts)

		denomDecimals := cfg.chainCoins.DenomDecimalsMap()

		// All extra denom specified are represented with the base coin decimal.
		for _, denom := range cfg.otherCoinDenoms {
			denomDecimals[denom] = cfg.chainCoins.BaseDecimals()
		}

		balances = createBalances(cfg.preFundedAccounts, denomDecimals)
	}

	// append validators to genesis accounts and balances
	valAccs := make([]sdktypes.AccAddress, len(validators))
	for i, v := range validators {
		valAddr, err := sdktypes.ValAddressFromBech32(v.OperatorAddress)
		if err != nil {
			panic(fmt.Sprintf("failed to derive validator address from %q: %s", v.OperatorAddress, err.Error()))
		}
		valAccs[i] = sdktypes.AccAddress(valAddr.Bytes())
	}
	genAccounts = append(genAccounts, createGenesisAccounts(valAccs)...)

	return genAccounts, balances
}

// ConfigOption defines a function that can modify the NetworkConfig.
// The purpose of this is to force to be declarative when the default configuration
// requires to be changed.
type ConfigOption func(*Config)

// WithChainID sets a custom chainID for the network. Changing the chainID
// change automatically also the EVM coin used. It panics if the chainID is invalid.
func WithChainID(chainID testconstants.ChainID) ConfigOption {
	evmCoinInfo, found := testconstants.ExampleChainCoinInfo[chainID]
	if !found {
		panic(fmt.Sprintf(
			"chain id %q not found in chain coin info; available: %v",
			chainID,
			testconstants.ExampleChainCoinInfo,
		))
	}

	return func(cfg *Config) {
		cfg.chainID = chainID.ChainID
		cfg.eip155ChainID = big.NewInt(int64(chainID.EVMChainID)) //nolint:gosec // G115 // won't exceed uint64

		if cfg.chainCoins.IsBaseEqualToEVM() {
			cfg.chainCoins.baseCoin.Denom = evmCoinInfo.Denom
			cfg.chainCoins.baseCoin.Decimals = evmtypes.Decimals(evmCoinInfo.Decimals)
		}
		cfg.chainCoins.evmCoin.Denom = evmCoinInfo.Denom
		cfg.chainCoins.evmCoin.Decimals = evmtypes.Decimals(evmCoinInfo.Decimals)
	}
}

// WithAmountOfValidators sets the amount of validators for the network.
func WithAmountOfValidators(amount int) ConfigOption {
	return func(cfg *Config) {
		cfg.amountOfValidators = amount
	}
}

// WithPreFundedAccounts sets the pre-funded accounts for the network.
func WithPreFundedAccounts(accounts ...sdktypes.AccAddress) ConfigOption {
	return func(cfg *Config) {
		cfg.preFundedAccounts = accounts
	}
}

// WithBalances sets the specific balances for the pre-funded accounts, that
// are being set up for the network.
func WithBalances(balances ...banktypes.Balance) ConfigOption {
	return func(cfg *Config) {
		cfg.balances = append(cfg.balances, balances...)
	}
}

// WithBaseCoin sets the denom and decimals for the base coin in the network.
func WithBaseCoin(denom string, decimals uint8) ConfigOption {
	return func(cfg *Config) {
		cfg.chainCoins.baseCoin.Denom = denom
		cfg.chainCoins.baseCoin.Decimals = evmtypes.Decimals(decimals)
	}
}

// WithEVMCoin sets the denom and decimals for the evm coin in the network.
func WithEVMCoin(_ string, _ uint8) ConfigOption {
	// The evm config can be changed only via chain ID because it should be
	// handled properly from the configurator.
	panic("EVM coin can be changed only via ChainID: se WithChainID method")
}

// WithCustomGenesis sets the custom genesis of the network for specific modules.
func WithCustomGenesis(customGenesis CustomGenesisState) ConfigOption {
	return func(cfg *Config) {
		cfg.customGenesisState = customGenesis
	}
}

// WithOtherDenoms sets other possible coin denominations for the network.
func WithOtherDenoms(otherDenoms []string) ConfigOption {
	return func(cfg *Config) {
		cfg.otherCoinDenoms = otherDenoms
	}
}

// WithValidatorOperators overwrites the used operator address for the network instantiation.
func WithValidatorOperators(keys []sdktypes.AccAddress) ConfigOption {
	return func(cfg *Config) {
		cfg.operatorsAddrs = keys
	}
}

// WithCustomBaseAppOpts sets custom base app options for the network.
func WithCustomBaseAppOpts(opts ...func(*baseapp.BaseApp)) ConfigOption {
	return func(cfg *Config) {
		cfg.customBaseAppOpts = opts
	}
}

// WithConsensusParams sets the custom consensus parameters for the network.
func WithConsensusParams(params *cmtproto.ConsensusParams) ConfigOption {
	return func(cfg *Config) {
		if params != nil {
			cfg.customConsensusParams = params
		}
	}
}
