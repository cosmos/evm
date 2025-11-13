package gov

import (
	"time"

	"github.com/stretchr/testify/suite"

	evmaddress "github.com/cosmos/evm/encoding/address"
	"github.com/cosmos/evm/precompiles/gov"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

type PrecompileTestSuite struct {
	suite.Suite

	create      network.CreateEvmApp
	options     []network.ConfigOption
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     testkeyring.Keyring

	precompile *gov.Precompile
}

func NewPrecompileTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *PrecompileTestSuite {
	return &PrecompileTestSuite{
		create:  create,
		options: options,
	}
}

func (s *PrecompileTestSuite) SetupTest() {
	keyring := testkeyring.New(3)

	// seed the db with one proposal
	customGen := network.CustomGenesisState{}
	now := time.Now().UTC()
	inOneHour := now.Add(time.Hour)

	var err error
	anyMessage, err := types.NewAnyWithValue(TestProposalMsgs[0])
	if err != nil {
		panic(err)
	}
	prop := &govv1.Proposal{
		Id:              1,
		Status:          govv1.ProposalStatus_PROPOSAL_STATUS_VOTING_PERIOD,
		SubmitTime:      &now,
		DepositEndTime:  &inOneHour,
		VotingStartTime: &now,
		FinalTallyResult: &govv1.TallyResult{
			YesCount:        "0",
			AbstainCount:    "0",
			NoCount:         "0",
			NoWithVetoCount: "0",
		},
		VotingEndTime: &inOneHour,
		Metadata:      "ipfs://CID",
		Title:         "test prop",
		Summary:       "test prop",
		Proposer:      keyring.GetAccAddr(0).String(),
		Messages:      []*types.Any{anyMessage},
	}

	prop2 := &govv1.Proposal{
		Id:              2,
		Status:          govv1.ProposalStatus_PROPOSAL_STATUS_VOTING_PERIOD,
		SubmitTime:      &now,
		DepositEndTime:  &inOneHour,
		VotingStartTime: &now,
		FinalTallyResult: &govv1.TallyResult{
			YesCount:        "0",
			AbstainCount:    "0",
			NoCount:         "0",
			NoWithVetoCount: "0",
		},
		VotingEndTime: &inOneHour,
		Metadata:      "ipfs://CID",
		Title:         "test prop",
		Summary:       "test prop",
		Proposer:      keyring.GetAccAddr(1).String(),
		Messages:      []*types.Any{anyMessage},
	}

	bankGen := banktypes.DefaultGenesisState()
	bankGen.Balances = []banktypes.Balance{{
		Address: authtypes.NewModuleAddress(govtypes.ModuleName).String(),
		Coins:   sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(200))),
	}}
	govGen := govv1.DefaultGenesisState()
	govGen.StartingProposalId = 3
	govGen.Deposits = []*govv1.Deposit{
		{
			ProposalId: 1,
			Depositor:  keyring.GetAccAddr(0).String(),
			Amount:     sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(100))),
		},
		{
			ProposalId: 2,
			Depositor:  keyring.GetAccAddr(1).String(),
			Amount:     sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(100))),
		},
	}
	govGen.Params.MinDeposit = sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(100)))
	govGen.Params.ProposalCancelDest = keyring.GetAccAddr(2).String()
	govGen.Proposals = append(govGen.Proposals, prop)
	govGen.Proposals = append(govGen.Proposals, prop2)
	customGen[govtypes.ModuleName] = govGen
	customGen[banktypes.ModuleName] = bankGen

	options := []network.ConfigOption{
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		network.WithCustomGenesis(customGen),
	}
	options = append(options, s.options...)
	nw := network.NewUnitTestNetwork(s.create, options...)
	grpcHandler := grpc.NewIntegrationHandler(nw)
	txFactory := factory.New(nw, grpcHandler)

	s.factory = txFactory
	s.grpcHandler = grpcHandler
	s.keyring = keyring
	s.network = nw

	govKeeper := s.network.App.GetGovKeeper()
	s.precompile = gov.NewPrecompile(
		govkeeper.NewMsgServerImpl(&govKeeper),
		govkeeper.NewQueryServer(&govKeeper),
		s.network.App.GetBankKeeper(),
		s.network.App.AppCodec(),
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix()),
	)
}
