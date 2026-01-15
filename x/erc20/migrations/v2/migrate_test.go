package v2_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/cometbft/cometbft/crypto/tmhash"

	"github.com/cosmos/evm/precompiles/bank"
	v2 "github.com/cosmos/evm/x/erc20/migrations/v2"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const testFactoryDenom = "testFactoryDenom"

type mockEVMKeeper struct {
	callCount    int
	shouldFail   bool
	lastCallData []byte
}

func (m *mockEVMKeeper) CallEVMWithData(
	_ sdk.Context,
	_ common.Address,
	_ *common.Address,
	data []byte,
	_ bool,
	_ *big.Int,
) (*evmtypes.MsgEthereumTxResponse, error) {
	m.callCount++
	m.lastCallData = data
	if m.shouldFail {
		return &evmtypes.MsgEthereumTxResponse{VmError: "deployment failed"}, nil
	}
	return &evmtypes.MsgEthereumTxResponse{}, nil
}

type mockERC20Keeper struct {
	dynamicPrecompiles []string
	tokenPairs         map[string]erc20types.TokenPair
	erc20Map           map[common.Address][]byte
	denomMap           map[string][]byte

	deletedPrecompiles  []common.Address
	deletedERC20Maps    []common.Address
	deletedDenomMaps    []string
	deletedTokenPairIDs [][]byte
	setTokenPairs       []erc20types.TokenPair
	setERC20Maps        map[common.Address][]byte
	setDenomMaps        map[string][]byte
}

func newMockERC20Keeper() *mockERC20Keeper {
	return &mockERC20Keeper{
		dynamicPrecompiles:  []string{},
		tokenPairs:          make(map[string]erc20types.TokenPair),
		erc20Map:            make(map[common.Address][]byte),
		denomMap:            make(map[string][]byte),
		deletedPrecompiles:  []common.Address{},
		deletedERC20Maps:    []common.Address{},
		deletedDenomMaps:    []string{},
		deletedTokenPairIDs: [][]byte{},
		setTokenPairs:       []erc20types.TokenPair{},
		setERC20Maps:        make(map[common.Address][]byte),
		setDenomMaps:        make(map[string][]byte),
	}
}

func (m *mockERC20Keeper) GetDynamicPrecompiles(_ sdk.Context) []string {
	return m.dynamicPrecompiles
}

func (m *mockERC20Keeper) DeleteDynamicPrecompile(_ sdk.Context, precompile common.Address) {
	m.deletedPrecompiles = append(m.deletedPrecompiles, precompile)
}

func (m *mockERC20Keeper) GetTokenPairID(_ sdk.Context, token string) []byte {
	if common.IsHexAddress(token) {
		addr := common.HexToAddress(token)
		return m.erc20Map[addr]
	}
	return m.denomMap[token]
}

func (m *mockERC20Keeper) GetTokenPair(_ sdk.Context, id []byte) (erc20types.TokenPair, bool) {
	pair, found := m.tokenPairs[common.Bytes2Hex(id)]
	return pair, found
}

func (m *mockERC20Keeper) SetTokenPair(_ sdk.Context, tokenPair erc20types.TokenPair) {
	m.setTokenPairs = append(m.setTokenPairs, tokenPair)
	m.tokenPairs[common.Bytes2Hex(tokenPair.GetID())] = tokenPair
}

func (m *mockERC20Keeper) SetERC20Map(_ sdk.Context, erc20 common.Address, id []byte) {
	m.setERC20Maps[erc20] = id
	m.erc20Map[erc20] = id
}

func (m *mockERC20Keeper) DeleteERC20Map(_ sdk.Context, erc20 common.Address) {
	m.deletedERC20Maps = append(m.deletedERC20Maps, erc20)
	delete(m.erc20Map, erc20)
}

func (m *mockERC20Keeper) SetDenomMap(_ sdk.Context, denom string, id []byte) {
	m.setDenomMaps[denom] = id
	m.denomMap[denom] = id
}

func (m *mockERC20Keeper) DeleteTokenPairByID(_ sdk.Context, id []byte) {
	m.deletedTokenPairIDs = append(m.deletedTokenPairIDs, id)
	delete(m.tokenPairs, common.Bytes2Hex(id))
}

func (m *mockERC20Keeper) DeleteDenomMap(_ sdk.Context, denom string) {
	m.deletedDenomMaps = append(m.deletedDenomMaps, denom)
	delete(m.denomMap, denom)
}

func (m *mockERC20Keeper) addTokenPair(pair erc20types.TokenPair) {
	pairID := pair.GetID()
	m.tokenPairs[common.Bytes2Hex(pairID)] = pair
	m.erc20Map[pair.GetERC20Contract()] = pairID
	m.denomMap[pair.Denom] = pairID
	m.dynamicPrecompiles = append(m.dynamicPrecompiles, pair.Erc20Address)
}

