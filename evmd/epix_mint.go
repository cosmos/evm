package evmd

import (
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	mintkeeper "github.com/cosmos/cosmos-sdk/x/mint/keeper"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	"github.com/cosmos/evm/evmd/cmd/evmd/config"
)

// EpixTokenomics defines the Epix chain tokenomics parameters
type EpixTokenomics struct {
	// Genesis supply: 23,689,538 EPIX (Airdrop + Community Pool seed)
	GenesisSupply math.Int

	// Airdrop allocation: 11,844,769 EPIX
	AirdropAllocation math.Int

	// Community pool: 11,844,769 EPIX
	CommunityPoolAllocation math.Int

	// Initial minting rate: 10.527B EPIX per year
	InitialMintingRate math.Int

	// Reduction period: 1 year (25% reduction rate)
	ReductionPeriod time.Duration

	// Reduction rate: 25% (retention rate: 75%)
	ReductionRate math.LegacyDec

	// Max supply: 42B EPIX over 20 years
	MaxSupply math.Int

	// Timeline: 20 years
	Timeline time.Duration

	// Chain start time (will be set at genesis)
	ChainStartTime time.Time
}

// NewEpixTokenomics creates a new EpixTokenomics instance with the specified parameters
func NewEpixTokenomics() *EpixTokenomics {
	// Convert values to proper units (aepix = 10^18 epix)
	// 1 EPIX = 10^18 aepix
	epixToAepix := math.NewIntWithDecimal(1, 18)

	return &EpixTokenomics{
		GenesisSupply:           math.NewInt(23689538).Mul(epixToAepix),
		AirdropAllocation:       math.NewInt(11844769).Mul(epixToAepix),
		CommunityPoolAllocation: math.NewInt(11844769).Mul(epixToAepix),
		InitialMintingRate:      math.NewInt(10527000000).Mul(epixToAepix), // 10.527B EPIX per year
		ReductionPeriod:         365 * 24 * time.Hour,                      // 1 year
		ReductionRate:           math.LegacyNewDecWithPrec(25, 2),          // 25% reduction (0.25)
		MaxSupply:               math.NewInt(42000000000).Mul(epixToAepix), // 42B EPIX
		Timeline:                20 * 365 * 24 * time.Hour,                 // 20 years
	}
}

// GetCurrentMintingRate calculates the current minting rate based on the time elapsed since chain start
func (et *EpixTokenomics) GetCurrentMintingRate(ctx sdk.Context) math.Int {
	if et.ChainStartTime.IsZero() {
		// If chain start time is not set, use block time as reference
		et.ChainStartTime = ctx.BlockTime()
	}

	timeElapsed := ctx.BlockTime().Sub(et.ChainStartTime)

	// If we've exceeded the timeline, no more minting
	if timeElapsed >= et.Timeline {
		return math.ZeroInt()
	}

	// Calculate how many reduction periods have passed
	reductionPeriodsPassed := int64(timeElapsed / et.ReductionPeriod)

	// Calculate current minting rate: initial_rate * (0.75)^periods_passed
	retentionRate := math.LegacyOneDec().Sub(et.ReductionRate) // 0.75
	currentRate := math.LegacyNewDecFromInt(et.InitialMintingRate)

	for i := int64(0); i < reductionPeriodsPassed; i++ {
		currentRate = currentRate.Mul(retentionRate)
	}

	return currentRate.TruncateInt()
}

// GetBlockProvision calculates the provision for the current block
func (et *EpixTokenomics) GetBlockProvision(ctx sdk.Context, blocksPerYear uint64) math.Int {
	currentMintingRate := et.GetCurrentMintingRate(ctx)

	if currentMintingRate.IsZero() || blocksPerYear == 0 {
		return math.ZeroInt()
	}

	// Calculate per-block provision: annual_rate / blocks_per_year
	blockProvision := math.LegacyNewDecFromInt(currentMintingRate).QuoInt64(int64(blocksPerYear))
	return blockProvision.TruncateInt()
}

