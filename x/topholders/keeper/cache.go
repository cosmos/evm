package keeper

import (
	"context"
	"sort"
	"time"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/cosmos/evm/x/topholders/types"
)

// getModuleTag returns a human-readable tag for module addresses
func getModuleTag(address string) string {
	// Map of known module addresses to their tags
	moduleAddresses := map[string]string{
		authtypes.NewModuleAddress(authtypes.FeeCollectorName).String():     "Fee Collector",
		authtypes.NewModuleAddress(distrtypes.ModuleName).String():          "Distribution",
		authtypes.NewModuleAddress(govtypes.ModuleName).String():            "Governance",
		authtypes.NewModuleAddress(stakingtypes.BondedPoolName).String():    "Bonded Pool",
		authtypes.NewModuleAddress(stakingtypes.NotBondedPoolName).String(): "Not Bonded Pool",
		authtypes.NewModuleAddress("mint").String():                         "Mint",
		authtypes.NewModuleAddress("ibc").String():                          "IBC",
		authtypes.NewModuleAddress("transfer").String():                     "IBC Transfer",
	}

	if tag, exists := moduleAddresses[address]; exists {
		return tag
	}
	return ""
}

// shouldExcludeAddress returns true if the address should be excluded from top holders
func shouldExcludeAddress(address string) bool {
	// Exclude fee collector as it's an accumulation of all transaction fees
	feeCollectorAddr := authtypes.NewModuleAddress(authtypes.FeeCollectorName).String()
	return address == feeCollectorAddr
}

// UpdateCache updates the top holders cache by scanning all accounts
func (k *Keeper) UpdateCache(ctx context.Context) error {
	k.SetUpdating(true)
	defer k.SetUpdating(false)

	k.logger.Info("starting top holders cache update")
	start := time.Now()

	sdkCtx := sdk.UnwrapSDKContext(ctx)
	
	// Get the bond denom for staking calculations
	bondDenom, err := k.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return err
	}

	// Map to store holder information
	holderMap := make(map[string]*types.HolderInfo)

	// Step 1: Collect all account balances
	k.bankKeeper.IterateAllBalances(ctx, func(address sdk.AccAddress, coin sdk.Coin) bool {
		// Only process the bond denom (main token)
		if coin.Denom != bondDenom {
			return false
		}

		addrStr := address.String()

		// Skip excluded addresses (like fee collector)
		if shouldExcludeAddress(addrStr) {
			return false
		}

		if holderMap[addrStr] == nil {
			holderMap[addrStr] = &types.HolderInfo{
				Address:          addrStr,
				LiquidBalance:    math.ZeroInt(),
				BondedBalance:    math.ZeroInt(),
				UnbondingBalance: math.ZeroInt(),
				ModuleTag:        getModuleTag(addrStr),
			}
		}
		holderMap[addrStr].LiquidBalance = coin.Amount
		return false
	})

	// Step 2: Collect all bonded delegations
	delegations, err := k.stakingKeeper.GetAllDelegations(ctx)
	if err != nil {
		k.logger.Error("failed to get all delegations", "error", err)
		// Continue without staking data rather than failing completely
	} else {
		for _, delegation := range delegations {
			delAddr := delegation.DelegatorAddress

			// Skip excluded addresses
			if shouldExcludeAddress(delAddr) {
				continue
			}

			// Get the delegation balance
			delBalance := delegation.Shares.TruncateInt()

			if holderMap[delAddr] == nil {
				holderMap[delAddr] = &types.HolderInfo{
					Address:          delAddr,
					LiquidBalance:    math.ZeroInt(),
					BondedBalance:    math.ZeroInt(),
					UnbondingBalance: math.ZeroInt(),
					ModuleTag:        getModuleTag(delAddr),
				}
			}
			holderMap[delAddr].BondedBalance = holderMap[delAddr].BondedBalance.Add(delBalance)
		}
	}

	// Step 3: Collect all unbonding delegations

	for addr := range holderMap {
		accAddr, err := sdk.AccAddressFromBech32(addr)
		if err != nil {
			continue
		}

		unbondingDelegations, err := k.stakingKeeper.GetAllUnbondingDelegations(ctx, accAddr)
		if err != nil {

			continue
		}

		for _, unbonding := range unbondingDelegations {
			for _, entry := range unbonding.Entries {
				holderMap[addr].UnbondingBalance = holderMap[addr].UnbondingBalance.Add(entry.Balance)
			}
		}
	}

	// Step 4: Calculate total balances and filter out zero balances
	var holders []types.HolderInfo
	for _, holder := range holderMap {
		holder.TotalBalance = holder.LiquidBalance.Add(holder.BondedBalance).Add(holder.UnbondingBalance)


		if holder.TotalBalance.GT(math.ZeroInt()) {
			holders = append(holders, *holder)
		}
	}

	// Step 4: Sort by total balance (descending)
	sort.Slice(holders, func(i, j int) bool {
		return holders[i].TotalBalance.GT(holders[j].TotalBalance)
	})

	// Step 5: Take top 1000 and assign ranks
	if len(holders) > 1000 {
		holders = holders[:1000]
	}
	
	for i := range holders {
		holders[i].Rank = uint32(i + 1)
	}

	// Step 6: Create and store the cache
	cache := types.NewTopHoldersCache(
		holders,
		time.Now().Unix(),
		sdkCtx.BlockHeight(),
	)

	if err := k.SetTopHoldersCache(ctx, cache); err != nil {
		return err
	}

	duration := time.Since(start)
	k.logger.Info("completed top holders cache update", 
		"duration", duration,
		"total_holders", len(holders),
		"block_height", sdkCtx.BlockHeight(),
	)

	return nil
}

// ForceUpdateCache forces an immediate cache update regardless of timing
func (k *Keeper) ForceUpdateCache(ctx context.Context) error {
	if k.IsUpdating() {
		return types.ErrUpdateInProgress
	}
	return k.UpdateCache(ctx)
}
