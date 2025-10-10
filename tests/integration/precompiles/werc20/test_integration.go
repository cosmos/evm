package werc20

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/precompiles/werc20"
	"github.com/cosmos/evm/precompiles/werc20/testdata"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	utiltx "github.com/cosmos/evm/testutil/tx"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// -------------------------------------------------------------------------------------------------
// Integration test suite
// -------------------------------------------------------------------------------------------------

type PrecompileIntegrationTestSuite struct {
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     keyring.Keyring

	wrappedCoinDenom string

	// WERC20 precompile instance and configuration
	precompile        *werc20.Precompile
	precompileAddrHex string
}

func TestPrecompileIntegrationTestSuite(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	_ = DescribeTableSubtree("a user interact with the WEVMOS precompiled contract", func(chainId testconstants.ChainID) {
		var (
			is                                         *PrecompileIntegrationTestSuite
			passCheck, failCheck                       testutil.LogCheckArgs
			transferCheck, depositCheck, withdrawCheck testutil.LogCheckArgs

			callsData CallsData

			txSender, user keyring.Key

			revertContractAddr common.Address

			// Account balance tracking
			accountBalances      []*AccountBalanceInfo
			precisebankRemainder *big.Int
		)

		// Configure deposit amounts with integer and fractional components to test
		// precise balance handling across different decimal configurations
		var conversionFactor *big.Int
		switch chainId {
		case testconstants.SixDecimalsChainID:
			conversionFactor = big.NewInt(1e12) // For 6-decimal chains
		case testconstants.TwelveDecimalsChainID:
			conversionFactor = big.NewInt(1e6) // For 12-decimal chains
		default:
			conversionFactor = big.NewInt(1) // For 18-decimal chains
		}

		// Create deposit with 1000 integer units + fractional part
		depositAmount := big.NewInt(1000)
		depositAmount = depositAmount.Mul(depositAmount, conversionFactor)                                       // 1000 integer units
		depositFractional := new(big.Int).Div(new(big.Int).Mul(conversionFactor, big.NewInt(3)), big.NewInt(10)) // 0.3 * conversion factor as fractional
		depositAmount = depositAmount.Add(depositAmount, depositFractional)

		withdrawAmount := depositAmount
		transferAmount := big.NewInt(10) // Start with 10 integer units

		// Helper function to get account balance info by type
		balanceOf := func(accountType AccountType) *AccountBalanceInfo {
			return GetAccountBalance(accountBalances, accountType)
		}

		BeforeEach(func() {
			is = new(PrecompileIntegrationTestSuite)
			keyring := keyring.New(2)

			txSender = keyring.GetKey(0)
			user = keyring.GetKey(1)

			// Set the base fee to zero to allow for zero cost tx. The final gas cost is
			// not part of the logic tested here so this makes testing more easy.
			customGenesis := network.CustomGenesisState{}
			feemarketGenesis := feemarkettypes.DefaultGenesisState()
			feemarketGenesis.Params.NoBaseFee = true
			customGenesis[feemarkettypes.ModuleName] = feemarketGenesis

			// Reset evm config here for the standard case
			configurator := evmtypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			Expect(configurator.
				WithEVMCoinInfo(testconstants.ExampleChainCoinInfo[chainId]).
				Configure()).To(BeNil(), "expected no error setting the evm configurator")

			opts := []network.ConfigOption{
				network.WithChainID(chainId),
				network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
				network.WithCustomGenesis(customGenesis),
			}
			opts = append(opts, options...)
			integrationNetwork := network.NewUnitTestNetwork(create, opts...)
			grpcHandler := grpc.NewIntegrationHandler(integrationNetwork)
			txFactory := factory.New(integrationNetwork, grpcHandler)

			is.network = integrationNetwork
			is.factory = txFactory
			is.grpcHandler = grpcHandler
			is.keyring = keyring

			is.wrappedCoinDenom = evmtypes.GetEVMCoinDenom()
			is.precompileAddrHex = network.GetWEVMOSContractHex(testconstants.ChainID{
				ChainID:    is.network.GetChainID(),
				EVMChainID: is.network.GetEIP155ChainID().Uint64(),
			})

			ctx := integrationNetwork.GetContext()

			// Perform some check before adding the precompile to the suite.

			// Check that WEVMOS is part of the native precompiles.
			available := is.network.App.GetErc20Keeper().IsNativePrecompileAvailable(is.network.GetContext(), common.HexToAddress(is.precompileAddrHex))
			Expect(available).To(
				BeTrue(),
				"expected wevmos to be in the native precompiles",
			)
			_, found := is.network.App.GetBankKeeper().GetDenomMetaData(ctx, evmtypes.GetEVMCoinDenom())
			Expect(found).To(BeTrue(), "expected native token metadata to be registered")

			// Check that WEVMOS is registered in the token pairs map.
			tokenPairID := is.network.App.GetErc20Keeper().GetTokenPairID(ctx, is.wrappedCoinDenom)
			tokenPair, found := is.network.App.GetErc20Keeper().GetTokenPair(ctx, tokenPairID)
			Expect(found).To(BeTrue(), "expected wevmos precompile to be registered in the tokens map")
			Expect(tokenPair.Erc20Address).To(Equal(is.precompileAddrHex))

			precompileAddr := common.HexToAddress(is.precompileAddrHex)
			tokenPair = erc20types.NewTokenPair(
				precompileAddr,
				evmtypes.GetEVMCoinDenom(),
				erc20types.OWNER_MODULE,
			)

			precompile := werc20.NewPrecompile(
				tokenPair,
				is.network.App.GetBankKeeper(),
				is.network.App.GetErc20Keeper(),
				is.network.App.GetTransferKeeper(),
			)
			is.precompile = precompile

			// Setup of the contract calling into the precompile to tests revert
			// edge cases and proper handling of snapshots.
			revertCallerContract, err := testdata.LoadWEVMOS9TestCaller()
			Expect(err).ToNot(HaveOccurred(), "failed to load werc20 reverter caller contract")

			txArgs := evmtypes.EvmTxArgs{}
			txArgs.GasTipCap = new(big.Int).SetInt64(0)
			txArgs.GasLimit = 1_000_000_000_000
			revertContractAddr, err = is.factory.DeployContract(
				txSender.Priv,
				txArgs,
				testutiltypes.ContractDeploymentData{
					Contract: revertCallerContract,
					ConstructorArgs: []interface{}{
						common.HexToAddress(is.precompileAddrHex),
					},
				},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to deploy werc20 reverter contract")
			Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")

			// Support struct used to simplify transactions creation.
			callsData = CallsData{
				sender: txSender,

				precompileAddr: precompileAddr,
				precompileABI:  precompile.ABI,

				precompileReverterAddr: revertContractAddr,
				precompileReverterABI:  revertCallerContract.ABI,
			}

			// Utility types used to check the different events emitted.
			failCheck = testutil.LogCheckArgs{ABIEvents: is.precompile.Events}
			passCheck = failCheck.WithExpPass(true)
			withdrawCheck = passCheck.WithExpEvents(werc20.EventTypeWithdrawal)
			depositCheck = passCheck.WithExpEvents(werc20.EventTypeDeposit)
			transferCheck = passCheck.WithExpEvents(erc20.EventTypeTransfer)

			// Initialize and reset balance tracking state for each test
			accountBalances = InitializeAccountBalances(
				txSender.AccAddr, user.AccAddr,
				callsData.precompileAddr, revertContractAddr,
			)

			// Reset expected balance change of accounts
			ResetExpectedDeltas(accountBalances)
			precisebankRemainder = big.NewInt(0)
		})

		// JustBeforeEach takes snapshots after individual test setup
		JustBeforeEach(func() {
			TakeBalanceSnapshots(accountBalances, is.grpcHandler)
		})

		// AfterEach verifies balance changes
		AfterEach(func() {
			VerifyBalanceChanges(accountBalances, is.grpcHandler, precisebankRemainder)
		})

		Context("calling a specific wrapped coin method", func() {
			Context("and funds are part of the transaction", func() {
				When("the method is deposit", func() {
					It("it should return funds to sender and emit the event", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, werc20.DepositMethod)
						txArgs.Amount = depositAmount

						_, _, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
					})
					It("it should consume at least the deposit requested gas", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, werc20.DepositMethod)
						txArgs.Amount = depositAmount

						_, ethRes, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
						Expect(ethRes.GasUsed).To(BeNumerically(">=", werc20.DepositRequiredGas), "expected different gas used for deposit")
					})
				})
				//nolint:dupl
				When("no calldata is provided", func() {
					It("it should call the receive which behave like deposit", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, "")
						txArgs.Amount = depositAmount

						_, _, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
					})
					It("it should consume at least the deposit requested gas", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, werc20.DepositMethod)
						txArgs.Amount = depositAmount

						_, ethRes, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
						Expect(ethRes.GasUsed).To(BeNumerically(">=", werc20.DepositRequiredGas), "expected different gas used for receive")
					})
				})
				When("the specified method is too short", func() {
					It("it should call the fallback which behave like deposit", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, "")
						txArgs.Amount = depositAmount
						// Short method is directly set in the input to skip ABI validation
						txArgs.Input = []byte{1, 2, 3}

						_, _, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
					})
					It("it should consume at least the deposit requested gas", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, "")
						txArgs.Amount = depositAmount
						// Short method is directly set in the input to skip ABI validation
						txArgs.Input = []byte{1, 2, 3}

						_, ethRes, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
						Expect(ethRes.GasUsed).To(BeNumerically(">=", werc20.DepositRequiredGas), "expected different gas used for fallback")
					})
				})
				When("the specified method does not exist", func() {
					It("it should call the fallback which behave like deposit", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, "")
						txArgs.Amount = depositAmount
						// Wrong method is directly set in the input to skip ABI validation
						txArgs.Input = []byte("nonExistingMethod")

						_, _, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
					})
					It("it should consume at least the deposit requested gas", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, "")
						txArgs.Amount = depositAmount
						// Wrong method is directly set in the input to skip ABI validation
						txArgs.Input = []byte("nonExistingMethod")

						_, ethRes, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
						Expect(ethRes.GasUsed).To(BeNumerically(">=", werc20.DepositRequiredGas), "expected different gas used for fallback")
					})
				})
			})
			Context("and funds are NOT part of the transaction", func() {
				When("the method is withdraw", func() {
					It("it should fail if user doesn't have enough funds", func() {
						newUserAcc, newUserPriv := utiltx.NewAccAddressAndKey()
						newUserBalance := sdk.Coins{sdk.Coin{
							Denom:  evmtypes.GetEVMCoinDenom(),
							Amount: math.NewIntFromBigInt(withdrawAmount).Quo(precisebanktypes.ConversionFactor()).SubRaw(1),
						}}
						err := is.network.App.GetBankKeeper().SendCoins(is.network.GetContext(), user.AccAddr, newUserAcc, newUserBalance)
						Expect(err).ToNot(HaveOccurred(), "expected no error sending tokens")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")

						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, werc20.WithdrawMethod, withdrawAmount)

						_, _, err = is.factory.CallContractAndCheckLogs(newUserPriv, txArgs, callArgs, withdrawCheck)
						Expect(err).To(HaveOccurred(), "expected an error because not enough funds")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
					})
					It("it should be a no-op and emit the event", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, werc20.WithdrawMethod, withdrawAmount)

						_, _, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, withdrawCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
					})
					It("it should consume at least the withdraw requested gas", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, werc20.WithdrawMethod, withdrawAmount)

						_, ethRes, _ := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, withdrawCheck)
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
						Expect(ethRes.GasUsed).To(BeNumerically(">=", werc20.WithdrawRequiredGas), "expected different gas used for withdraw")
					})
				})
				//nolint:dupl
				When("no calldata is provided", func() {
					It("it should call the fallback which behave like deposit", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, "")
						txArgs.Amount = depositAmount

						_, _, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
					})
					It("it should consume at least the deposit requested gas", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, werc20.DepositMethod)
						txArgs.Amount = depositAmount

						_, ethRes, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
						Expect(ethRes.GasUsed).To(BeNumerically(">=", werc20.DepositRequiredGas), "expected different gas used for receive")
					})
				})
				When("the specified method is too short", func() {
					It("it should call the fallback which behave like deposit", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, "")
						txArgs.Amount = depositAmount
						// Short method is directly set in the input to skip ABI validation
						txArgs.Input = []byte{1, 2, 3}

						_, _, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
					})
					It("it should consume at least the deposit requested gas", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, "")
						txArgs.Amount = depositAmount
						// Short method is directly set in the input to skip ABI validation
						txArgs.Input = []byte{1, 2, 3}

						_, ethRes, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
						Expect(ethRes.GasUsed).To(BeNumerically(">=", werc20.DepositRequiredGas), "expected different gas used for fallback")
					})
				})
				When("the specified method does not exist", func() {
					It("it should call the fallback which behave like deposit", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, "")
						txArgs.Amount = depositAmount
						// Wrong method is directly set in the input to skip ABI validation
						txArgs.Input = []byte("nonExistingMethod")

						_, _, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
					})
					It("it should consume at least the deposit requested gas", func() {
						txArgs, callArgs := callsData.getTxAndCallArgs(directCall, "")
						txArgs.Amount = depositAmount
						// Wrong method is directly set in the input to skip ABI validation
						txArgs.Input = []byte("nonExistingMethod")

						_, ethRes, err := is.factory.CallContractAndCheckLogs(user.Priv, txArgs, callArgs, depositCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
						Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
						Expect(ethRes.GasUsed).To(BeNumerically(">=", werc20.DepositRequiredGas), "expected different gas used for fallback")
					})
				})
			})
		})
		Context("calling a reverter contract", func() {
			When("to call the deposit", func() {
				It("it should return funds to the last sender and emit the event", func() {
					borrow := big.NewInt(0)
					if conversionFactor.Cmp(big.NewInt(1)) != 0 { // 18-decimal chain (conversionFactor = 1)
						borrow = big.NewInt(1)
					}

					balanceOf(Sender).IntegerDelta = new(big.Int).Sub(new(big.Int).Neg((new(big.Int).Quo(depositAmount, conversionFactor))), borrow)
					balanceOf(Sender).FractionalDelta = new(big.Int).Mod(new(big.Int).Sub(conversionFactor, depositFractional), conversionFactor)

					balanceOf(Contract).IntegerDelta = new(big.Int).Quo(depositAmount, conversionFactor)
					balanceOf(Contract).FractionalDelta = depositFractional

					balanceOf(PrecisebankModule).IntegerDelta = borrow

					txArgs, callArgs := callsData.getTxAndCallArgs(contractCall, "depositWithRevert", false, false)
					txArgs.Amount = depositAmount

					_, _, err := is.factory.CallContractAndCheckLogs(txSender.Priv, txArgs, callArgs, depositCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected error calling the precompile")
					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
				})
			})
			DescribeTable("to call the deposit", func(before, after bool) {
				txArgs, callArgs := callsData.getTxAndCallArgs(contractCall, "depositWithRevert", before, after)
				txArgs.Amount = depositAmount

				_, _, err := is.factory.CallContractAndCheckLogs(txSender.Priv, txArgs, callArgs, depositCheck)
				Expect(err).To(HaveOccurred(), "execution should have reverted")
				Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
			},
				Entry("it should not move funds and dont emit the event reverting before changing state", true, false),
				Entry("it should not move funds and dont emit the event reverting after changing state", false, true),
			)
		})
		Context("calling an erc20 method", func() {
			When("transferring tokens", func() {
				It("it should transfer tokens to a receiver using `transfer`", func() {
					balanceOf(Sender).IntegerDelta = new(big.Int).Neg(transferAmount)
					balanceOf(Sender).FractionalDelta = big.NewInt(0)
					balanceOf(Receiver).IntegerDelta = transferAmount
					balanceOf(Receiver).FractionalDelta = big.NewInt(0)

					// First, sender needs to deposit to get WERC20 tokens
					// Use a larger deposit amount to ensure sufficient balance for transfer
					depositForTransfer := new(big.Int).Mul(transferAmount, big.NewInt(10)) // 10x transfer amount
					txArgs, callArgs := callsData.getTxAndCallArgs(directCall, werc20.DepositMethod)
					txArgs.Amount = depositForTransfer
					_, _, err := is.factory.CallContractAndCheckLogs(txSender.Priv, txArgs, callArgs, depositCheck)
					Expect(err).ToNot(HaveOccurred(), "failed to deposit before transfer")
					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock after deposit")

					// Now perform the transfer
					txArgs, transferArgs := callsData.getTxAndCallArgs(directCall, erc20.TransferMethod, user.Addr, transferAmount)

					_, _, err = is.factory.CallContractAndCheckLogs(txSender.Priv, txArgs, transferArgs, transferCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock after transfer")
				})
				It("it should fail to transfer tokens to a receiver using `transferFrom`", func() {
					txArgs, transferArgs := callsData.getTxAndCallArgs(directCall, erc20.TransferFromMethod, txSender.Addr, user.Addr, transferAmount)

					insufficientAllowanceCheck := failCheck.WithErrContains(erc20.ErrInsufficientAllowance.Error())
					_, _, err := is.factory.CallContractAndCheckLogs(txSender.Priv, txArgs, transferArgs, insufficientAllowanceCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")
					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock after transfer")
				})
			})
			When("querying information", func() {
				Context("to retrieve a balance", func() {
					It("should return the correct balance for an existing account", func() {
						// Query the balance
						txArgs, balancesArgs := callsData.getTxAndCallArgs(directCall, erc20.BalanceOfMethod, txSender.Addr)

						_, ethRes, err := is.factory.CallContractAndCheckLogs(txSender.Priv, txArgs, balancesArgs, passCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						// Get expected balance using grpcHandler for accurate state
						expBalanceRes, err := is.grpcHandler.GetBalanceFromBank(txSender.AccAddr, is.wrappedCoinDenom)
						Expect(err).ToNot(HaveOccurred(), "failed to get balance from grpcHandler")

						var balance *big.Int
						err = is.precompile.UnpackIntoInterface(&balance, erc20.BalanceOfMethod, ethRes.Ret)
						Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
						Expect(balance).To(Equal(expBalanceRes.Balance.Amount.BigInt()), "expected different balance")
					})
					It("should return 0 for a new account", func() {
						// Query the balance
						txArgs, balancesArgs := callsData.getTxAndCallArgs(directCall, erc20.BalanceOfMethod, utiltx.GenerateAddress())

						_, ethRes, err := is.factory.CallContractAndCheckLogs(txSender.Priv, txArgs, balancesArgs, passCheck)
						Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

						var balance *big.Int
						err = is.precompile.UnpackIntoInterface(&balance, erc20.BalanceOfMethod, ethRes.Ret)
						Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
						Expect(balance.Int64()).To(Equal(int64(0)), "expected different balance")
					})
				})
				It("should return the correct name", func() {
					txArgs, nameArgs := callsData.getTxAndCallArgs(directCall, erc20.NameMethod)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(txSender.Priv, txArgs, nameArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var name string
					err = is.precompile.UnpackIntoInterface(&name, erc20.NameMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(name).To(ContainSubstring("Cosmos EVM"), "expected different name")
				})

				It("should return the correct symbol", func() {
					txArgs, symbolArgs := callsData.getTxAndCallArgs(directCall, erc20.SymbolMethod)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(txSender.Priv, txArgs, symbolArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var symbol string
					err = is.precompile.UnpackIntoInterface(&symbol, erc20.SymbolMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")
					Expect(symbol).To(ContainSubstring("ATOM"), "expected different symbol")
				})

				It("should return the decimals", func() {
					txArgs, decimalsArgs := callsData.getTxAndCallArgs(directCall, erc20.DecimalsMethod)

					_, ethRes, err := is.factory.CallContractAndCheckLogs(txSender.Priv, txArgs, decimalsArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var decimals uint8
					err = is.precompile.UnpackIntoInterface(&decimals, erc20.DecimalsMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack result")

					coinInfo := testconstants.ExampleChainCoinInfo[testconstants.ChainID{
						ChainID:    is.network.GetChainID(),
						EVMChainID: is.network.GetEIP155ChainID().Uint64(),
					}]
					Expect(decimals).To(Equal(uint8(coinInfo.Decimals)), "expected different decimals") //nolint:gosec // G115
				},
				)
			})
		})
	},
		Entry("6 decimals chain", testconstants.SixDecimalsChainID),
		Entry("12 decimals chain", testconstants.TwelveDecimalsChainID),
		Entry("18 decimals chain", testconstants.ExampleChainID),
	)

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "WEVMOS precompile test suite")
}
