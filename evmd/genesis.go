package evmd

import (
	"encoding/json"
	"time"

	"cosmossdk.io/math"
	"github.com/cosmos/evm/evmd/cmd/evmd/config"
	testconstants "github.com/cosmos/evm/testutil/constants"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GenesisState of the blockchain is represented here as a map of raw json
// messages key'd by an identifier string.
// The identifier is used to determine which module genesis information belongs
// to so it may be appropriately routed during init chain.
// Within this application default genesis information is retrieved from
// the ModuleBasicManager which populates json from each BasicModule
// object provided to it during init.
type GenesisState map[string]json.RawMessage

// NewEVMGenesisState returns the default genesis state for the EVM module.
//
// NOTE: for the example chain implementation we need to set the default EVM denomination,
// enable ALL precompiles, and include default preinstalls.
func NewEVMGenesisState() *evmtypes.GenesisState {
	evmGenState := evmtypes.DefaultGenesisState()
	evmGenState.Params.ActiveStaticPrecompiles = evmtypes.AvailableStaticPrecompiles
	evmGenState.Preinstalls = evmtypes.DefaultPreinstalls

	return evmGenState
}

// NewErc20GenesisState returns the default genesis state for the ERC20 module.
//
// NOTE: for the example chain implementation we are also adding a default token pair,
// which is the base denomination of the chain (i.e. the WEVMOS contract).
func NewErc20GenesisState() *erc20types.GenesisState {
	erc20GenState := erc20types.DefaultGenesisState()
	erc20GenState.TokenPairs = testconstants.ExampleTokenPairs
	erc20GenState.NativePrecompiles = []string{testconstants.WEVMOSContractMainnet}

	return erc20GenState
}

// NewMintGenesisState returns the default genesis state for the mint module.
//
// NOTE: for the example chain implementation we are also adding a default minter.
func NewMintGenesisState() *minttypes.GenesisState {
	mintGenState := minttypes.DefaultGenesisState()
	mintGenState.Params.MintDenom = config.ExampleChainDenom

	return mintGenState
}

// NewEpixMintGenesisState returns the genesis state for the Epix mint module.
//
// NOTE: This uses the custom Epix tokenomics with the specified minting schedule.
func NewEpixMintGenesisState() *minttypes.GenesisState {
	return NewEpixGenesisState()
}

// NewEpixErc20GenesisState returns the genesis state for the Epix ERC20 module.
//
// NOTE: This sets up the WEPIX token pair for the Epix chain.
func NewEpixErc20GenesisState() *erc20types.GenesisState {
	erc20GenState := erc20types.DefaultGenesisState()
	// For now, use empty token pairs - these will be set up after deployment
	erc20GenState.TokenPairs = []erc20types.TokenPair{}
	erc20GenState.NativePrecompiles = []string{} // Will be set after WEPIX deployment

	return erc20GenState
}

// NewEpixStakingGenesisState returns the genesis state for the Epix staking module.
//
// NOTE: This sets up the staking module to use aepix as the bond denomination.
func NewEpixStakingGenesisState() *stakingtypes.GenesisState {
	stakingGenState := stakingtypes.DefaultGenesisState()
	stakingGenState.Params.BondDenom = config.EpixChainDenom // Use aepix instead of stake

	return stakingGenState
}

// NewEpixGovGenesisState returns the genesis state for the Epix governance module.
//
// NOTE: This sets up the governance module to use aepix for deposits.
func NewEpixGovGenesisState() *govtypes.GenesisState {
	govGenState := govtypes.DefaultGenesisState()

	// Update min deposit to use aepix (10 EPIX = 10 * 10^18 aepix)
	tenEpix := math.NewIntWithDecimal(10, 18)
	fiftyEpix := math.NewIntWithDecimal(50, 18)

	govGenState.Params.MinDeposit = sdk.NewCoins(sdk.NewCoin(config.EpixChainDenom, tenEpix))
	govGenState.Params.ExpeditedMinDeposit = sdk.NewCoins(sdk.NewCoin(config.EpixChainDenom, fiftyEpix))

	return govGenState
}

// NewEpixDistributionGenesisState returns the genesis state for the Epix distribution module.
//
// NOTE: This sets up the modern reward distribution with 50% to staking rewards and 50% to community pool.
// These ratios can be adjusted through governance proposals.
func NewEpixDistributionGenesisState() *distrtypes.GenesisState {
	distrGenState := distrtypes.DefaultGenesisState()

	// Set community tax to 2% (0.02) - standard for most Cosmos chains
	// This means 2% of minted tokens go to the community pool
	// The remaining 98% goes to staking rewards distributed equally among all validators
	distrGenState.Params.CommunityTax = math.LegacyMustNewDecFromStr("0.020000000000000000")

	// Enable withdraw address changes
	distrGenState.Params.WithdrawAddrEnabled = true

	return distrGenState
}

// NewEpixSlashingGenesisState returns the genesis state for the Epix slashing module.
//
// NOTE: This sets up slashing parameters optimized for Epix network:
// - 12-hour rolling window (21,600 blocks at 2 seconds per block)
// - 5% minimum signing requirement
// - 60 second jail duration for downtime
// - 5% slash for double signing, 1% for downtime
func NewEpixSlashingGenesisState() *slashingtypes.GenesisState {
	slashingGenState := slashingtypes.DefaultGenesisState()

	// Set signed blocks window to 21,600 blocks (12-hour rolling window at 2 seconds per block)
	slashingGenState.Params.SignedBlocksWindow = 21600

	// Set minimum signed per window to 5% (validators must sign at least 5% of blocks)
	slashingGenState.Params.MinSignedPerWindow = math.LegacyMustNewDecFromStr("0.050000000000000000")

	// Set downtime jail duration to 60 seconds
	slashingGenState.Params.DowntimeJailDuration = time.Second * 60

	// Set double sign slash fraction to 5%
	slashingGenState.Params.SlashFractionDoubleSign = math.LegacyMustNewDecFromStr("0.050000000000000000")

	// Set downtime slash fraction to 1%
	slashingGenState.Params.SlashFractionDowntime = math.LegacyMustNewDecFromStr("0.010000000000000000")

	return slashingGenState
}

// NewFeeMarketGenesisState returns the default genesis state for the feemarket module.
//
// NOTE: for the example chain implementation we are disabling the base fee.
func NewFeeMarketGenesisState() *feemarkettypes.GenesisState {
	feeMarketGenState := feemarkettypes.DefaultGenesisState()
	feeMarketGenState.Params.NoBaseFee = true

	return feeMarketGenState
}
