package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttime "github.com/cometbft/cometbft/types/time"

	"github.com/cosmos/evm/testutil/constants"
	vmkeeper "github.com/cosmos/evm/x/vm/keeper"
	vmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/types/mocks"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

type KeeperTestSuite struct {
	suite.Suite

	ctx             sdk.Context
	bankKeeper      *mocks.BankKeeper
	accKeeper       *mocks.AccountKeeper
	stakingKeeper   *mocks.StakingKeeper
	fmKeeper        *mocks.FeeMarketKeeper
	erc20Keeper     *mocks.Erc20Keeper
	vmKeeper        *vmkeeper.Keeper
	consensusKeeper *mocks.ConsensusParamsKeeper
}

func TestKeeperTestSuite(t *testing.T) {
	suite.Run(t, new(KeeperTestSuite))
}

func (suite *KeeperTestSuite) SetupTest() {
	key := storetypes.NewKVStoreKey(vmtypes.StoreKey)
	oKey := storetypes.NewObjectStoreKey(vmtypes.ObjectKey)
	allKeys := []storetypes.StoreKey{key, oKey}
	testCtx := testutil.DefaultContextWithObjectStore(suite.T(), key,
		storetypes.NewTransientStoreKey("store_test"), oKey)
	ctx := testCtx.Ctx.WithBlockHeader(cmtproto.Header{Time: cmttime.Now()})
	encCfg := moduletestutil.MakeTestEncodingConfig()

	// storeService := runtime.NewKVStoreService(key)
	authority := sdk.AccAddress("foobar")

	suite.bankKeeper = mocks.NewBankKeeper(suite.T())
	suite.accKeeper = mocks.NewAccountKeeper(suite.T())
	suite.stakingKeeper = mocks.NewStakingKeeper(suite.T())
	suite.fmKeeper = mocks.NewFeeMarketKeeper(suite.T())
	suite.erc20Keeper = mocks.NewErc20Keeper(suite.T())
	suite.consensusKeeper = mocks.NewConsensusParamsKeeper(suite.T())
	suite.ctx = ctx

	suite.accKeeper.On("GetModuleAddress", vmtypes.ModuleName).Return(sdk.AccAddress("evm"))
	suite.vmKeeper = vmkeeper.NewKeeper(
		encCfg.Codec,
		key,
		oKey,
		allKeys,
		authority,
		suite.accKeeper,
		suite.bankKeeper,
		suite.stakingKeeper,
		suite.fmKeeper,
		suite.consensusKeeper,
		suite.erc20Keeper,
		constants.EighteenDecimalsChainID,
		"",
	)
}

func (suite *KeeperTestSuite) TestAddPreinstalls() {
	testCases := []struct {
		name        string
		malleate    func()
		preinstalls []vmtypes.Preinstall
		err         error
	}{
		{
			"Default pass",
			func() {
				suite.accKeeper.On("GetAccount", mock.Anything, mock.Anything).Return(nil)
				suite.accKeeper.On("NewAccountWithAddress", mock.Anything,
					mock.Anything).Return(authtypes.NewBaseAccountWithAddress(sdk.AccAddress("evm")), nil)
				suite.accKeeper.On("SetAccount", mock.Anything, mock.Anything).Return()
			},
			vmtypes.DefaultPreinstalls,
			nil,
		},
		{
			"Acc already exists -- expect error",
			func() {
				suite.accKeeper.ExpectedCalls = suite.accKeeper.ExpectedCalls[:0]
				suite.accKeeper.On("GetAccount", mock.Anything, mock.Anything).Return(authtypes.NewBaseAccountWithAddress(sdk.AccAddress("evm")))
			},
			vmtypes.DefaultPreinstalls,
			vmtypes.ErrInvalidPreinstall,
		},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tc.malleate()
			err := suite.vmKeeper.AddPreinstalls(suite.ctx, vmtypes.DefaultPreinstalls)
			if tc.err != nil {
				suite.Require().ErrorContains(err, tc.err.Error())
			} else {
				suite.Require().NoError(err)
			}
		})
	}
}
func (suite *KeeperTestSuite) TestEnableEIPs() {
	// Use valid EIPs from the error message: [1153 1344 1884 2200 2929 3198 3529 3855 3860 4762 5656 6780 7702 7939]
	testCases := []struct {
		name           string
		initialEIPs    []int64
		eipsToAdd      []int64
		expectedEIPs   []int64
		expectError    bool
	}{
		{
			name:         "add EIPs to empty list",
			initialEIPs:  []int64{},
			eipsToAdd:    []int64{2929, 1884, 1344},
			expectedEIPs: []int64{1344, 1884, 2929}, // should be sorted
			expectError:  false,
		},
		{
			name:         "add EIPs to existing list",
			initialEIPs:  []int64{3198, 1153},
			eipsToAdd:    []int64{2929, 1884},
			expectedEIPs: []int64{1153, 1884, 2929, 3198}, // should be sorted
			expectError:  false,
		},
		{
			name:         "add single EIP",
			initialEIPs:  []int64{3198},
			eipsToAdd:    []int64{2929},
			expectedEIPs: []int64{2929, 3198}, // should be sorted
			expectError:  false,
		},
		{
			name:         "add duplicate EIPs should fail",
			initialEIPs:  []int64{3198},
			eipsToAdd:    []int64{3198, 2929},
			expectedEIPs: []int64{3198}, // should remain unchanged due to error
			expectError:  true,
		},
		{
			name:         "add EIPs in reverse order",
			initialEIPs:  []int64{},
			eipsToAdd:    []int64{7939, 3529, 1153},
			expectedEIPs: []int64{1153, 3529, 7939}, // should be sorted
			expectError:  false,
		},
		{
			name:         "add no EIPs",
			initialEIPs:  []int64{3198},
			eipsToAdd:    []int64{},
			expectedEIPs: []int64{3198}, // should remain unchanged
			expectError:  false,
		},
		{
			name:         "add invalid EIP should fail",
			initialEIPs:  []int64{3198},
			eipsToAdd:    []int64{1559}, // 1559 is not in the valid list
			expectedEIPs: []int64{3198}, // should remain unchanged
			expectError:  true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			// Set initial parameters with initial EIPs
			initialParams := vmtypes.DefaultParams()
			initialParams.ExtraEIPs = tc.initialEIPs
			err := suite.vmKeeper.SetParams(suite.ctx, initialParams)
			suite.Require().NoError(err)

			// Call EnableEIPs
			err = suite.vmKeeper.EnableEIPs(suite.ctx, tc.eipsToAdd...)

			if tc.expectError {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)

				// Verify the EIPs were added and sorted correctly
				params := suite.vmKeeper.GetParams(suite.ctx)
				suite.Require().Equal(tc.expectedEIPs, params.ExtraEIPs, "EIPs should match expected values and be sorted")
			}
		})
	}
}