package keeper_test

import (
	"testing"

	evmosencoding "github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/evmd"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/precisebank/keeper"
	"github.com/cosmos/evm/x/precisebank/types"
	"github.com/cosmos/evm/x/precisebank/types/mocks"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

// newMockedTestData creates a new testData instance with mocked bank and
// account keepers.
func newMockedTestData(t *testing.T) testData {
	t.Helper()

	storeKey := storetypes.NewKVStoreKey(types.ModuleName)
	// Not required by module, but needs to be non-nil for context
	tKey := storetypes.NewTransientStoreKey("transient_test")
	ctx := testutil.DefaultContext(storeKey, tKey)

	bk := mocks.NewMockBankKeeper(t)
	ak := mocks.NewMockAccountKeeper(t)

	chainID := testconstants.SixDecimalsChainID.EVMChainID
	cfg := evmosencoding.MakeConfig(chainID)
	cdc := cfg.Codec
	k := keeper.NewKeeper(cdc, storeKey, bk, ak)
	err := evmd.EvmAppOptions(chainID)
	if err != nil {
		return testData{}
	}

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