type MigrateTestSuite struct {
	suite.Suite
	ctx         sdk.Context
	evmKeeper   *mockEVMKeeper
	erc20Keeper *mockERC20Keeper
	deployer    common.Address
}

func TestMigrateSuite(t *testing.T) {
	suite.Run(t, new(MigrateTestSuite))
}

func (suite *MigrateTestSuite) SetupTest() {
	storeKey := storetypes.NewKVStoreKey("test")
	testCtx := testutil.DefaultContextWithDB(suite.T(), storeKey, storetypes.NewTransientStoreKey("transient_test"))
	suite.ctx = testCtx.Ctx
	suite.evmKeeper = &mockEVMKeeper{}
	suite.erc20Keeper = newMockERC20Keeper()
	suite.deployer = common.HexToAddress("0x1234567890123456789012345678901234567890")
}

func (suite *MigrateTestSuite) TestMigrateDynamicPrecompilesToERC20() {
	testCases := []struct {
		name                  string
		setupFunc             func()
		deploymentShouldFail  bool
		expectError           bool
		errorMsg              string
		expectedDeployCount   int
		expectedDeletedCount  int
		expectedSetPairsCount int
	}{
		{
			name:                  "no dynamic precompiles",
			setupFunc:             func() {},
			deploymentShouldFail:  false,
			expectError:           false,
			expectedDeployCount:   0,
			expectedDeletedCount:  0,
			expectedSetPairsCount: 0,
		},
		{
			name: "no token pair found for precompile",
			setupFunc: func() {
				suite.erc20Keeper.dynamicPrecompiles = []string{"0xABCDEF1234567890ABCDEF1234567890ABCDEF12"}
			},
			deploymentShouldFail:  false,
			expectError:           false,
			expectedDeployCount:   0,
			expectedDeletedCount:  0,
			expectedSetPairsCount: 0,
		},
		{
			name: "deployment failure",
			setupFunc: func() {
				pair := erc20types.TokenPair{
					Erc20Address:  "0xABCDEF1234567890ABCDEF1234567890ABCDEF12",
					Denom:         "testFactoryDenom",
					Enabled:       true,
					ContractOwner: erc20types.OWNER_MODULE,
				}
				suite.erc20Keeper.addTokenPair(pair)
			},
			deploymentShouldFail:  true,
			expectError:           true,
			errorMsg:              "ERC20 deployment failed",
			expectedDeployCount:   1,
			expectedDeletedCount:  0,
			expectedSetPairsCount: 0,
		},
		{
			name: "single precompile migration",
			setupFunc: func() {
				pair := erc20types.TokenPair{
					Erc20Address:  "0xABCDEF1234567890ABCDEF1234567890ABCDEF12",
					Denom:         testFactoryDenom,
					Enabled:       true,
					ContractOwner: erc20types.OWNER_MODULE,
				}
				suite.erc20Keeper.addTokenPair(pair)
			},
			deploymentShouldFail:  false,
			expectError:           false,
			expectedDeployCount:   1,
			expectedDeletedCount:  1,
			expectedSetPairsCount: 1,
		},
		{
			name: "multiple precompiles migration",
			setupFunc: func() {
				denoms := []string{
					"factory/mantra1abc/token1",
					"factory/mantra1def/token2",
					"factory/mantra1ghi/token3",
				}
				addrs := []string{
					"0x1111111111111111111111111111111111111111",
					"0x2222222222222222222222222222222222222222",
					"0x3333333333333333333333333333333333333333",
				}
				for i, denom := range denoms {
					pair := erc20types.TokenPair{
						Erc20Address:  addrs[i],
						Denom:         denom,
						Enabled:       true,
						ContractOwner: erc20types.OWNER_MODULE,
					}
					suite.erc20Keeper.addTokenPair(pair)
				}
			},
			deploymentShouldFail:  false,
			expectError:           false,
			expectedDeployCount:   3,
			expectedDeletedCount:  3,
			expectedSetPairsCount: 3,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.evmKeeper = &mockEVMKeeper{shouldFail: tc.deploymentShouldFail}
			suite.erc20Keeper = newMockERC20Keeper()

			tc.setupFunc()

			err := v2.MigrateDynamicPrecompilesToERC20(suite.ctx, suite.evmKeeper, suite.erc20Keeper, suite.deployer)

			if tc.expectError {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.errorMsg)
			} else {
				suite.Require().NoError(err)
			}

			suite.Require().Equal(tc.expectedDeployCount, suite.evmKeeper.callCount)
			suite.Require().Len(suite.erc20Keeper.deletedPrecompiles, tc.expectedDeletedCount)
			suite.Require().Len(suite.erc20Keeper.setTokenPairs, tc.expectedSetPairsCount)
		})
	}
}

