package erc20_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/cometbft/cometbft/crypto/tmhash"
	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"
	tmversion "github.com/cometbft/cometbft/proto/tendermint/version"
	"github.com/cometbft/cometbft/version"

	exampleapp "github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/os/network"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20"
	"github.com/cosmos/evm/x/erc20/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type GenesisTestSuite struct {
	suite.Suite
	ctx     sdk.Context
	app     *exampleapp.EVMD
	genesis types.GenesisState
}

const osmoERC20ContractAddr = "0x5D87876250185593977a6F94aF98877a5E7eD60E"

var osmoDenom = transfertypes.NewDenom("uosmo", transfertypes.NewHop(transfertypes.PortID, "channel-0"))

func TestGenesisTestSuite(t *testing.T) {
	suite.Run(t, new(GenesisTestSuite))
}

func (suite *GenesisTestSuite) SetupTest() {
	// consensus key
	consAddress := sdk.ConsAddress(utiltx.GenerateAddress().Bytes())

	chainID := constants.ExampleChainID
	suite.app = exampleapp.Setup(suite.T(), chainID.ChainID, chainID.EVMChainID)
	suite.ctx = suite.app.NewContextLegacy(false, tmproto.Header{
		Height:          1,
		ChainID:         chainID.ChainID,
		Time:            time.Now().UTC(),
		ProposerAddress: consAddress.Bytes(),

		Version: tmversion.Consensus{
			Block: version.BlockProtocol,
		},
		LastBlockId: tmproto.BlockID{
			Hash: tmhash.Sum([]byte("block_id")),
			PartSetHeader: tmproto.PartSetHeader{
				Total: 11,
				Hash:  tmhash.Sum([]byte("partset_header")),
			},
		},
		AppHash:            tmhash.Sum([]byte("app")),
		DataHash:           tmhash.Sum([]byte("data")),
		EvidenceHash:       tmhash.Sum([]byte("evidence")),
		ValidatorsHash:     tmhash.Sum([]byte("validators")),
		NextValidatorsHash: tmhash.Sum([]byte("next_validators")),
		ConsensusHash:      tmhash.Sum([]byte("consensus")),
		LastResultsHash:    tmhash.Sum([]byte("last_result")),
	})

	suite.genesis = *types.DefaultGenesisState()
}

func (suite *GenesisTestSuite) TestERC20InitGenesis() {
	testCases := []struct {
		name         string
		genesisState types.GenesisState
	}{
		{
			name:         "empty genesis",
			genesisState: types.GenesisState{},
		},
		{
			name:         "default genesis",
			genesisState: *types.DefaultGenesisState(),
		},
		{
			name: "custom genesis",
			genesisState: types.NewGenesisState(
				types.DefaultParams(),
				[]types.TokenPair{
					{
						Erc20Address:  osmoERC20ContractAddr,
						Denom:         osmoDenom.IBCDenom(),
						Enabled:       true,
						ContractOwner: types.OWNER_MODULE,
					},
				},
				[]types.Allowance{},
			),
		},
		{
			name: "custom genesis with allowances and enabled token pair",
			genesisState: types.NewGenesisState(
				types.DefaultParams(),
				[]types.TokenPair{
					{
						Erc20Address:  osmoERC20ContractAddr,
						Denom:         osmoDenom.IBCDenom(),
						Enabled:       true,
						ContractOwner: types.OWNER_MODULE,
					},
				},
				[]types.Allowance{
					{
						Erc20Address: osmoERC20ContractAddr,
						Owner:        utiltx.GenerateAddress().String(),
						Spender:      utiltx.GenerateAddress().String(),
						Value:        math.NewInt(100),
					},
				},
			),
		},
		{
			name: "custom genesis with allowances and disabled token pair",
			genesisState: types.NewGenesisState(
				types.DefaultParams(),
				[]types.TokenPair{
					{
						Erc20Address:  osmoERC20ContractAddr,
						Denom:         osmoDenom.IBCDenom(),
						Enabled:       false,
						ContractOwner: types.OWNER_MODULE,
					},
				},
				[]types.Allowance{
					{
						Erc20Address: osmoERC20ContractAddr,
						Owner:        utiltx.GenerateAddress().String(),
						Spender:      utiltx.GenerateAddress().String(),
						Value:        math.NewInt(100),
					},
				},
			),
		},
	}

	for _, tc := range testCases {
		gen := network.CustomGenesisState{
			types.ModuleName: &tc.genesisState, // #nosec G601
		}
		nw := network.NewUnitTestNetwork(
			network.WithCustomGenesis(gen),
		)

		params := nw.App.Erc20Keeper.GetParams(nw.GetContext())

		tokenPairs := nw.App.Erc20Keeper.GetTokenPairs(nw.GetContext())
		suite.Require().Equal(tc.genesisState.Params, params)
		if len(tokenPairs) > 0 {
			suite.Require().Equal(tc.genesisState.TokenPairs, tokenPairs, tc.name)
		} else {
			suite.Require().Len(tc.genesisState.TokenPairs, 0, tc.name)
		}

		allowances := nw.App.Erc20Keeper.GetAllowances(nw.GetContext())
		if len(allowances) > 0 {
			suite.Require().Equal(tc.genesisState.Allowances, allowances, tc.name)
		} else {
			suite.Require().Len(tc.genesisState.Allowances, 0, tc.name)
		}
	}
}

