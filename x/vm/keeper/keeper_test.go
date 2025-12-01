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