func (suite *MigrateTestSuite) TestMigrateDynamicPrecompilesToERC20_VerifyMappings() {
	oldAddr := common.HexToAddress("0xABCDEF1234567890ABCDEF1234567890ABCDEF12")
	pair := erc20types.TokenPair{
		Erc20Address:  oldAddr.Hex(),
		Denom:         testFactoryDenom,
		Enabled:       true,
		ContractOwner: erc20types.OWNER_MODULE,
	}
	suite.erc20Keeper.addTokenPair(pair)

	oldPairID := pair.GetID()

	err := v2.MigrateDynamicPrecompilesToERC20(suite.ctx, suite.evmKeeper, suite.erc20Keeper, suite.deployer)
	suite.Require().NoError(err)

	suite.Require().Equal(oldPairID, suite.erc20Keeper.deletedTokenPairIDs[0])
	suite.Require().Equal(oldAddr, suite.erc20Keeper.deletedERC20Maps[0])
	suite.Require().Equal(testFactoryDenom, suite.erc20Keeper.deletedDenomMaps[0])
	suite.Require().Equal(oldAddr, suite.erc20Keeper.deletedPrecompiles[0])

	newPair := suite.erc20Keeper.setTokenPairs[0]
	suite.Require().Equal(testFactoryDenom, newPair.Denom)
	suite.Require().NotEqual(oldAddr.Hex(), newPair.Erc20Address)

	expectedAddr, err := bank.ERC20ContractAddress(v2.BankPrecompileAddress, testFactoryDenom)
	suite.Require().NoError(err)
	suite.Require().Equal(expectedAddr.Hex(), newPair.Erc20Address)

	newPairID := newPair.GetID()
	suite.Require().NotEqual(oldPairID, newPairID)
	suite.Require().Equal(newPairID, suite.erc20Keeper.setERC20Maps[expectedAddr])
	suite.Require().Equal(newPairID, suite.erc20Keeper.setDenomMaps[testFactoryDenom])
}

func (suite *MigrateTestSuite) TestMigrateDynamicPrecompilesToERC20_PairIDChanges() {
	oldAddr := common.HexToAddress("0xABCDEF1234567890ABCDEF1234567890ABCDEF12")
	pair := erc20types.TokenPair{
		Erc20Address:  oldAddr.Hex(),
		Denom:         testFactoryDenom,
		Enabled:       true,
		ContractOwner: erc20types.OWNER_MODULE,
	}
	suite.erc20Keeper.addTokenPair(pair)

	oldPairID := tmhash.Sum([]byte(oldAddr.Hex() + "|" + testFactoryDenom))

	expectedNewAddr, err := bank.ERC20ContractAddress(v2.BankPrecompileAddress, testFactoryDenom)
	suite.Require().NoError(err)
	expectedNewPairID := tmhash.Sum([]byte(expectedNewAddr.Hex() + "|" + testFactoryDenom))

	err = v2.MigrateDynamicPrecompilesToERC20(suite.ctx, suite.evmKeeper, suite.erc20Keeper, suite.deployer)
	suite.Require().NoError(err)

	suite.Require().Equal(oldPairID, suite.erc20Keeper.deletedTokenPairIDs[0])

	suite.Require().Equal(expectedNewPairID, suite.erc20Keeper.setTokenPairs[0].GetID())
	suite.Require().Equal(expectedNewPairID, suite.erc20Keeper.setERC20Maps[expectedNewAddr])
	suite.Require().Equal(expectedNewPairID, suite.erc20Keeper.setDenomMaps[testFactoryDenom])

	pairID := suite.erc20Keeper.GetTokenPairID(suite.ctx, testFactoryDenom)
	suite.Require().Equal(expectedNewPairID, pairID)

	migratedPair, found := suite.erc20Keeper.GetTokenPair(suite.ctx, pairID)
	suite.Require().True(found)
	suite.Require().Equal(expectedNewAddr.Hex(), migratedPair.Erc20Address)
}

func (suite *MigrateTestSuite) TestDeployCanonicalERC20() {
	testCases := []struct {
		name        string
		shouldFail  bool
		expectError bool
		errorMsg    string
	}{
		{
			name:        "success",
			shouldFail:  false,
			expectError: false,
		},
		{
			name:        "deployment failure",
			shouldFail:  true,
			expectError: true,
			errorMsg:    "ERC20 deployment failed",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.evmKeeper = &mockEVMKeeper{shouldFail: tc.shouldFail}

			addr, err := v2.DeployCanonicalERC20(suite.ctx, suite.evmKeeper, suite.deployer, testFactoryDenom)

			if tc.expectError {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.errorMsg)
				suite.Require().Equal(common.Address{}, addr)
			} else {
				suite.Require().NoError(err)
				suite.Require().Equal(1, suite.evmKeeper.callCount)

				expectedAddr, err := bank.ERC20ContractAddress(v2.BankPrecompileAddress, testFactoryDenom)
				suite.Require().NoError(err)
				suite.Require().Equal(expectedAddr, addr)
			}
		})
	}
}