func (suite *GenesisTestSuite) TestErc20ExportGenesis() {
	testGenCases := []struct {
		name         string
		genesisState types.GenesisState
	}{
		{
			name:         "empty genesis",
			genesisState: types.GenesisState{},
		},
		{
			name:         "default genesis",
			genesisState: *types.DefaultGenesisState(),
		},
		{
			name: "custom genesis with empty allowance",
			genesisState: types.NewGenesisState(
				types.DefaultParams(),
				[]types.TokenPair{
					{
						Erc20Address:  osmoERC20ContractAddr,
						Denom:         osmoDenom.IBCDenom(),
						Enabled:       true,
						ContractOwner: types.OWNER_MODULE,
					},
				},
				[]types.Allowance{},
			),
		},
		{
			name: "custom genesis with allowances",
			genesisState: types.NewGenesisState(
				types.DefaultParams(),
				[]types.TokenPair{
					{
						Erc20Address:  osmoERC20ContractAddr,
						Denom:         osmoDenom.IBCDenom(),
						Enabled:       true,
						ContractOwner: types.OWNER_MODULE,
					},
				},
				[]types.Allowance{
					{
						Erc20Address: osmoERC20ContractAddr,
						Owner:        utiltx.GenerateAddress().String(),
						Spender:      utiltx.GenerateAddress().String(),
						Value:        math.NewInt(100),
					},
					{
						Erc20Address: osmoERC20ContractAddr,
						Owner:        utiltx.GenerateAddress().String(),
						Spender:      utiltx.GenerateAddress().String(),
						Value:        math.NewInt(200),
					},
				},
			),
		},
	}

	for _, tc := range testGenCases {
		erc20.InitGenesis(suite.ctx, suite.app.Erc20Keeper, suite.app.AccountKeeper, tc.genesisState)
		suite.Require().NotPanics(func() {
			genesisExported := erc20.ExportGenesis(suite.ctx, suite.app.Erc20Keeper)
			params := suite.app.Erc20Keeper.GetParams(suite.ctx)
			suite.Require().Equal(genesisExported.Params, params)

			tokenPairs := suite.app.Erc20Keeper.GetTokenPairs(suite.ctx)
			if len(tokenPairs) > 0 {
				suite.Require().Equal(genesisExported.TokenPairs, tokenPairs)
			} else {
				suite.Require().Len(genesisExported.TokenPairs, 0)
			}
		})
	}
}
