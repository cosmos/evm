package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttime "github.com/cometbft/cometbft/types/time"

	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	vmkeeper "github.com/cosmos/evm/x/vm/keeper"
	vmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/types/mocks"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"

	storetypes "cosmossdk.io/store/types"
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	upgradetypes "cosmossdk.io/x/upgrade/types"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	authzkeeper "github.com/cosmos/cosmos-sdk/x/authz/keeper"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	consensusparamtypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
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
	keys := storetypes.NewKVStoreKeys(
		authtypes.StoreKey, banktypes.StoreKey, stakingtypes.StoreKey,
		minttypes.StoreKey, distrtypes.StoreKey, slashingtypes.StoreKey,
		govtypes.StoreKey, paramstypes.StoreKey, consensusparamtypes.StoreKey,
		upgradetypes.StoreKey, feegrant.StoreKey, evidencetypes.StoreKey, authzkeeper.StoreKey,
		// ibc keys
		ibcexported.StoreKey, ibctransfertypes.StoreKey,
		// Cosmos EVM store keys
		vmtypes.StoreKey, feemarkettypes.StoreKey, erc20types.StoreKey, precisebanktypes.StoreKey,
	)
	key := storetypes.NewKVStoreKey(vmtypes.StoreKey)
	transientKey := storetypes.NewTransientStoreKey(vmtypes.TransientKey)
	testCtx := testutil.DefaultContextWithDB(suite.T(), key, storetypes.NewTransientStoreKey("transient_test"))
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
		transientKey,
		keys,
		authority,
		suite.accKeeper,
		suite.bankKeeper,
		suite.stakingKeeper,
		suite.fmKeeper,
		suite.consensusKeeper,
		suite.erc20Keeper,
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
