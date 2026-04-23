package ibc

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/cosmos/evm/testutil"

	"github.com/cosmos/evm/contracts"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/precompiles/ics20"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
)

// Test constants for sequential ICS20 sends
const (
	SeqICS20InitialTokenAmount = 1_000_000_000_000_000_000 // 1 token with 18 decimals
	SeqICS20SenderIndex        = 1
	SeqICS20TimeoutHeight      = 110
)

// Test suite for ICS20 sequential max sends
type ICS20SequentialPrecompileCallsTestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator

	chainA           *evmibctesting.TestChain
	chainAPrecompile *ics20.Precompile
	chainB           *evmibctesting.TestChain
	chainBPrecompile *ics20.Precompile
}

func (suite *ICS20SequentialPrecompileCallsTestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 2, 0, integration.SetupEvmd)
	suite.chainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(2))

	evmAppA := suite.chainA.App.(*evmd.EVMD)
	suite.chainAPrecompile = ics20.NewPrecompile(
		evmAppA.BankKeeper,
		*evmAppA.StakingKeeper,
		evmAppA.TransferKeeper,
		evmAppA.IBCKeeper.ChannelKeeper,
		evmAppA.Erc20Keeper,
	)
	bondDenom, err := evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
	suite.Require().NoError(err)

	evmAppA.Erc20Keeper.GetTokenPair(suite.chainA.GetContext(), evmAppA.Erc20Keeper.GetTokenPairID(suite.chainA.GetContext(), bondDenom))

	avail := evmAppA.Erc20Keeper.IsNativePrecompileAvailable(suite.chainA.GetContext(), common.HexToAddress("0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"))
	suite.Require().True(avail)

	evmAppB := suite.chainB.App.(*evmd.EVMD)
	suite.chainBPrecompile = ics20.NewPrecompile(
		evmAppB.BankKeeper,
		*evmAppB.StakingKeeper,
		evmAppB.TransferKeeper,
		evmAppB.IBCKeeper.ChannelKeeper,
		evmAppB.Erc20Keeper,
	)
}