// EpixMintFn is the custom mint function for the Epix chain
func EpixMintFn(tokenomics *EpixTokenomics) mintkeeper.MintFn {
	return func(ctx sdk.Context, k *mintkeeper.Keeper) error {
		// Get current minter state
		minter, err := k.Minter.Get(ctx)
		if err != nil {
			return err
		}

		// Get mint parameters
		params, err := k.Params.Get(ctx)
		if err != nil {
			return err
		}

		// Check if we should continue minting based on time elapsed
		// Note: We'll implement max supply checking through governance or other mechanisms
		timeElapsed := ctx.BlockTime().Sub(tokenomics.ChainStartTime)
		if timeElapsed >= tokenomics.Timeline {
			// Timeline exceeded, no more minting
			return nil
		}

		// Calculate block provision based on Epix tokenomics
		blockProvision := tokenomics.GetBlockProvision(ctx, params.BlocksPerYear)

		if blockProvision.IsZero() {
			return nil
		}

		// For now, we'll rely on the time-based minting schedule
		// Max supply enforcement can be added through governance or other mechanisms

		// Mint the coins
		mintedCoin := sdk.NewCoin(params.MintDenom, blockProvision)
		mintedCoins := sdk.NewCoins(mintedCoin)

		err = k.MintCoins(ctx, mintedCoins)
		if err != nil {
			return err
		}

		// Send minted coins to fee collector
		err = k.AddCollectedFees(ctx, mintedCoins)
		if err != nil {
			return err
		}

		// Update minter state for tracking
		currentMintingRate := tokenomics.GetCurrentMintingRate(ctx)
		if params.BlocksPerYear > 0 {
			// Calculate annual provisions for tracking
			annualProvisions := math.LegacyNewDecFromInt(currentMintingRate)
			minter.AnnualProvisions = annualProvisions
		}

		// Update inflation for tracking (this is just for display/query purposes)
		// For Epix, we'll set inflation based on the current minting rate relative to a base supply
		// This is primarily for display in queries
		baseSupply := tokenomics.GenesisSupply.Add(math.NewInt(1000000000)) // Add some base to avoid division by zero
		inflation := math.LegacyNewDecFromInt(currentMintingRate).QuoInt(baseSupply)
		minter.Inflation = inflation

		// Save updated minter state
		if err := k.Minter.Set(ctx, minter); err != nil {
			return err
		}

		// Emit events using the standard SDK event format
		ctx.EventManager().EmitEvent(
			sdk.NewEvent(
				minttypes.EventTypeMint,
				sdk.NewAttribute(minttypes.AttributeKeyBondedRatio, math.LegacyZeroDec().String()), // Not applicable for Epix
				sdk.NewAttribute(minttypes.AttributeKeyInflation, minter.Inflation.String()),
				sdk.NewAttribute(minttypes.AttributeKeyAnnualProvisions, minter.AnnualProvisions.String()),
				sdk.NewAttribute(sdk.AttributeKeyAmount, mintedCoin.Amount.String()),
			),
		)

		ctx.Logger().Info(
			"minted coins from Epix tokenomics",
			"minted_coins", mintedCoins,
			"current_minting_rate", currentMintingRate,
		)

		return nil
	}
}

// NewEpixGenesisState creates the genesis state for Epix chain with custom tokenomics
func NewEpixGenesisState() *minttypes.GenesisState {
	tokenomics := NewEpixTokenomics()

	// Create minter with initial state
	minter := minttypes.NewMinter(
		math.LegacyZeroDec(), // inflation will be calculated dynamically
		math.LegacyNewDecFromInt(tokenomics.InitialMintingRate), // annual provisions
	)

	// Create params for Epix
	params := minttypes.NewParams(
		config.EpixChainDenom, // mint denom
		math.LegacyZeroDec(),  // inflation rate change (not used in custom mint)
		math.LegacyZeroDec(),  // inflation max (not used in custom mint)
		math.LegacyZeroDec(),  // inflation min (not used in custom mint)
		math.LegacyZeroDec(),  // goal bonded (not used in custom mint)
		6311520,               // blocks per year (assuming ~5 second block times)
	)

	return &minttypes.GenesisState{
		Minter: minter,
		Params: params,
	}
}
