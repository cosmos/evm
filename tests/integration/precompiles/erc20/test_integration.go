package erc20

import (
	"math/big"
	"slices"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/suite"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/precompiles/erc20/testdata"
	"github.com/cosmos/evm/precompiles/testutil"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	"github.com/cosmos/evm/testutil/keyring"
	utiltx "github.com/cosmos/evm/testutil/tx"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

var is *IntegrationTestSuite

type IntegrationTestSuite struct {
	suite.Suite

	create  network.CreateEvmApp
	options []network.ConfigOption
	// NOTE: we have to use the Unit testing network because we access a keeper in a setup function.
	// Might adjust this on a follow-up PR.
	network *network.UnitTestNetwork
	handler grpc.Handler
	keyring keyring.Keyring
	factory factory.TxFactory

	bondDenom     string
	tokenDenom    string // erc20 precompile denom with supply
	tokenDenomTwo string // erc20 precompile denom with zero supply

	precompile    *erc20.Precompile // erc20 precompile with supply
	precompileTwo *erc20.Precompile // erc20 precompile with zero supply
}

func NewIntegrationTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *IntegrationTestSuite {
	return &IntegrationTestSuite{
		create:  create,
		options: options,
	}
}

func (is *IntegrationTestSuite) SetupTest() {
	is.tokenDenom = "xmpl"
	is.tokenDenomTwo = "xmpl2"

	keys := keyring.New(2)
	genesis := utils.CreateGenesisWithTokenPairs(keys, is.tokenDenom, is.tokenDenomTwo)

	options := []network.ConfigOption{
		network.WithPreFundedAccounts(keys.GetAllAccAddrs()...),
		network.WithOtherDenoms([]string{is.tokenDenom}), // add balance (supply) to is.tokenDenom
		network.WithCustomGenesis(genesis),
	}
	options = append(options, is.options...)
	nw := network.NewUnitTestNetwork(is.create, options...)
	gh := grpc.NewIntegrationHandler(nw)
	tf := factory.New(nw, gh)

	is.network = nw
	is.factory = tf
	is.handler = gh
	is.keyring = keys

	is.bondDenom = nw.GetBaseDenom()

	erc20Gen := genesis[erc20types.ModuleName].(*erc20types.GenesisState)
	is.precompile = is.setupERC20Precompile(is.tokenDenom, erc20Gen.TokenPairs)
	is.precompileTwo = is.setupERC20Precompile(is.tokenDenomTwo, erc20Gen.TokenPairs)
}

var (
	revertContractAddr common.Address
	gasLimit           = uint64(5000000)
	gasPrice           = big.NewInt(800_000_000)
)

func TestIntegrationTestSuite(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	is = NewIntegrationTestSuite(create, options...)
	_ = Describe("ERC20 Extension -", func() {
		var (
			// contractsData holds the addresses and ABIs for the different
			// contract instances that are subject to testing here.
			contractsData ContractsData

			erc20MdCallerContract evmtypes.CompiledContract
			revertCallerContract  evmtypes.CompiledContract
			erc20MinterV5Contract evmtypes.CompiledContract

			execRevertedCheck testutil.LogCheckArgs
			failCheck         testutil.LogCheckArgs
			passCheck         testutil.LogCheckArgs
		)

		BeforeEach(func() {
			is.SetupTest()

			var err error
			erc20MdCallerContract, err = testdata.LoadERC20TestCaller()
			Expect(err).ToNot(HaveOccurred(), "failed to load ERC20 allowance caller contract")

			erc20MinterV5Contract, err = testdata.LoadERC20MinterV5Contract()
			Expect(err).ToNot(HaveOccurred(), "failed to load ERC20 minter contract")

			revertCallerContract, err = testdata.LoadERC20TestCaller()
			Expect(err).ToNot(HaveOccurred(), "failed to load ERC20 allowance caller contract")

			sender := is.keyring.GetKey(0)
			contractAddr, err := is.factory.DeployContract(
				sender.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: erc20MdCallerContract,
					// NOTE: we're passing the precompile address to the constructor because that initiates the contract
					// to make calls to the correct ERC20 precompile.
					ConstructorArgs: []interface{}{is.precompile.Address()},
				},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to deploy contract")

			// commit the changes to update state (account nonce mostly)
			err = is.network.NextBlock()
			Expect(err).ToNot(HaveOccurred(), "failed to advance block")

			contractAddrTokenTwo, err := is.factory.DeployContract(
				sender.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: erc20MdCallerContract,
					// NOTE: we're passing the precompile address to the constructor because that initiates the contract
					// to make calls to the correct ERC20 precompile.
					ConstructorArgs: []interface{}{is.precompileTwo.Address()},
				},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to deploy contract")

			// commit the changes to update state (account nonce mostly)
			err = is.network.NextBlock()
			Expect(err).ToNot(HaveOccurred(), "failed to advance block")

			erc20MinterBurnerAddr, err := is.factory.DeployContract(
				sender.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: contracts.ERC20MinterBurnerDecimalsContract,
					ConstructorArgs: []interface{}{
						"Xmpl", "Xmpl", uint8(6),
					},
				},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to deploy ERC20 minter burner contract")

			// commit the changes to update state (account nonce mostly)
			err = is.network.NextBlock()
			Expect(err).ToNot(HaveOccurred(), "failed to advance block")

			ERC20MinterV5Addr, err := is.factory.DeployContract(
				sender.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: erc20MinterV5Contract,
					ConstructorArgs: []interface{}{
						"Xmpl", "Xmpl",
					},
				},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to deploy ERC20 minter contract")

			// commit the changes to update state (account nonce mostly)
			err = is.network.NextBlock()
			Expect(err).ToNot(HaveOccurred(), "failed to advance block")

			erc20MinterV5CallerAddr, err := is.factory.DeployContract(
				sender.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: erc20MdCallerContract,
					ConstructorArgs: []interface{}{
						ERC20MinterV5Addr,
					},
				},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to deploy ERC20 minter caller contract")

			// commit the changes to update state (account nonce mostly)
			err = is.network.NextBlock()
			Expect(err).ToNot(HaveOccurred(), "failed to advance block")

			// Store the data of the deployed contracts
			contractsData = ContractsData{
				ownerPriv: sender.Priv,
				contractData: map[CallType]ContractData{
					directCall: {
						Address: is.precompile.Address(),
						ABI:     is.precompile.ABI,
					},
					directCallToken2: {
						Address: is.precompileTwo.Address(),
						ABI:     is.precompileTwo.ABI,
					},
					contractCall: {
						Address: contractAddr,
						ABI:     erc20MdCallerContract.ABI,
					},
					contractCallToken2: {
						Address: contractAddrTokenTwo,
						ABI:     erc20MdCallerContract.ABI,
					},
					erc20Call: {
						Address: erc20MinterBurnerAddr,
						ABI:     contracts.ERC20MinterBurnerDecimalsContract.ABI,
					},
					erc20V5Call: {
						Address: ERC20MinterV5Addr,
						ABI:     erc20MinterV5Contract.ABI,
					},
					erc20V5CallerCall: {
						Address: erc20MinterV5CallerAddr,
						ABI:     erc20MdCallerContract.ABI,
					},
				},
			}

			failCheck = testutil.LogCheckArgs{ABIEvents: is.precompile.Events}
			execRevertedCheck = failCheck.WithErrContains("execution reverted")
			passCheck = failCheck.WithExpPass(true)

			erc20Keeper := is.network.App.GetErc20Keeper()
			available := erc20Keeper.IsNativePrecompileAvailable(is.network.GetContext(), common.HexToAddress(testconstants.WEVMOSContractMainnet))
			Expect(available).To(BeTrue())

			revertContractAddr, err = is.factory.DeployContract(
				sender.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: revertCallerContract,
					// NOTE: we're passing the precompile address to the constructor because that initiates the contract
					// to make calls to the correct ERC20 precompile.
					ConstructorArgs: []interface{}{common.HexToAddress(testconstants.WEVMOSContractMainnet)},
				},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to deploy reverter contract")

			err = is.network.NextBlock()
			Expect(err).ToNot(HaveOccurred(), "failed to advance block")
		})

		Context("basic functionality -", func() {
			When("sending tokens to contract", func() {
				It("it should return error", func() {
					sender := is.keyring.GetKey(0)
					fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 300)}

					// Fund account with some tokens
					is.fundWithTokens(directCall, contractsData, sender.Addr, fundCoins)

					// Taking custom args from the table entry
					txArgs := evmtypes.EvmTxArgs{}
					txArgs.Amount = big.NewInt(int64(1000))
					precompileAddress := is.precompile.Address()
					txArgs.To = &precompileAddress

					_, err := is.factory.ExecuteEthTx(sender.Priv, txArgs)
					// Currently, this check pass because the erc20 precompile does
					// not expose a fallback handler. Adding a fallback handler, the
					// test should pass again because of the check on the message
					// value in the precompile before the setup.
					Expect(err.Error()).To(ContainSubstring(vm.ErrExecutionReverted.Error()), "precompile should not accept transfers")
				},
				)
			})
			When("transferring tokens", func() {
				DescribeTable("it should transfer tokens to a non-existing address", func(callType CallType, expGasUsedLowerBound int64, expGasUsedUpperBound int64) {
					sender := is.keyring.GetKey(0)
					receiver := utiltx.GenerateAddress()
					fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 300)}
					transferCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 100)}

					senderInitialAmt := is.fundWithTokens(callType, contractsData, sender.Addr, fundCoins)
					senderInitialBalance := sdk.Coins{sdk.NewCoin(is.tokenDenom, senderInitialAmt)}

					// Transfer tokens
					txArgs, transferArgs := is.getTxAndCallArgs(callType, contractsData, erc20.TransferMethod, receiver, transferCoins[0].Amount.BigInt())

					transferCheck := passCheck.WithExpEvents(erc20.EventTypeTransfer)

					res, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, transferArgs, transferCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					err = is.network.NextBlock()
					Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

					is.ExpectTrueToBeReturned(ethRes, erc20.TransferMethod)
					is.ExpectBalancesForContract(
						callType, contractsData,
						[]ExpectedBalance{
							{address: sender.AccAddr, expCoins: senderInitialBalance.Sub(transferCoins...)},
							{address: receiver.Bytes(), expCoins: transferCoins},
						},
					)

					Expect(res.GasUsed).To(BeNumerically(">", expGasUsedLowerBound), "expected more gas used")
					Expect(res.GasUsed).To(BeNumerically("<", expGasUsedUpperBound), "expected less gas used")
				},
					// FIXME: The gas used on the precompile is much higher than on the EVM
					Entry(" - direct call", directCall, int64(30_000), int64(31_000)),
					Entry(" - through erc20 contract", erc20Call, int64(53_000), int64(54_500)),
					Entry(" - through erc20 v5 contract", erc20V5Call, int64(52_000), int64(52_200)),
				)

				DescribeTable("it should transfer tokens to an existing address", func(callType CallType) {
					sender := is.keyring.GetKey(0)
					receiver := is.keyring.GetKey(1)
					fundCoinsSender := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 300)}
					fundCoinsReceiver := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 500)}
					transferCoin := sdk.NewInt64Coin(is.tokenDenom, 100)

					// Fund accounts with some tokens
					receiverInitialAmt := is.fundWithTokens(callType, contractsData, receiver.Addr, fundCoinsReceiver)
					receiverInitialBalance := sdk.Coins{sdk.NewCoin(is.tokenDenom, receiverInitialAmt)}

					senderInitialAmt := is.fundWithTokens(callType, contractsData, sender.Addr, fundCoinsSender)
					senderInitialBalance := sdk.Coins{sdk.NewCoin(is.tokenDenom, senderInitialAmt)}

					// Transfer tokens
					txArgs, transferArgs := is.getTxAndCallArgs(callType, contractsData, erc20.TransferMethod, receiver.Addr, transferCoin.Amount.BigInt())

					transferCheck := passCheck.WithExpEvents(erc20.EventTypeTransfer)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, transferArgs, transferCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					err = is.network.NextBlock()
					Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

					is.ExpectTrueToBeReturned(ethRes, erc20.TransferMethod)
					is.ExpectBalancesForContract(
						callType, contractsData,
						[]ExpectedBalance{
							{address: sender.AccAddr, expCoins: senderInitialBalance.Sub(transferCoin)},
							{address: receiver.AccAddr, expCoins: receiverInitialBalance.Add(transferCoin)},
						},
					)
				},
					Entry(" - direct call", directCall),
					// NOTE: we are not passing the contract call here because transferring using a caller contract
					// is only supported through transferFrom method.
					Entry(" - through erc20 contract", erc20Call),
					Entry(" - through erc20 v5 contract", erc20V5Call),
				)

				DescribeTable("it should return an error trying to call from a smart contract", func(callType CallType) {
					sender := is.keyring.GetKey(0)
					receiver := is.keyring.GetAddr(1)
					fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 300)}
					transferAmount := big.NewInt(100)

					// Fund account with some tokens
					is.fundWithTokens(callType, contractsData, sender.Addr, fundCoins)

					// Transfer tokens
					txArgs, transferArgs := is.getTxAndCallArgs(callType, contractsData, erc20.TransferMethod, receiver, transferAmount)

					revertReasonCheck := execRevertedCheck.WithErrNested(erc20.ErrTransferAmountExceedsBalance.Error())

					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, transferArgs, revertReasonCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
					Expect(ethRes).To(BeNil(), "expected empty result")
				},
					// NOTE: we are not passing the direct call here because this test is specific to the contract calls
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
				)

				DescribeTable("it should return an error if the sender does not have enough tokens", func(callType CallType) {
					sender := is.keyring.GetKey(0)
					receiver := is.keyring.GetAddr(1)
					fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 200)}

					// Fund account with some tokens
					senderInitialAmt := is.fundWithTokens(callType, contractsData, sender.Addr, fundCoins)
					transferAmt := new(big.Int).Add(senderInitialAmt.BigInt(), big.NewInt(100))

					// Transfer tokens
					txArgs, transferArgs := is.getTxAndCallArgs(callType, contractsData, erc20.TransferMethod, receiver, transferAmt)

					insufficientBalanceCheck := failCheck.WithErrContains(
						erc20.ErrTransferAmountExceedsBalance.Error(),
					)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, transferArgs, insufficientBalanceCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
					Expect(ethRes).To(BeNil(), "expected empty result")
				},
					Entry(" - direct call", directCall),
					// NOTE: we are not passing the contract call here because this test is for direct calls only

					Entry(" - through erc20 contract", erc20Call),
					// // TODO: The ERC20 V5 contract is raising the ERC-6093 standardized error which we are not as of yet
					// Entry(" - through erc20 v5 contract", erc20V5Call),
				)
			})
			When("calling reverter contract", func() {
				Context("in a direct call to the WEVMOS contract", func() {
					var (
						args   testutiltypes.CallArgs
						txArgs evmtypes.EvmTxArgs
					)
					BeforeEach(func() {
						args = testutiltypes.CallArgs{
							ContractABI: revertCallerContract.ABI,
						}

						txArgs = evmtypes.EvmTxArgs{
							To:       &revertContractAddr,
							GasLimit: gasLimit,
							GasPrice: gasPrice,
						}
					})
					It("should transfer tokens", func() {
						sender := is.keyring.GetKey(0)
						receiver := is.keyring.GetKey(1)
						amountToSend := big.NewInt(100)

						balRes, err := is.handler.GetBalanceFromBank(receiver.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						denomInitialBalance := balRes.Balance
						balRes, err = is.handler.GetBalanceFromBank(sender.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						senderInitialBalance := balRes.Balance

						args.MethodName = "transferWithRevert"
						args.Args = []interface{}{
							receiver.Addr,
							amountToSend,
							false,
							false,
						}
						txArgs.Amount = amountToSend

						transferCheck := passCheck.WithExpEvents(erc20.EventTypeTransfer)
						res, _, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, args, transferCheck)
						Expect(err).To(BeNil())
						Expect(is.network.NextBlock()).To(BeNil())
						fees := math.NewIntFromBigInt(gasPrice).MulRaw(res.GasUsed)

						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "failed to advance block")

						balRes, err = is.handler.GetBalanceFromBank(receiver.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						denomFinalBalance := balRes.Balance
						Expect(denomFinalBalance.Amount).To(Equal(denomInitialBalance.Amount.Add(math.NewInt(amountToSend.Int64()))))

						balRes, err = is.handler.GetBalanceFromBank(revertContractAddr.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						contractBalance := balRes.Balance
						Expect(contractBalance.Amount).To(Equal(math.ZeroInt()))

						balRes, err = is.handler.GetBalanceFromBank(sender.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						senderFinalBalance := balRes.Balance
						denomSpent := fees.Add(math.NewIntFromBigInt(amountToSend))
						Expect(senderFinalBalance.Amount).To(Equal(senderInitialBalance.Amount.Sub(denomSpent)))
					},
					)
					DescribeTable("it should revert token transfer from the WEVMOS contract", func(before bool, after bool) {
						sender := is.keyring.GetKey(0)
						receiver := is.keyring.GetAddr(1)
						amountToSend := big.NewInt(100)
						balRes, err := is.handler.GetBalanceFromBank(receiver.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						denomInitialBalance := balRes.Balance
						balRes, err = is.handler.GetBalanceFromBank(sender.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						senderInitialBalance := balRes.Balance

						args.MethodName = "transferWithRevert"
						args.Args = []interface{}{
							receiver,
							amountToSend,
							before,
							after,
						}
						txArgs.Amount = amountToSend

						revertReasonCheck := execRevertedCheck.WithErrNested("revert here")

						res, _, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, args, revertReasonCheck)
						Expect(err).To(BeNil())
						Expect(is.network.NextBlock()).To(BeNil())

						fees := math.NewIntFromBigInt(gasPrice).MulRaw(res.GasUsed)

						// contract balance should remain unchanged
						balRes, err = is.handler.GetBalanceFromBank(receiver.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						denomFinalBalance := balRes.Balance
						Expect(denomFinalBalance.Amount).To(Equal(denomInitialBalance.Amount))

						balRes, err = is.handler.GetBalanceFromBank(revertContractAddr.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						contractBalance := balRes.Balance
						Expect(contractBalance.Amount).To(Equal(math.ZeroInt()))

						balRes, err = is.handler.GetBalanceFromBank(sender.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						senderFinalBalance := balRes.Balance
						Expect(senderFinalBalance.Amount).To(Equal(senderInitialBalance.Amount.Sub(fees)))
					},
						Entry("revert before", true, false),
						Entry("revert after", false, true),
					)
					It("it should send token transfer and send from WEVMOS contract", func() {
						sender := is.keyring.GetKey(0)
						receiver := is.keyring.GetAddr(1)
						totalToSend := int64(350)
						balRes, err := is.handler.GetBalanceFromBank(receiver.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						denomInitialBalance := balRes.Balance
						balRes, err = is.handler.GetBalanceFromBank(sender.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						senderInitialBalance := balRes.Balance

						args.MethodName = "testTransferAndSend"
						args.Args = []interface{}{
							receiver,
							big.NewInt(100),
							big.NewInt(100),
							big.NewInt(150),
							false,
							false,
						}
						txArgs.Amount = big.NewInt(totalToSend)

						transferCheck := passCheck.WithExpEvents(erc20.EventTypeTransfer)
						res, _, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, args, transferCheck)
						Expect(err).To(BeNil())
						Expect(is.network.NextBlock()).To(BeNil())
						fees := math.NewIntFromBigInt(gasPrice).MulRaw(res.GasUsed)

						// contract balance should remain unchanged
						balRes, err = is.handler.GetBalanceFromBank(receiver.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						denomFinalBalance := balRes.Balance
						Expect(denomFinalBalance.Amount).To(Equal(denomInitialBalance.Amount.Add(math.NewInt(totalToSend))))

						balRes, err = is.handler.GetBalanceFromBank(revertContractAddr.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						contractBalance := balRes.Balance
						Expect(contractBalance.Amount).To(Equal(math.ZeroInt()))

						balRes, err = is.handler.GetBalanceFromBank(sender.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						senderFinalBalance := balRes.Balance
						denomSpent := fees.AddRaw(totalToSend)
						Expect(senderFinalBalance.Amount).To(Equal(senderInitialBalance.Amount.Sub(denomSpent)))
					},
					)
					DescribeTable("it should revert token transfer and send from WEVMOS contract", func(before bool, after bool) {
						sender := is.keyring.GetKey(0)
						receiver := is.keyring.GetAddr(1)
						balRes, err := is.handler.GetBalanceFromBank(receiver.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						denomInitialBalance := balRes.Balance
						balRes, err = is.handler.GetBalanceFromBank(sender.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						senderInitialBalance := balRes.Balance

						args.MethodName = "testTransferAndSend"
						args.Args = []interface{}{
							receiver,
							big.NewInt(100),
							big.NewInt(100),
							big.NewInt(100),
							before,
							after,
						}
						txArgs.Amount = big.NewInt(300)

						revertReasonCheck := execRevertedCheck.WithErrNested("revert here")

						res, _, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, args, revertReasonCheck)
						Expect(err).To(BeNil())
						Expect(is.network.NextBlock()).To(BeNil())
						fees := math.NewIntFromBigInt(gasPrice).MulRaw(res.GasUsed)

						// contract balance should remain unchanged
						balRes, err = is.handler.GetBalanceFromBank(receiver.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						denomFinalBalance := balRes.Balance
						Expect(denomFinalBalance.Amount).To(Equal(denomInitialBalance.Amount))

						balRes, err = is.handler.GetBalanceFromBank(revertContractAddr.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						contractBalance := balRes.Balance
						Expect(contractBalance.Amount).To(Equal(math.ZeroInt()))

						balRes, err = is.handler.GetBalanceFromBank(sender.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						senderFinalBalance := balRes.Balance
						Expect(senderFinalBalance.Amount).To(Equal(senderInitialBalance.Amount.Sub(fees)))
					},
						Entry("revert before", true, false),
						Entry("revert after", false, true),
					)
					It("revert when transfer with try", func() {
						sender := is.keyring.GetKey(0)
						receiver := is.keyring.GetAddr(1)
						amountToSend := big.NewInt(100)
						balRes, err := is.handler.GetBalanceFromBank(receiver.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						denomInitialBalance := balRes.Balance
						balRes, err = is.handler.GetBalanceFromBank(sender.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						senderInitialBalance := balRes.Balance

						args.MethodName = "transfersWithTry"
						args.Args = []interface{}{
							receiver,
							amountToSend,
							amountToSend,
						}
						txArgs.Amount = big.NewInt(200)

						transferCheck := passCheck.WithExpEvents(erc20.EventTypeTransfer)
						res, _, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, args, transferCheck)
						Expect(err).To(BeNil())
						Expect(is.network.NextBlock()).To(BeNil())
						fees := math.NewIntFromBigInt(gasPrice).MulRaw(res.GasUsed)

						balRes, err = is.handler.GetBalanceFromBank(receiver.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						denomFinalBalance := balRes.Balance
						Expect(denomFinalBalance.Amount).To(Equal(denomInitialBalance.Amount.Add(math.NewInt(amountToSend.Int64()))))

						balRes, err = is.handler.GetBalanceFromBank(revertContractAddr.Bytes(), is.bondDenom)
						Expect(err).To(BeNil())
						contractBalance := balRes.Balance
						Expect(contractBalance.Amount.Int64()).To(Equal(amountToSend.Int64()))

						balRes, err = is.handler.GetBalanceFromBank(sender.AccAddr, is.bondDenom)
						Expect(err).To(BeNil())
						senderFinalBalance := balRes.Balance
						denomSpent := fees.AddRaw(amountToSend.Int64() + amountToSend.Int64())
						Expect(senderFinalBalance.Amount).To(Equal(senderInitialBalance.Amount.Sub(denomSpent)))
					})
				})
			})

			When("transferring tokens from another account", func() {
				Context("in a direct call to the token contract", func() {
					DescribeTable("it should transfer tokens from another account with a sufficient approval set", func(callType CallType) {
						owner := is.keyring.GetKey(0)
						spender := is.keyring.GetKey(1)
						receiver := utiltx.GenerateAddress()

						fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 300)}
						transferAmount := big.NewInt(100)
						transferCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, transferAmount.Int64())}

						// Fund account with some tokens
						ownerInitialAmt := is.fundWithTokens(callType, contractsData, owner.Addr, fundCoins)
						ownerInitialBalance := sdk.Coins{sdk.NewCoin(is.tokenDenom, ownerInitialAmt)}

						// Set allowance
						is.setAllowanceForContract(callType, contractsData, owner.Priv, spender.Addr, transferAmount)

						// Transfer tokens
						txArgs, transferArgs := is.getTxAndCallArgs(
							callType, contractsData,
							erc20.TransferFromMethod,
							owner.Addr, receiver, transferAmount,
						)

						transferCheck := passCheck.WithExpEvents(
							erc20.EventTypeTransfer,
							erc20.EventTypeApproval,
						)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(spender.Priv, txArgs, transferArgs, transferCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit the changes to the chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.TransferFromMethod)
						is.ExpectBalancesForContract(
							callType, contractsData,
							[]ExpectedBalance{
								{address: owner.AccAddr, expCoins: ownerInitialBalance.Sub(transferCoins...)},
								{address: receiver.Bytes(), expCoins: transferCoins},
							},
						)

						// Check that the allowance was removed since we approved only the transferred amount
						is.ExpectAllowanceForContract(
							callType, contractsData,
							spender.Addr, owner.Addr, common.Big0,
						)
					},
						Entry(" - direct call", directCall),
						// NOTE: we are not passing the contract call here because this test is for direct calls only

						Entry(" - through erc20 contract", erc20Call),
						Entry(" - through erc20 v5 contract", erc20V5Call),
					)

					When("the spender is the same as the sender", func() {
						It("should return an error when transfer funds without an approval when calling the erc20 precompile", func() {
							owner := is.keyring.GetKey(0)
							spender := owner
							receiver := utiltx.GenerateAddress()

							fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 300)}
							transferCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 100)}

							// Fund account with some tokens
							ownerInitialAmt := is.fundWithTokens(directCall, contractsData, owner.Addr, fundCoins)
							ownerInitialBalance := sdk.Coins{sdk.NewCoin(is.tokenDenom, ownerInitialAmt)}

							// Transfer tokens
							txArgs, transferArgs := is.getTxAndCallArgs(
								directCall, contractsData,
								erc20.TransferFromMethod,
								owner.Addr, receiver, transferCoins[0].Amount.BigInt(),
							)

							insufficientAllowanceCheck := failCheck.WithErrContains(
								erc20.ErrInsufficientAllowance.Error(),
							)

							_, ethRes, err := is.factory.CallContractAndCheckLogs(spender.Priv, txArgs, transferArgs, insufficientAllowanceCheck)
							Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
							Expect(ethRes).To(BeNil(), "expected empty result")

							// commit changes to chain state
							err = is.network.NextBlock()
							Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

							is.ExpectBalancesForContract(
								directCall, contractsData,
								[]ExpectedBalance{
									{address: owner.AccAddr, expCoins: ownerInitialBalance},
									{address: receiver.Bytes(), expCoins: sdk.NewCoins(sdk.NewCoin(is.tokenDenom, math.ZeroInt()))},
								},
							)
						})

						DescribeTable("it should transfer funds from the own account in case sufficient approval is set", func(callType CallType) {
							owner := is.keyring.GetKey(0)
							receiver := utiltx.GenerateAddress()

							fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 300)}
							transferCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 100)}

							// Fund account with some tokens
							ownerInitialAmt := is.fundWithTokens(callType, contractsData, owner.Addr, fundCoins)
							ownerInitialBalance := sdk.Coins{sdk.NewCoin(is.tokenDenom, ownerInitialAmt)}

							is.setAllowanceForContract(
								callType, contractsData,
								owner.Priv, owner.Addr, transferCoins[0].Amount.BigInt(),
							)

							is.ExpectAllowanceForContract(
								callType, contractsData,
								owner.Addr, owner.Addr, transferCoins[0].Amount.BigInt(),
							)

							err := is.network.NextBlock()
							Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

							// Transfer tokens
							txArgs, transferArgs := is.getTxAndCallArgs(
								callType, contractsData,
								erc20.TransferFromMethod,
								owner.Addr, receiver, transferCoins[0].Amount.BigInt(),
							)

							transferCheck := passCheck.WithExpEvents(
								erc20.EventTypeTransfer,
								erc20.EventTypeApproval,
							)

							_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, transferArgs, transferCheck)
							Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

							// commit changes to chain state
							err = is.network.NextBlock()
							Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

							is.ExpectTrueToBeReturned(ethRes, erc20.TransferFromMethod)
							is.ExpectBalancesForContract(
								callType, contractsData,
								[]ExpectedBalance{
									{address: owner.AccAddr, expCoins: ownerInitialBalance.Sub(transferCoins...)},
									{address: receiver.Bytes(), expCoins: transferCoins},
								},
							)

							// Check that the allowance was removed since we approved only the transferred amount
							// FIXME: This is not working for the case where we transfer from the own account
							// because the allowance is not removed on the SDK side.
							is.ExpectAllowanceForContract(
								callType, contractsData,
								owner.Addr, owner.Addr, common.Big0,
							)
						},
							Entry(" - through erc20 contract", erc20Call),
							Entry(" - through erc20 v5 contract", erc20V5Call),
						)
					})

					DescribeTable("it should return an error when the spender does not have enough allowance", func(callType CallType) {
						owner := is.keyring.GetKey(0)
						spender := is.keyring.GetKey(1)
						receiver := utiltx.GenerateAddress()
						fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 300)}
						approveAmount := big.NewInt(100)
						transferAmount := big.NewInt(200)

						// Fund account with some tokens
						is.fundWithTokens(callType, contractsData, owner.Addr, fundCoins)
						// Set allowance
						is.setAllowanceForContract(
							callType, contractsData,
							owner.Priv, spender.Addr, approveAmount,
						)

						// Transfer tokens
						txArgs, transferArgs := is.getTxAndCallArgs(
							callType, contractsData,
							erc20.TransferFromMethod,
							owner.Addr, receiver, transferAmount,
						)

						insufficientAllowanceCheck := failCheck.WithErrContains(erc20.ErrInsufficientAllowance.Error())

						_, ethRes, err := is.factory.CallContractAndCheckLogs(spender.Priv, txArgs, transferArgs, insufficientAllowanceCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
						Expect(ethRes).To(BeNil(), "expected empty result")
					},
						Entry(" - direct call", directCall),
						// NOTE: we are not passing the contract call here because this test case only covers direct calls

						Entry(" - through erc20 contract", erc20Call),

						// TODO: the ERC20 V5 contract is raising the ERC-6093 standardized error which we are not using as of yet
						// Entry(" - through erc20 v5 contract", erc20V5Call),
					)

					DescribeTable("it should return an error if there is no allowance set", func(callType CallType) {
						sender := is.keyring.GetKey(0)
						from := is.keyring.GetKey(1)
						receiver := utiltx.GenerateAddress()
						fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 300)}
						transferAmount := big.NewInt(100)

						// Fund account with some tokens
						is.fundWithTokens(callType, contractsData, from.Addr, fundCoins)

						// Transfer tokens
						txArgs, transferArgs := is.getTxAndCallArgs(
							callType, contractsData,
							erc20.TransferFromMethod,
							from.Addr, receiver, transferAmount,
						)

						insufficientAllowanceCheck := failCheck.WithErrContains(
							erc20.ErrInsufficientAllowance.Error(),
						)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, transferArgs, insufficientAllowanceCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
						Expect(ethRes).To(BeNil(), "expected empty result")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")
					},
						Entry(" - direct call", directCall),
						// NOTE: we are not passing the contract call here because this test case only covers direct calls

						Entry(" - through erc20 contract", erc20Call),

						// TODO: the ERC20 V5 contract is raising the ERC-6093 standardized error which we are not using as of yet
						// Entry(" - through erc20 v5 contract", erc20V5Call),
					)

					DescribeTable("it should return an error if the sender does not have enough tokens", func(callType CallType) {
						sender := is.keyring.GetKey(0)
						from := is.keyring.GetKey(1)
						receiver := utiltx.GenerateAddress()
						fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 200)}

						// Fund account with some tokens
						senderInitialAmt := is.fundWithTokens(callType, contractsData, from.Addr, fundCoins)
						transferAmt := new(big.Int).Add(senderInitialAmt.BigInt(), big.NewInt(100))

						// Set allowance
						is.setAllowanceForContract(
							callType, contractsData,
							from.Priv, sender.Addr, transferAmt,
						)

						// Transfer tokens
						txArgs, transferArgs := is.getTxAndCallArgs(callType, contractsData, erc20.TransferFromMethod, from.Addr, receiver, transferAmt)

						insufficientBalanceCheck := failCheck.WithErrContains(
							erc20.ErrTransferAmountExceedsBalance.Error(),
						)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, transferArgs, insufficientBalanceCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
						Expect(ethRes).To(BeNil(), "expected empty result")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")
					},
						Entry(" - direct call", directCall),
						// NOTE: we are not passing the contract call here because this test case only covers direct calls

						Entry(" - through erc20 contract", erc20Call),

						// TODO: the ERC20 V5 contract is raising the ERC-6093 standardized error which we are not using as of yet
						// Entry(" - through erc20 v5 contract", erc20V5Call),
					)
				})

				Context("in a call from another smart contract to the token contract", func() {
					DescribeTable("it should transfer tokens with a sufficient approval set", func(callType CallType) {
						owner := is.keyring.GetKey(0)
						receiver := utiltx.GenerateAddress()
						fundCoin := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 300)}
						transferAmount := big.NewInt(100)
						transferCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, transferAmount.Int64())}

						// NOTE: the spender will be the contract address
						spender := contractsData.GetContractData(callType).Address

						// Fund account with some tokens
						ownerInitialAmt := is.fundWithTokens(callType, contractsData, owner.Addr, fundCoin)
						ownerInitialBalance := sdk.Coins{sdk.NewCoin(is.tokenDenom, ownerInitialAmt)}

						// Set allowance
						is.setAllowanceForContract(
							callType, contractsData,
							owner.Priv, spender, transferAmount,
						)

						// Transfer tokens
						txArgs, transferArgs := is.getTxAndCallArgs(
							callType, contractsData,
							erc20.TransferFromMethod,
							owner.Addr, receiver, transferCoins[0].Amount.BigInt(),
						)

						transferCheck := passCheck.WithExpEvents(
							erc20.EventTypeTransfer,
							erc20.EventTypeApproval,
						)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, transferArgs, transferCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.TransferFromMethod)
						is.ExpectBalancesForContract(
							callType, contractsData,
							[]ExpectedBalance{
								{address: owner.AccAddr, expCoins: ownerInitialBalance.Sub(transferCoins...)},
								{address: receiver.Bytes(), expCoins: transferCoins},
							},
						)

						// Check that the allowance was removed since we approved only the transferred amount
						is.ExpectAllowanceForContract(
							callType, contractsData,
							spender, owner.Addr, common.Big0,
						)
					},
						// Entry(" - direct call", directCall),
						Entry(" - through contract", contractCall),
						// NOTE: we are not passing the erc20 contract call here because this is supposed to
						// test external contract calls
						Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
					)

					DescribeTable("it should transfer funds with a sufficient allowance and triggered from another account", func(callType CallType) {
						msgSender := is.keyring.GetKey(0)
						owner := is.keyring.GetKey(1)
						receiver := utiltx.GenerateAddress()

						// NOTE: the spender will be the contract address
						spender := contractsData.GetContractData(callType).Address

						fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 300)}
						transferCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 100)}

						// Fund account with some tokens
						ownerInitialAmt := is.fundWithTokens(callType, contractsData, owner.Addr, fundCoins)
						ownerInitialBalance := sdk.Coins{sdk.NewCoin(is.tokenDenom, ownerInitialAmt)}

						// Set allowance
						is.setAllowanceForContract(
							callType, contractsData,
							owner.Priv, spender, transferCoins.AmountOf(is.tokenDenom).BigInt(),
						)

						// Transfer tokens
						txArgs, transferArgs := is.getTxAndCallArgs(
							callType, contractsData,
							erc20.TransferFromMethod,
							owner.Addr, receiver, transferCoins[0].Amount.BigInt(),
						)

						transferCheck := passCheck.WithExpEvents(
							erc20.EventTypeTransfer,
							erc20.EventTypeApproval,
						)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(msgSender.Priv, txArgs, transferArgs, transferCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.TransferFromMethod)
						is.ExpectBalancesForContract(
							callType, contractsData,
							[]ExpectedBalance{
								{address: owner.AccAddr, expCoins: ownerInitialBalance.Sub(transferCoins...)},
								{address: receiver.Bytes(), expCoins: transferCoins},
							},
						)

						// Check that the allowance was removed since we approved only the transferred amount
						is.ExpectAllowanceForContract(
							callType, contractsData,
							spender, owner.Addr, common.Big0,
						)
					},
						// NOTE: we are not passing the direct call here because this test is specific to the contract calls

						Entry(" - through contract", contractCall),
						Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
					)

					DescribeTable("it should return an error when the spender does not have enough allowance", func(callType CallType) {
						from := is.keyring.GetKey(0)
						receiver := utiltx.GenerateAddress()
						fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 400)}
						approveAmount := big.NewInt(100)
						transferAmount := big.NewInt(300)

						// NOTE: the spender will be the contract address
						spender := contractsData.GetContractData(callType).Address

						// Fund account with some tokens
						is.fundWithTokens(callType, contractsData, from.Addr, fundCoins)

						// Set allowance
						is.setAllowanceForContract(callType, contractsData, from.Priv, spender, approveAmount)

						// Transfer tokens
						txArgs, transferArgs := is.getTxAndCallArgs(
							callType, contractsData,
							erc20.TransferFromMethod,
							from.Addr, receiver, transferAmount,
						)

						revertReasonCheck := execRevertedCheck.WithErrNested(erc20.ErrInsufficientAllowance.Error())

						_, ethRes, err := is.factory.CallContractAndCheckLogs(from.Priv, txArgs, transferArgs, revertReasonCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
						Expect(ethRes).To(BeNil(), "expected empty result")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")
					},
						// NOTE: we are not passing the direct call here because this test is for contract calls only
						Entry(" - through contract", contractCall),
						Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
					)
				})
			})

			When("querying balance", func() {
				DescribeTable("it should return an existing balance", func(callType CallType) {
					sender := is.keyring.GetKey(0)
					addedAmt := big.NewInt(100)
					fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, addedAmt.Int64())}

					// Fund account with some tokens
					ownerInitialAmt := is.fundWithTokens(callType, contractsData, sender.Addr, fundCoins)

					// Query the balance
					txArgs, balancesArgs := is.getTxAndCallArgs(callType, contractsData, erc20.BalanceOfMethod, sender.Addr)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var balance *big.Int
					err = is.precompile.UnpackIntoInterface(&balance, erc20.BalanceOfMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(math.NewIntFromBigInt(balance)).To(Equal(ownerInitialAmt), "expected different balance")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 contract", erc20Call),
					Entry(" - through erc20 v5 contract", erc20V5Call),
					Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
				)

				DescribeTable("it should return zero if balance only exists for other tokens", func(callType CallType) {
					sender := is.keyring.GetKey(0)
					address := utiltx.GenerateAddress()
					fundCoins := sdk.Coins{sdk.NewInt64Coin(is.network.GetBaseDenom(), 100)}

					// Fund account with some tokens
					err := is.factory.FundAccount(is.keyring.GetKey(0), sender.AccAddr, fundCoins)
					Expect(err).ToNot(HaveOccurred(), "failed to fund account")
					Expect(is.network.NextBlock()).To(BeNil())

					// Query the balance
					txArgs, balancesArgs := is.getTxAndCallArgs(callType, contractsData, erc20.BalanceOfMethod, address)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var balance *big.Int
					err = is.precompile.UnpackIntoInterface(&balance, erc20.BalanceOfMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(balance.Int64()).To(BeZero(), "expected zero balance")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					// NOTE: we are not passing the erc20 contract call here because the ERC20 contracts
					// only support the actual token denomination and don't know of other balances.
				)

				DescribeTable("it should return zero if the account does not exist", func(callType CallType) {
					sender := is.keyring.GetKey(0)
					address := utiltx.GenerateAddress()

					// Query the balance
					txArgs, balancesArgs := is.getTxAndCallArgs(callType, contractsData, erc20.BalanceOfMethod, address)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var balance *big.Int
					err = is.precompile.UnpackIntoInterface(&balance, erc20.BalanceOfMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(balance.Int64()).To(BeZero(), "expected zero balance")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 contract", erc20Call),
					Entry(" - through erc20 v5 contract", erc20V5Call),
					Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
				)
			})

			When("querying allowance", func() {
				DescribeTable("it should return an existing allowance", func(callType CallType) {
					spender := utiltx.GenerateAddress()
					owner := is.keyring.GetKey(0)
					approveAmount := big.NewInt(100)

					is.setAllowanceForContract(callType, contractsData, owner.Priv, spender, approveAmount)

					txArgs, allowanceArgs := is.getTxAndCallArgs(callType, contractsData, erc20.AllowanceMethod, owner.Addr, spender)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, allowanceArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var allowance *big.Int
					err = is.precompile.UnpackIntoInterface(&allowance, erc20.AllowanceMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(allowance).To(Equal(approveAmount), "expected different allowance")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 contract", erc20Call),
					Entry(" - through erc20 v5 contract", erc20V5Call),
					Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
				)

				When("querying the allowance for the own address", func() {
					It("should return the 0 value when calling the erc20 precompile", func() {
						spender := is.keyring.GetAddr(0)
						owner := is.keyring.GetKey(0)

						txArgs, allowanceArgs := is.getTxAndCallArgs(directCall, contractsData, erc20.AllowanceMethod, spender, spender)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, allowanceArgs, passCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						var allowance *big.Int
						err = is.precompile.UnpackIntoInterface(&allowance, erc20.AllowanceMethod, ethRes.Ret)
						Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
						Expect(allowance.Sign()).To(Equal(0), "expected different allowance")
					})

					// NOTE: Since it's possible to set an allowance for the own address with the Solidity ERC20 contracts,
					// we describe this case here for completion purposes, to describe the difference in behavior.
					DescribeTable("should return the actual allowance value when calling the ERC20 contract", func(callType CallType) {
						owner := is.keyring.GetKey(0)
						approveAmount := big.NewInt(100)

						is.setAllowanceForContract(callType, contractsData, owner.Priv, owner.Addr, approveAmount)

						txArgs, allowanceArgs := is.getTxAndCallArgs(callType, contractsData, erc20.AllowanceMethod, owner.Addr, owner.Addr)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, allowanceArgs, passCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						var allowance *big.Int
						err = is.precompile.UnpackIntoInterface(&allowance, erc20.AllowanceMethod, ethRes.Ret)
						Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
						Expect(allowance).To(Equal(approveAmount), "expected different allowance")
					},
						Entry(" - through erc20 contract", erc20Call),
						Entry(" - through erc20 v5 contract", erc20V5Call),
					)
				})

				DescribeTable("it should return zero if no allowance exists", func(callType CallType) {
					spender := is.keyring.GetAddr(1)
					owner := is.keyring.GetKey(0)

					txArgs, allowanceArgs := is.getTxAndCallArgs(callType, contractsData, erc20.AllowanceMethod, owner.Addr, spender)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, allowanceArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var allowance *big.Int
					err = is.precompile.UnpackIntoInterface(&allowance, erc20.AllowanceMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(allowance.Int64()).To(BeZero(), "expected zero allowance")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 contract", erc20Call),
					Entry(" - through erc20 v5 contract", erc20V5Call),
					Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
				)

				DescribeTable("it should return zero if the account does not exist", func(callType CallType) {
					spender := utiltx.GenerateAddress()
					owner := is.keyring.GetKey(0)

					txArgs, allowanceArgs := is.getTxAndCallArgs(callType, contractsData, erc20.AllowanceMethod, owner.Addr, spender)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, allowanceArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var allowance *big.Int
					err = is.precompile.UnpackIntoInterface(&allowance, erc20.AllowanceMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(allowance.Int64()).To(BeZero(), "expected zero allowance")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 contract", erc20Call),
					Entry(" - through erc20 v5 contract", erc20V5Call),
					Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
				)
			})

			When("querying total supply", func() {
				DescribeTable("it should return the total supply", func(callType CallType) {
					sender := is.keyring.GetKey(0)
					expSupply := big.NewInt(100)
					fundCoins := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, expSupply.Int64())}

					// Fund account with some tokens
					is.fundWithTokens(callType, contractsData, sender.Addr, fundCoins)

					// if is native coin, get expSupply from the bank mod
					if slices.Contains(nativeCallTypes, callType) {
						qc := is.network.GetBankClient()
						qRes, err := qc.SupplyOf(is.network.GetContext(), &banktypes.QuerySupplyOfRequest{Denom: is.tokenDenom})
						Expect(err).To(BeNil())
						Expect(qRes).NotTo(BeNil())
						expSupply = qRes.Amount.Amount.BigInt()
					}

					// Query the balance
					txArgs, supplyArgs := is.getTxAndCallArgs(callType, contractsData, erc20.TotalSupplyMethod)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var supply *big.Int
					err = is.precompile.UnpackIntoInterface(&supply, erc20.TotalSupplyMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(supply).To(Equal(expSupply), "expected different supply")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 contract", erc20Call),
					Entry(" - through erc20 v5 contract", erc20V5Call),
					Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
				)

				DescribeTable("it should return zero if no tokens exist", func(callType CallType) {
					sender := is.keyring.GetKey(0)
					txArgs, supplyArgs := is.getTxAndCallArgs(callType, contractsData, erc20.TotalSupplyMethod)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var supply *big.Int
					err = is.precompile.UnpackIntoInterface(&supply, erc20.TotalSupplyMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(supply.Int64()).To(BeZero(), "expected zero supply")
				},
					Entry(" - direct call", directCallToken2),
					Entry(" - through contract", contractCallToken2),
					Entry(" - through erc20 contract", erc20Call),
					Entry(" - through erc20 v5 contract", erc20V5Call),
					Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
				)
			})

			When("approving an allowance", func() {
				Context("in a call to the token contract", func() {
					DescribeTable("it should approve an allowance", func(callType CallType) {
						spender := is.keyring.GetKey(0)
						owner := is.keyring.GetKey(1)
						approveAmount := big.NewInt(200)

						// Approve allowance
						txArgs, approveArgs := is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, approveAmount)

						approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)
						is.ExpectAllowanceForContract(
							callType, contractsData,
							owner.Addr, spender.Addr, approveAmount,
						)
					},
						Entry(" - direct call", directCall),
						Entry(" - through erc20 contract", erc20Call),
						Entry(" - through erc20 v5 contract", erc20V5Call),
					)

					DescribeTable("it should set the new spend limit for an existing allowance", func(callType CallType) {
						spender := is.keyring.GetKey(1)
						owner := is.keyring.GetKey(0)
						prevAllowance := big.NewInt(200)
						approveAmount := big.NewInt(100)

						// set up a previous allowance
						is.setAllowanceForContract(callType, contractsData, owner.Priv, spender.Addr, prevAllowance)

						// Approve allowance
						txArgs, approveArgs := is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, approveAmount)

						approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)
						// Check allowance contains both spend limits
						is.ExpectAllowanceForContract(callType, contractsData, owner.Addr, spender.Addr, approveAmount)
					},
						Entry(" - direct call", directCall),
						Entry(" - through erc20 contract", erc20Call),
						Entry(" - through erc20 v5 contract", erc20V5Call),
					)

					DescribeTable("it should remove the token from the spend limit of an existing allowance when approving zero", func(callType CallType) {
						owner := is.keyring.GetKey(0)
						spender := is.keyring.GetKey(1)
						approveAmount := big.NewInt(100)

						// set up a previous allowance
						is.setAllowanceForContract(callType, contractsData, owner.Priv, spender.Addr, approveAmount)

						// Approve allowance
						txArgs, approveArgs := is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, common.Big0)

						approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)

						// commit the changes to state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error while calling NextBlock")

						// Check allowance is set to zero
						is.ExpectAllowanceForContract(callType, contractsData, owner.Addr, spender.Addr, common.Big0)
					},
						Entry(" - direct call", directCall),
						// NOTE: we are not passing the erc20 contract call here because the ERC20 contract
						// only supports the actual token denomination and doesn't know of other allowances.
					)

					DescribeTable("it should delete the allowance when approving zero with no other spend limits", func(callType CallType) {
						spender := is.keyring.GetKey(1)
						owner := is.keyring.GetKey(0)
						allowance := big.NewInt(100)

						// set up a previous allowance
						is.setAllowanceForContract(callType, contractsData, owner.Priv, spender.Addr, allowance)

						// Approve allowance
						txArgs, approveArgs := is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, common.Big0)

						approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)
						// Check allowance was deleted
						is.ExpectAllowanceForContract(callType, contractsData, owner.Addr, spender.Addr, common.Big0)
					},
						Entry(" - direct call", directCall),
						Entry(" - through erc20 contract", erc20Call),
						Entry(" - through erc20 v5 contract", erc20V5Call),
					)

					DescribeTable("it should no-op if approving 0 and no allowance exists", func(callType CallType) {
						spender := is.keyring.GetKey(1)
						owner := is.keyring.GetKey(0)

						// Approve allowance
						txArgs, approveArgs := is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, common.Big0)

						// We are expecting an approval to be made, but no allowance stored since it's 0
						approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)
						// Check still no allowance exists
						is.ExpectAllowanceForContract(callType, contractsData, owner.Addr, spender.Addr, common.Big0)
					},
						Entry(" - direct call", directCall),
						Entry(" - through erc20 contract", erc20Call),
						Entry(" - through erc20 v5 contract", erc20V5Call),
					)

					When("the spender is the same as the owner", func() {
						It("should return an error when calling the erc20 precompile", func() {
							spender := is.keyring.GetKey(0)
							owner := is.keyring.GetKey(0)
							approveAmount := big.NewInt(100)

							// Approve allowance
							txArgs, approveArgs := is.getTxAndCallArgs(
								directCall, contractsData,
								erc20.ApproveMethod,
								spender.Addr, approveAmount,
							)

							approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)

							_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, approveArgs, approveCheck)
							Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

							// commit changes to chain state
							err = is.network.NextBlock()
							Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

							is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)
							is.ExpectAllowanceForContract(
								directCall, contractsData,
								owner.Addr, spender.Addr, approveAmount,
							)
						})

						DescribeTable("it should create an allowance", func(callType CallType) {
							spender := is.keyring.GetKey(0)
							owner := is.keyring.GetKey(0)
							allowance := big.NewInt(100)

							// Approve allowance
							txArgs, approveArgs := is.getTxAndCallArgs(
								callType, contractsData,
								erc20.ApproveMethod,
								spender.Addr, allowance,
							)

							approvalCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)

							_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, approveArgs, approvalCheck)
							Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

							// commit changes to chain state
							err = is.network.NextBlock()
							Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

							is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)
							is.ExpectAllowanceForContract(
								callType, contractsData,
								owner.Addr, spender.Addr, allowance,
							)
						},
							Entry(" - through erc20 contract", erc20Call),
							Entry(" - through erc20 v5 contract", erc20V5Call),
						)
					})

					DescribeTable("it should return an error if approving 0 and allowance only exists for other tokens", func(callType CallType) {
						owner := is.keyring.GetKey(0)
						spender := is.keyring.GetKey(1)
						prevAllowance := big.NewInt(200)

						callTypeForOtherToken := directCallToken2

						// set up a previous allowance for other token
						is.setAllowanceForContract(callTypeForOtherToken, contractsData, owner.Priv, spender.Addr, prevAllowance)

						// Approve allowance for target token
						txArgs, approveArgs := is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, common.Big0)

						approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(owner.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)
						is.ExpectAllowanceForContract(
							callType, contractsData,
							owner.Addr, spender.Addr, common.Big0,
						)

						// Check allowance of other token is not chaged
						is.ExpectAllowanceForContract(
							callTypeForOtherToken, contractsData,
							owner.Addr, spender.Addr, prevAllowance,
						)
					},
						Entry(" - direct call", directCall),
						// NOTE: we are not passing the erc20 contract call here because the ERC20 contract
						// only supports the actual token denomination and doesn't know of other allowances.
					)
				})

				// NOTE: We have to split the tests for contract calls into a separate context because
				// when approving through a smart contract, the approval is created between the contract address and the
				// spender, instead of the sender address and the spender.
				Context("in a contract call", func() {
					DescribeTable("it should approve an allowance", func(callType CallType) {
						sender := is.keyring.GetKey(0)
						spender := is.keyring.GetKey(1)
						owner := contractsData.GetContractData(callType).Address // the owner will be the contract address
						transferAmount := big.NewInt(200)

						// Approve allowance
						txArgs, approveArgs := is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, transferAmount)

						approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)
						// Check allowance
						is.ExpectAllowanceForContract(
							callType, contractsData,
							owner, spender.Addr, transferAmount,
						)
					},
						Entry(" - through contract", contractCall),
						Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
					)

					DescribeTable("it should set the new spend limit for an existing allowance with the same token", func(callType CallType) {
						sender := is.keyring.GetKey(0)
						spender := is.keyring.GetKey(1)
						owner := contractsData.GetContractData(callType).Address // the owner will be the contract address
						initialAmount := sdk.Coins{sdk.NewInt64Coin(is.tokenDenom, 100)}
						newAmount := big.NewInt(200)

						// Set up a first approval
						txArgs, approveArgs := is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, initialAmount[0].Amount.BigInt())
						approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)
						_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)

						// Set up a second approval which should overwrite the initial one
						txArgs, approveArgs = is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, newAmount)
						approveCheck = passCheck.WithExpEvents(erc20.EventTypeApproval)
						_, ethRes, err = is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						// Check allowance has been updated
						is.ExpectAllowanceForContract(
							callType, contractsData,
							owner, spender.Addr, newAmount,
						)
					},
						Entry(" - through contract", contractCall),
						Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
					)

					DescribeTable("it should delete the allowance when approving zero with no other spend limits", func(callType CallType) {
						sender := is.keyring.GetKey(0)
						spender := is.keyring.GetKey(1)
						owner := contractsData.GetContractData(callType).Address // the owner will be the contract address
						approveAmount := big.NewInt(100)

						// set up a previous allowance
						//
						// TODO: refactor using helper
						txArgs, approveArgs := is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, approveAmount)
						approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)
						_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)

						// Approve allowance
						txArgs, approveArgs = is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, common.Big0)
						_, ethRes, err = is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)

						// Check allowance was deleted from the keeper / is returning 0 for smart contracts
						is.ExpectAllowanceForContract(callType, contractsData, owner, spender.Addr, common.Big0)
					},
						Entry(" - through contract", contractCall),
						Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
					)

					DescribeTable("it should no-op if approving 0 and no allowance exists", func(callType CallType) {
						sender := is.keyring.GetKey(0)
						spender := is.keyring.GetKey(1)
						owner := contractsData.GetContractData(callType).Address // the owner will be the contract address

						// Approve allowance
						txArgs, approveArgs := is.getTxAndCallArgs(callType, contractsData, erc20.ApproveMethod, spender.Addr, common.Big0)

						// We are expecting an approval event to be emitted, but no allowance to be stored
						approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, approveArgs, approveCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// commit changes to chain state
						err = is.network.NextBlock()
						Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

						is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)
						// Check still no allowance exists
						is.ExpectAllowanceForContract(callType, contractsData, owner, spender.Addr, common.Big0)
					},
						Entry(" - through contract", contractCall),
						Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
					)

					When("the spender is the same as the owner", func() {
						It("should approve an allowance", func() {
							callType := contractCall
							sender := is.keyring.GetKey(0)
							owner := contractsData.GetContractData(callType).Address // the owner will be the contract address
							spender := owner
							approveAmount := big.NewInt(100)

							// Approve allowance
							txArgs, approveArgs := is.getTxAndCallArgs(
								callType, contractsData,
								erc20.ApproveMethod,
								spender, approveAmount,
							)

							approveCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)
							_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, approveArgs, approveCheck)
							Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

							// commit changes to chain state
							err = is.network.NextBlock()
							Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

							is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)
							is.ExpectAllowanceForContract(
								callType, contractsData,
								owner, spender, approveAmount,
							)
						})

						DescribeTable("it should create an allowance when calling an ERC20 Solidity contract", func(callType CallType) {
							sender := is.keyring.GetKey(0)
							owner := contractsData.GetContractData(callType).Address // the owner will be the contract address
							spender := owner
							allowance := big.NewInt(100)

							// Approve allowance
							txArgs, approveArgs := is.getTxAndCallArgs(
								callType, contractsData,
								erc20.ApproveMethod,
								spender, allowance,
							)

							approvalCheck := passCheck.WithExpEvents(erc20.EventTypeApproval)

							_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, txArgs, approveArgs, approvalCheck)
							Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

							// commit changes to chain state
							err = is.network.NextBlock()
							Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

							is.ExpectTrueToBeReturned(ethRes, erc20.ApproveMethod)
							is.ExpectAllowanceForContract(
								callType, contractsData,
								owner, spender, allowance,
							)
						},
							Entry(" - through erc20 v5 caller contract", erc20V5CallerCall),
						)
					})
				})
			})
		})

		Context("metadata query -", func() {
			Context("for a token without registered metadata", func() {
				BeforeEach(func() {
					// Deploy ERC20NoMetadata contract for this test
					erc20NoMetadataContract, err := testdata.LoadERC20NoMetadataContract()
					Expect(err).ToNot(HaveOccurred(), "failed to load contract")

					erc20NoMetadataAddr, err := is.factory.DeployContract(
						is.keyring.GetPrivKey(0),
						evmtypes.EvmTxArgs{},
						testutiltypes.ContractDeploymentData{
							Contract: erc20NoMetadataContract,
						},
					)
					Expect(err).ToNot(HaveOccurred(), "failed to deploy contract")

					// commit changes to chain state
					err = is.network.NextBlock()
					Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

					// NOTE: update the address but leave the ABI as it is, so that the ABI includes
					// the metadata methods but the contract doesn't have them.
					contractsData.contractData[erc20Call] = ContractData{
						Address: erc20NoMetadataAddr,
						ABI:     contracts.ERC20MinterBurnerDecimalsContract.ABI,
					}
				})

				DescribeTable("querying the name should return an error", func(callType CallType) {
					txArgs, nameArgs := is.getTxAndCallArgs(callType, contractsData, erc20.NameMethod)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(is.keyring.GetPrivKey(0), txArgs, nameArgs, execRevertedCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
					Expect(ethRes).To(BeNil(), "expected empty result")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 contract", erc20Call), // NOTE: we're passing the ERC20 contract call here which was adjusted to point to a contract without metadata to expect the same errors
				)

				DescribeTable("querying the symbol should return an error", func(callType CallType) {
					txArgs, symbolArgs := is.getTxAndCallArgs(callType, contractsData, erc20.SymbolMethod)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(is.keyring.GetPrivKey(0), txArgs, symbolArgs, execRevertedCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
					Expect(ethRes).To(BeNil(), "expected empty result")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 contract", erc20Call), // NOTE: we're passing the ERC20 contract call here which was adjusted to point to a contract without metadata to expect the same errors
				)

				DescribeTable("querying the decimals should return an error", func(callType CallType) {
					txArgs, decimalsArgs := is.getTxAndCallArgs(callType, contractsData, erc20.DecimalsMethod)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(is.keyring.GetPrivKey(0), txArgs, decimalsArgs, execRevertedCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
					Expect(ethRes).To(BeNil(), "expected empty result")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 contract", erc20Call), // NOTE: we're passing the ERC20 contract call here which was adjusted to point to a contract without metadata to expect the same errors
				)
			})

			Context("for a token with available metadata", func() {
				const (
					expSymbol   = "Xmpl"
					expDecimals = uint8(18)
				)

				var (
					erc20Addr common.Address
					expName   string
				)

				BeforeEach(func() {
					erc20Addr = contractsData.GetContractData(erc20V5Call).Address
					expName = erc20types.CreateDenom(erc20Addr.String())

					// Register ERC20 token pair for this test
					tokenPairs, err := utils.RegisterERC20(is.factory, is.network, utils.ERC20RegistrationData{
						Addresses:    []string{erc20Addr.Hex()},
						ProposerPriv: is.keyring.GetPrivKey(0),
					})
					Expect(err).ToNot(HaveOccurred(), "failed to register ERC20 token")
					Expect(tokenPairs).To(HaveLen(1))

					// overwrite the other precompile with this one, so that the test utils like is.getTxAndCallArgs still work.
					is.precompile, err = is.setupNewERC20PrecompileForTokenPair(tokenPairs[0])
					Expect(err).ToNot(HaveOccurred(), "failed to set up erc20 precompile")

					// commit changes to chain state
					err = is.network.NextBlock()
					Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

					// update this in the global contractsData
					contractsData.contractData[directCall] = ContractData{
						Address: is.precompile.Address(),
						ABI:     is.precompile.ABI,
					}

					// Deploy contract calling the ERC20 precompile
					callerAddr, err := is.factory.DeployContract(
						is.keyring.GetPrivKey(0),
						evmtypes.EvmTxArgs{},
						testutiltypes.ContractDeploymentData{
							Contract: erc20MdCallerContract,
							ConstructorArgs: []interface{}{
								is.precompile.Address(),
							},
						},
					)
					Expect(err).ToNot(HaveOccurred(), "failed to deploy contract")

					// commit changes to chain state
					err = is.network.NextBlock()
					Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

					contractsData.contractData[contractCall] = ContractData{
						Address: callerAddr,
						ABI:     erc20MdCallerContract.ABI,
					}
				})

				DescribeTable("querying the name should return the name", func(callType CallType) {
					txArgs, nameArgs := is.getTxAndCallArgs(callType, contractsData, erc20.NameMethod)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(is.keyring.GetPrivKey(0), txArgs, nameArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var name string
					err = is.precompile.UnpackIntoInterface(&name, erc20.NameMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(name).To(Equal(expName), "expected different name")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 v5 contract", erc20V5Call),
				)

				DescribeTable("querying the symbol should return the symbol", func(callType CallType) {
					txArgs, symbolArgs := is.getTxAndCallArgs(callType, contractsData, erc20.SymbolMethod)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(is.keyring.GetPrivKey(0), txArgs, symbolArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var symbol string
					err = is.precompile.UnpackIntoInterface(&symbol, erc20.SymbolMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(symbol).To(Equal(expSymbol), "expected different symbol")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 v5 contract", erc20V5Call),
				)

				DescribeTable("querying the decimals should return the decimals", func(callType CallType) {
					txArgs, decimalsArgs := is.getTxAndCallArgs(callType, contractsData, erc20.DecimalsMethod)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(is.keyring.GetPrivKey(0), txArgs, decimalsArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var decimals uint8
					err = is.precompile.UnpackIntoInterface(&decimals, erc20.DecimalsMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(decimals).To(Equal(expDecimals), "expected different decimals")
				},
					Entry(" - direct call", directCall),
					Entry(" - through contract", contractCall),
					Entry(" - through erc20 v5 contract", erc20V5Call),
				)
			})
		})
	})
	_ = Describe("ERC20 Extension migration Flows -", func() {
		When("migrating an existing ERC20 token", func() {
			var (
				contractData          ContractsData
				erc20MinterV5Contract evmtypes.CompiledContract

				tokenDenom  = "xmpl"
				tokenName   = "Xmpl"
				tokenSymbol = strings.ToUpper(tokenDenom)

				supply = sdk.NewInt64Coin(tokenDenom, 1000000000000000000)
			)

			BeforeEach(func() {
				is.SetupTest()

				var err error
				erc20MinterV5Contract, err = testdata.LoadERC20MinterV5Contract()
				Expect(err).ToNot(HaveOccurred(), "failed to load ERC20 minter contract")

				contractOwner := is.keyring.GetKey(0)

				// Deploy an ERC20 contract
				erc20Addr, err := is.factory.DeployContract(
					contractOwner.Priv,
					evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
					testutiltypes.ContractDeploymentData{
						Contract: erc20MinterV5Contract,
						ConstructorArgs: []interface{}{
							tokenName, tokenSymbol,
						},
					},
				)
				Expect(err).ToNot(HaveOccurred(), "failed to deploy contract")

				// NOTE: We need to overwrite the information in the contractData here for this specific
				// deployed contract.
				contractData = ContractsData{
					ownerPriv: contractOwner.Priv,
					contractData: map[CallType]ContractData{
						erc20V5Call: {
							Address: erc20Addr,
							ABI:     erc20MinterV5Contract.ABI,
						},
					},
				}

				err = is.network.NextBlock()
				Expect(err).ToNot(HaveOccurred(), "failed to commit block")

				// Register the deployed erc20 contract as a token pair
				_, err = utils.RegisterERC20(is.factory, is.network, utils.ERC20RegistrationData{
					Addresses:    []string{erc20Addr.Hex()},
					ProposerPriv: contractOwner.Priv,
				})
				Expect(err).ToNot(HaveOccurred(), "failed to register ERC20 token")

				err = is.network.NextBlock()
				Expect(err).ToNot(HaveOccurred(), "failed to commit block")

				// Mint the supply of tokens
				err = is.MintERC20(erc20V5Call, contractData, contractOwner.Addr, supply.Amount.BigInt())
				Expect(err).ToNot(HaveOccurred(), "failed to mint tokens")

				// Check that the supply was minted
				is.ExpectBalancesForERC20(erc20V5Call, contractData, []ExpectedBalance{{
					address:  contractOwner.AccAddr,
					expCoins: sdk.Coins{supply},
				}})
			})

			It("should migrate the full token balance to the bank module", func() {
				// TODO: implement test on follow-up PR
				Skip("will be addressed on follow-up PR")

				Expect(true).To(BeFalse(), "not implemented")
			})
		})

		When("migrating an extended ERC20 token (e.g. ERC20Votes)", func() {
			It("should migrate the full token balance to the bank module", func() {
				// TODO: make sure that extended tokens are compatible with the ERC20 extensions
				Skip("not included in first tranche")

				Expect(true).To(BeFalse(), "not implemented")
			})
		})

		When("running the migration logic for a set of existing ERC20 tokens", func() {
			BeforeEach(func() {
				// TODO: Add some ERC20 tokens and then run migration logic
				// TODO: check here that the balance cannot be queried from the bank keeper before migrating the token
			})

			It("should add and enable the corresponding erc20 precompiles", func() {
				Skip("will be addressed in follow-up PR")

				Expect(true).To(BeFalse(), "not implemented")
			})

			It("should be possible to query the balances through the bank module", func() {
				Skip("will be addressed in follow-up PR")

				Expect(true).To(BeFalse(), "not implemented")
			})

			It("should return all tokens when querying all balances for an account", func() {
				Skip("will be addressed in follow-up PR")

				Expect(true).To(BeFalse(), "not implemented")
			})
		})

		When("registering a native IBC coin", func() {
			BeforeEach(func() {
				// TODO: Add some IBC coins, register the token pair and then run migration logic
			})

			It("should add the corresponding erc20 precompiles", func() {
				Skip("will be addressed in follow-up PR")

				Expect(true).To(BeFalse(), "not implemented")
			})

			It("should be possible to query the balances using an EVM transaction", func() {
				Skip("will be addressed in follow-up PR")

				Expect(true).To(BeFalse(), "not implemented")
			})
		})

		When("using Evmos (not wEvmos) in smart contracts", func() {
			It("should be using straight Evmos for sending funds in smart contracts", func() {
				Skip("will be addressed in follow-up PR")

				Expect(true).To(BeFalse(), "not implemented")
			})
		})
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "ERC20 Extension Suite")
}
