package keeper_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/cosmos/evm/x/epixmint"
	"github.com/cosmos/evm/x/epixmint/keeper"
	"github.com/cosmos/evm/x/epixmint/types"
)

type KeeperTestSuite struct {
	suite.Suite

	ctx           sdk.Context
	keeper        keeper.Keeper
	bankKeeper    *MockBankKeeper
	accountKeeper *MockAccountKeeper
	cdc           codec.BinaryCodec
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

func (s *KeeperTestSuite) SetupTest() {
	encCfg := moduletestutil.MakeTestEncodingConfig(epixmint.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(s.T(), key, storetypes.NewTransientStoreKey("transient_test"))
	s.ctx = testCtx.Ctx

	s.cdc = encCfg.Codec
	s.bankKeeper = &MockBankKeeper{}
	s.accountKeeper = &MockAccountKeeper{}

	s.keeper = keeper.NewKeeper(
		s.cdc,
		key,
		s.bankKeeper,
		s.accountKeeper,
		&MockDistributionKeeper{}, // Add mock distribution keeper
		&MockStakingKeeper{},      // Add mock staking keeper
		authtypes.NewModuleAddress("gov").String(),
	)
}

func (s *KeeperTestSuite) TestGetSetParams() {
	params := types.DefaultParams()

	// Test setting params
	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	// Test getting params
	retrievedParams := s.keeper.GetParams(s.ctx)
	s.Require().Equal(params, retrievedParams)
}

func (s *KeeperTestSuite) TestMintCoins() {
	// Set up default params
	params := types.DefaultParams()
	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	// Mock current supply (below max)
	currentSupply, _ := math.NewIntFromString("1000000000000000000000000000") // 1B EPIX in aepix
	s.bankKeeper.SetSupply(params.MintDenom, currentSupply)

	// Test minting
	err = s.keeper.MintCoins(s.ctx)
	s.Require().NoError(err)

	// Verify mint was called
	s.Require().True(s.bankKeeper.MintCalled)
	s.Require().True(s.bankKeeper.SendCalled)
}

func (s *KeeperTestSuite) TestMintCoins_MaxSupplyReached() {
	// Set up default params
	params := types.DefaultParams()
	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	// Mock current supply at max
	s.bankKeeper.SetSupply(params.MintDenom, params.MaxSupply)

	// Test minting when max supply is reached
	err = s.keeper.MintCoins(s.ctx)
	s.Require().NoError(err)

	// Verify no mint was called
	s.Require().False(s.bankKeeper.MintCalled)
	s.Require().False(s.bankKeeper.SendCalled)
}

func (s *KeeperTestSuite) TestMintCoins_NearMaxSupply() {
	// Set up default params
	params := types.DefaultParams()
	err := s.keeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	// Calculate tokens per block using dynamic emission (at genesis, it's the initial amount)
	secondsPerYear := uint64(365 * 24 * 60 * 60)
	blocksPerYear := secondsPerYear / params.BlockTimeSeconds
	tokensPerBlock := params.InitialAnnualMintAmount.Quo(math.NewIntFromUint64(blocksPerYear))

	// Mock current supply very close to max (less than one block's worth)
	nearMaxSupply := params.MaxSupply.Sub(tokensPerBlock.QuoRaw(2)) // Half a block's worth below max
	s.bankKeeper.SetSupply(params.MintDenom, nearMaxSupply)

	// Test minting
	err = s.keeper.MintCoins(s.ctx)
	s.Require().NoError(err)

	// Verify mint was called with reduced amount
	s.Require().True(s.bankKeeper.MintCalled)
	s.Require().True(s.bankKeeper.SendCalled)

	// Verify the minted amount was capped
	expectedMintAmount := params.MaxSupply.Sub(nearMaxSupply)
	s.Require().Equal(expectedMintAmount, s.bankKeeper.LastMintedAmount)
}

// Mock implementations
type MockBankKeeper struct {
	supply           map[string]math.Int
	MintCalled       bool
	SendCalled       bool
	LastMintedAmount math.Int
}

func (m *MockBankKeeper) GetSupply(ctx context.Context, denom string) sdk.Coin {
	if amount, exists := m.supply[denom]; exists {
		return sdk.NewCoin(denom, amount)
	}
	return sdk.NewCoin(denom, math.ZeroInt())
}

func (m *MockBankKeeper) SetSupply(denom string, amount math.Int) {
	if m.supply == nil {
		m.supply = make(map[string]math.Int)
	}
	m.supply[denom] = amount
}

func (m *MockBankKeeper) MintCoins(ctx context.Context, moduleName string, amt sdk.Coins) error {
	m.MintCalled = true
	if len(amt) > 0 {
		m.LastMintedAmount = amt[0].Amount
	}
	return nil
}

func (m *MockBankKeeper) SendCoinsFromModuleToModule(ctx context.Context, senderModule, recipientModule string, amt sdk.Coins) error {
	m.SendCalled = true
	return nil
}

type MockAccountKeeper struct{}

func (m *MockAccountKeeper) GetModuleAddress(name string) sdk.AccAddress {
	return authtypes.NewModuleAddress(name)
}

func (m *MockAccountKeeper) GetModuleAccount(ctx context.Context, name string) sdk.ModuleAccountI {
	return nil
}

// MockDistributionKeeper implements the DistributionKeeper interface for testing
type MockDistributionKeeper struct{}

func (m *MockDistributionKeeper) FundCommunityPool(ctx context.Context, amount sdk.Coins, sender sdk.AccAddress) error {
	return nil
}

func (m *MockDistributionKeeper) AllocateTokensToValidator(ctx context.Context, val stakingtypes.ValidatorI, tokens sdk.DecCoins) error {
	return nil
}

// MockStakingKeeper implements the StakingKeeper interface for testing
type MockStakingKeeper struct{}

func (m *MockStakingKeeper) GetAllValidators(ctx context.Context) (validators []stakingtypes.Validator, err error) {
	return []stakingtypes.Validator{}, nil
}

func (m *MockStakingKeeper) BondedRatio(ctx context.Context) (ratio math.LegacyDec, err error) {
	return math.LegacyNewDec(1), nil
}