// TestReceiveAndSendTwice tests the production flow: transfer tokens in -> ICS20 sends -> transfer out
// This mirrors the actual transaction pattern seen on-chain.
func (suite *ICS20SequentialPrecompileCallsTestSuite) TestReceiveAndSendTwice() {
	suite.SetupTest()

	pathAToB := evmibctesting.NewTransferPath(suite.chainA, suite.chainB)
	pathAToB.Setup()

	evmAppA := suite.chainA.App.(*evmd.EVMD)
	// Deployer has MINTER_ROLE - use it for minting
	deployerAddr := common.BytesToAddress(suite.chainA.SenderPrivKey.PubKey().Address().Bytes())
	// Use a different account for the actual test tx
	senderAccount := suite.chainA.SenderAccounts[SeqICS20SenderIndex]
	senderAddr := common.BytesToAddress(senderAccount.SenderAccount.GetAddress().Bytes())

	// 1. Deploy ERC20 contract
	erc20ContractData := contracts.ERC20MinterBurnerDecimalsContract
	erc20DeploymentData := testutiltypes.ContractDeploymentData{
		Contract:        erc20ContractData,
		ConstructorArgs: []interface{}{"TestToken", "TT", uint8(18)},
	}
	erc20Addr, err := DeployContract(suite.T(), suite.chainA, erc20DeploymentData)
	suite.chainA.NextBlock()
	suite.Require().NoError(err)
	fmt.Printf("ERC20 contract deployed at: %s\n", erc20Addr.Hex())

	// 2. Register the ERC20
	ctx := suite.chainA.GetContext()
	_, err = evmAppA.Erc20Keeper.RegisterERC20(ctx, &erc20types.MsgRegisterERC20{
		Signer:         evmAppA.AccountKeeper.GetModuleAddress("gov").String(),
		Erc20Addresses: []string{erc20Addr.Hex()},
	})
	suite.Require().NoError(err)
	suite.chainA.NextBlock()

	// 3. Deploy the Sender contract
	senderContractData, err := contracts.LoadSequentialICS20Sender()
	suite.Require().NoError(err)
	senderDeploymentData := testutiltypes.ContractDeploymentData{
		Contract:        senderContractData,
		ConstructorArgs: []interface{}{},
	}
	contractAddr, err := DeployContract(suite.T(), suite.chainA, senderDeploymentData)
	suite.chainA.NextBlock()
	suite.Require().NoError(err)
	fmt.Printf("Sender contract deployed at: %s\n", contractAddr.Hex())

	// 4. Mint ERC20 tokens to the test sender using deployer (who has MINTER_ROLE)
	ctx = suite.chainA.GetContext()
	mintStateDB := testutil.NewStateDB(ctx, evmAppA.EVMKeeper)
	_, err = evmAppA.GetEVMKeeper().CallEVM(ctx, mintStateDB, erc20ContractData.ABI, deployerAddr, erc20Addr, true, false, nil, "mint", senderAddr, big.NewInt(SeqICS20InitialTokenAmount))
	suite.Require().NoError(err)
	suite.chainA.NextBlock()

	// Verify minted balance
	senderBal := evmAppA.GetErc20Keeper().BalanceOf(suite.chainA.GetContext(), erc20ContractData.ABI, erc20Addr, senderAddr)
	suite.Require().Equal(big.NewInt(SeqICS20InitialTokenAmount), senderBal)
	fmt.Printf("Sender balance before: %s\n", senderBal.String())

	// 5. Approve the contract to spend tokens (called by sender)
	ctx = suite.chainA.GetContext()
	approveStateDB := testutil.NewStateDB(ctx, evmAppA.EVMKeeper)
	_, err = evmAppA.GetEVMKeeper().CallEVM(ctx, approveStateDB, erc20ContractData.ABI, senderAddr, erc20Addr, true, false, nil, "approve", contractAddr, big.NewInt(SeqICS20InitialTokenAmount*2))
	suite.Require().NoError(err)
	suite.chainA.NextBlock()

	// Get balances before
	senderBalBefore := evmAppA.GetErc20Keeper().BalanceOf(suite.chainA.GetContext(), erc20ContractData.ABI, erc20Addr, senderAddr)
	contractBalBefore := evmAppA.GetErc20Keeper().BalanceOf(suite.chainA.GetContext(), erc20ContractData.ABI, erc20Addr, contractAddr)
	fmt.Printf("Before tx - Sender: %s, Contract: %s\n", senderBalBefore.String(), contractBalBefore.String())

	// 6. Call receiveAndSendTwice - this should:
	//    - Transfer tokens from sender to contract
	//    - Attempt first ICS20 send (should succeed)
	//    - Attempt second ICS20 send (should fail - no balance)
	//    - Revert and return tokens
	denom := "erc20:" + erc20Addr.Hex()
	data, err := senderContractData.ABI.Pack(
		"receiveAndSendTwice",
		erc20Addr,
		pathAToB.EndpointA.ChannelConfig.PortID,
		pathAToB.EndpointA.ChannelID,
		denom,
		suite.chainB.SenderAccount.GetAddress().String(),
		uint64(SeqICS20TimeoutHeight),
		big.NewInt(SeqICS20InitialTokenAmount),
	)
	suite.Require().NoError(err)

	res, _, _, err := suite.chainA.SendEvmTx(senderAccount, SeqICS20SenderIndex, contractAddr, big.NewInt(0), data, 0)

	// Log events
	if res != nil {
		fmt.Printf("Total events: %d\n", len(res.Events))
		for i, event := range res.Events {
			fmt.Printf("Event %d: type=%s\n", i, event.Type)
			for _, attr := range event.Attributes {
				fmt.Printf("  %s: %s\n", attr.Key, attr.Value)
			}
		}
	}

	// Check for revert
	hasRevertEvent := false
	if res != nil {
		for _, event := range res.Events {
			if event.Type == "ethereum_tx" {
				for _, attr := range event.Attributes {
					if attr.Key == "ethereumTxFailed" {
						hasRevertEvent = true
						fmt.Printf("Found revert: %s\n", attr.Value)
					}
				}
			}
		}
	}

	// Get balances after
	senderBalAfter := evmAppA.GetErc20Keeper().BalanceOf(suite.chainA.GetContext(), erc20ContractData.ABI, erc20Addr, senderAddr)
	contractBalAfter := evmAppA.GetErc20Keeper().BalanceOf(suite.chainA.GetContext(), erc20ContractData.ABI, erc20Addr, contractAddr)
	fmt.Printf("After tx - Sender: %s, Contract: %s\n", senderBalAfter.String(), contractBalAfter.String())

	// The second ICS20 send should have failed, causing a revert
	// Sender balance should be unchanged (tokens returned on revert)
	suite.Require().True(hasRevertEvent || err != nil, "expected transaction to revert on second ICS20 send")
	suite.Require().Equal(senderBalBefore.String(), senderBalAfter.String(), "sender balance should be unchanged after revert")
	suite.Require().Equal(contractBalBefore.String(), contractBalAfter.String(), "contract balance should be unchanged after revert")
}

func TestICS20SequentialPrecompileCallsTestSuite(t *testing.T) {
	suite.Run(t, new(ICS20SequentialPrecompileCallsTestSuite))
}
