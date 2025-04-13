package keeper_test

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/testutil/integration/os/network"
	"github.com/cosmos/evm/x/precisebank/keeper"
	"github.com/cosmos/evm/x/precisebank/types"
	"github.com/cosmos/evm/x/precisebank/types/mocks"
)

// testData defines necessary fields for testing keeper store methods and mocks
// for unit tests without full app setup.
type testData struct {
	ctx      sdk.Context
	keeper   keeper.Keeper
	storeKey *storetypes.KVStoreKey
	bk       *mocks.MockBankKeeper
	ak       *mocks.MockAccountKeeper
}

// NewMockedTestData creates a new testData instance with mocked bank and
// account keepers.
func NewMockedTestData(t *testing.T) testData {
	storeKey := storetypes.NewKVStoreKey(types.ModuleName)
	// Not required by module, but needs to be non-nil for context
	tKey := storetypes.NewTransientStoreKey("transient_test")
	ctx := testutil.DefaultContext(storeKey, tKey)

	bk := mocks.NewMockBankKeeper(t)
	ak := mocks.NewMockAccountKeeper(t)

	nw := network.NewUnitTestNetwork()
	cdc := nw.App.AppCodec()
	k := keeper.NewKeeper(cdc, storeKey, bk, ak)

	return testData{
		ctx:      ctx,
		keeper:   k,
		storeKey: storeKey,
		bk:       bk,
		ak:       ak,
	}
}

func c(denom string, amount int64) sdk.Coin        { return sdk.NewInt64Coin(denom, amount) }
func ci(denom string, amount sdkmath.Int) sdk.Coin { return sdk.NewCoin(denom, amount) }
func cs(coins ...sdk.Coin) sdk.Coins               { return sdk.NewCoins(coins...) }
