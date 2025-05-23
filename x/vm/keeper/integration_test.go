package keeper_test

import (
	factory2 "github.com/cosmos/evm/testutil/integration/common/factory"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	abcitypes "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	testkeyring "github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"
	integrationutils "github.com/cosmos/evm/testutil/integration/os/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"
	"cosmossdk.io/x/feegrant"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
)

type IntegrationTestSuite struct {
	network     network.Network
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     testkeyring.Keyring
}

func TestVMIntegrationTestSuite(t *testing.T) {
	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keeper Suite")
}

// This test suite is meant to test the EVM module in the context of the ATOM.
// It uses the integration test framework to spin up a local ATOM network and
// perform transactions on it.
// The test suite focus on testing how the MsgEthereumTx message is handled under the
// different params configuration of the module while testing the different Tx types
// Ethereum supports (LegacyTx, AccessListTx, DynamicFeeTx) and the different types of
// transactions (transfer, contract deployment, contract call).
// Note that more in depth testing of the EVM and solidity execution is done through the
// hardhat and the nix setup.
var _ = Describe("Handling a MsgEthereumTx message", Label("EVM"), Ordered, func() {
	var s *IntegrationTestSuite

	BeforeAll(func() {
		keyring := testkeyring.New(4)
		integrationNetwork := network.New(
			network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		)
		grpcHandler := grpc.NewIntegrationHandler(integrationNetwork)
		txFactory := factory.New(integrationNetwork, grpcHandler)
		s = &IntegrationTestSuite{
			network:     integrationNetwork,
			factory:     txFactory,
			grpcHandler: grpcHandler,
			keyring:     keyring,
		}
	})

	AfterEach(func() {
		// Start each test with a fresh block
		err := s.network.NextBlock()
		Expect(err).To(BeNil())
	})

	When("the params have default values", Ordered, func() {
		BeforeAll(func() {
			// Set params to default values
			defaultParams := evmtypes.DefaultParams()
			err := integrationutils.UpdateEvmParams(
				integrationutils.UpdateParamsInput{
					Tf:      s.factory,
					Network: s.network,
					Pk:      s.keyring.GetPrivKey(0),
					Params:  defaultParams,
				},
			)
			Expect(err).To(BeNil())
		})
		DescribeTable("Executes a transfer transaction", func(getTxArgs func() evmtypes.EvmTxArgs) {
			senderKey := s.keyring.GetKey(0)
			receiverKey := s.keyring.GetKey(1)
			denom := s.network.GetBaseDenom()

			senderPrevBalanceResponse, err := s.grpcHandler.GetBalanceFromBank(senderKey.AccAddr, denom)
			Expect(err).To(BeNil())
			senderPrevBalance := senderPrevBalanceResponse.GetBalance().Amount

			receiverPrevBalanceResponse, err := s.grpcHandler.GetBalanceFromBank(receiverKey.AccAddr, denom)
			Expect(err).To(BeNil())
			receiverPrevBalance := receiverPrevBalanceResponse.GetBalance().Amount

			transferAmount := int64(1000)

			// Taking custom args from the table entry
			txArgs := getTxArgs()
			txArgs.Amount = big.NewInt(transferAmount)
			txArgs.To = &receiverKey.Addr

			res, err := s.factory.ExecuteEthTx(senderKey.Priv, txArgs)
			Expect(err).To(BeNil())
			Expect(res.IsOK()).To(Equal(true), "transaction should have succeeded", res.GetLog())

			err = s.network.NextBlock()
			Expect(err).To(BeNil())

			// Check sender balance after transaction
			senderBalanceResultBeforeFees := senderPrevBalance.Sub(math.NewInt(transferAmount))
			senderAfterBalance, err := s.grpcHandler.GetBalanceFromBank(senderKey.AccAddr, denom)
			Expect(err).To(BeNil())
			Expect(senderAfterBalance.GetBalance().Amount.LTE(senderBalanceResultBeforeFees)).To(BeTrue())

			// Check receiver balance after transaction
			receiverBalanceResult := receiverPrevBalance.Add(math.NewInt(transferAmount))
			receverAfterBalanceResponse, err := s.grpcHandler.GetBalanceFromBank(receiverKey.AccAddr, denom)
			Expect(err).To(BeNil())
			Expect(receverAfterBalanceResponse.GetBalance().Amount).To(Equal(receiverBalanceResult))
		},
			Entry("as a DynamicFeeTx", func() evmtypes.EvmTxArgs { return evmtypes.EvmTxArgs{} }),
			Entry("as an AccessListTx",
				func() evmtypes.EvmTxArgs {
					return evmtypes.EvmTxArgs{
						Accesses: &ethtypes.AccessList{{
							Address:     s.keyring.GetAddr(1),
							StorageKeys: []common.Hash{{0}},
						}},
					}
				},
			),
			Entry("as a LegacyTx", func() evmtypes.EvmTxArgs {
				return evmtypes.EvmTxArgs{
					GasPrice: big.NewInt(1e9),
				}
			}),
		)

		DescribeTable("Executes a contract deployment", func(getTxArgs func() evmtypes.EvmTxArgs) {
			// Deploy contract
			senderPriv := s.keyring.GetPrivKey(0)
			constructorArgs := []interface{}{"coin", "token", uint8(18)}
			compiledContract := contracts.ERC20MinterBurnerDecimalsContract

			txArgs := getTxArgs()
			contractAddr, err := s.factory.DeployContract(
				senderPriv,
				txArgs,
				factory.ContractDeploymentData{
					Contract:        compiledContract,
					ConstructorArgs: constructorArgs,
				},
			)
			Expect(err).To(BeNil())
			Expect(contractAddr).ToNot(Equal(common.Address{}))

			err = s.network.NextBlock()
			Expect(err).To(BeNil())

			// Check contract account got created correctly
			contractBechAddr := sdktypes.AccAddress(contractAddr.Bytes()).String()
			contractAccount, err := s.grpcHandler.GetAccount(contractBechAddr)
			Expect(err).To(BeNil())
			Expect(contractAccount).ToNot(BeNil(), "expected account to be retrievable via auth query")

			ethAccountRes, err := s.grpcHandler.GetEvmAccount(contractAddr)
			Expect(err).To(BeNil(), "expected no error retrieving account from the state db")
			Expect(ethAccountRes.CodeHash).ToNot(Equal(common.BytesToHash(evmtypes.EmptyCodeHash).Hex()),
				"expected code hash not to be the empty code hash",
			)
		},
			Entry("as a DynamicFeeTx", func() evmtypes.EvmTxArgs { return evmtypes.EvmTxArgs{} }),
			Entry("as an AccessListTx",
				func() evmtypes.EvmTxArgs {
					return evmtypes.EvmTxArgs{
						Accesses: &ethtypes.AccessList{{
							Address:     s.keyring.GetAddr(1),
							StorageKeys: []common.Hash{{0}},
						}},
					}
				},
			),
			Entry("as a LegacyTx", func() evmtypes.EvmTxArgs {
				return evmtypes.EvmTxArgs{
					GasPrice: big.NewInt(1e9),
				}
			}),
		)

		Context("With a predeployed ERC20MinterBurnerDecimalsContract", func() {
			var contractAddr common.Address

			BeforeEach(func() {
				// Deploy contract
				senderPriv := s.keyring.GetPrivKey(0)
				constructorArgs := []interface{}{"coin", "token", uint8(18)}
				compiledContract := contracts.ERC20MinterBurnerDecimalsContract

				var err error // Avoid shadowing
				contractAddr, err = s.factory.DeployContract(
					senderPriv,
					evmtypes.EvmTxArgs{}, // Default values
					factory.ContractDeploymentData{
						Contract:        compiledContract,
						ConstructorArgs: constructorArgs,
					},
				)
				Expect(err).To(BeNil())
				Expect(contractAddr).ToNot(Equal(common.Address{}))

				err = s.network.NextBlock()
				Expect(err).To(BeNil())
			})

			DescribeTable("Executes a contract call", func(getTxArgs func() evmtypes.EvmTxArgs) {
				senderPriv := s.keyring.GetPrivKey(0)
				compiledContract := contracts.ERC20MinterBurnerDecimalsContract
				recipientKey := s.keyring.GetKey(1)

				// Execute contract call
				mintTxArgs := getTxArgs()
				mintTxArgs.To = &contractAddr

				amountToMint := big.NewInt(1e18)
				mintArgs := factory.CallArgs{
					ContractABI: compiledContract.ABI,
					MethodName:  "mint",
					Args:        []interface{}{recipientKey.Addr, amountToMint},
				}
				mintResponse, err := s.factory.ExecuteContractCall(senderPriv, mintTxArgs, mintArgs)
				Expect(err).To(BeNil())
				Expect(mintResponse.IsOK()).To(Equal(true), "transaction should have succeeded", mintResponse.GetLog())

				err = checkMintTopics(mintResponse)
				Expect(err).To(BeNil())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				totalSupplyTxArgs := evmtypes.EvmTxArgs{
					To: &contractAddr,
				}
				totalSupplyArgs := factory.CallArgs{
					ContractABI: compiledContract.ABI,
					MethodName:  "totalSupply",
					Args:        []interface{}{},
				}
				totalSupplyRes, err := s.factory.ExecuteContractCall(senderPriv, totalSupplyTxArgs, totalSupplyArgs)
				Expect(err).To(BeNil())
				Expect(totalSupplyRes.IsOK()).To(Equal(true), "transaction should have succeeded", totalSupplyRes.GetLog())

				var totalSupplyResponse *big.Int
				err = integrationutils.DecodeContractCallResponse(&totalSupplyResponse, totalSupplyArgs, totalSupplyRes)
				Expect(err).To(BeNil())
				Expect(totalSupplyResponse).To(Equal(amountToMint))
			},
				Entry("as a DynamicFeeTx", func() evmtypes.EvmTxArgs { return evmtypes.EvmTxArgs{} }),
				Entry("as an AccessListTx",
					func() evmtypes.EvmTxArgs {
						return evmtypes.EvmTxArgs{
							Accesses: &ethtypes.AccessList{{
								Address:     s.keyring.GetAddr(1),
								StorageKeys: []common.Hash{{0}},
							}},
						}
					},
				),
				Entry("as a LegacyTx", func() evmtypes.EvmTxArgs {
					return evmtypes.EvmTxArgs{
						GasPrice: big.NewInt(1e9),
					}
				}),
			)
		})

		It("should fail when ChainID is wrong", func() {
			senderPriv := s.keyring.GetPrivKey(0)
			receiver := s.keyring.GetKey(1)
			txArgs := evmtypes.EvmTxArgs{
				To:      &receiver.Addr,
				Amount:  big.NewInt(1000),
				ChainID: big.NewInt(1),
			}

			res, err := s.factory.ExecuteEthTx(senderPriv, txArgs)
			Expect(err).NotTo(BeNil())
			Expect(err.Error()).To(ContainSubstring("invalid chain id"))
			// Transaction fails before being broadcasted
			Expect(res).To(Equal(abcitypes.ExecTxResult{}))
		})
	})

	//DescribeTable("Performs transfer and contract call", func(getTestParams func() evmtypes.Params, transferParams, contractCallParams PermissionsTableTest) {
	//	params := getTestParams()
	//	err := integrationutils.UpdateEvmParams(
	//		integrationutils.UpdateParamsInput{
	//			Tf:      s.factory,
	//			Network: s.network,
	//			Pk:      s.keyring.GetPrivKey(0),
	//			Params:  params,
	//		},
	//	)
	//	Expect(err).To(BeNil())
	//
	//	err = s.network.NextBlock()
	//	Expect(err).To(BeNil())
	//
	//	signer := s.keyring.GetKey(transferParams.SignerIndex)
	//	receiver := s.keyring.GetKey(1)
	//	txArgs := evmtypes.EvmTxArgs{
	//		To:     &receiver.Addr,
	//		Amount: big.NewInt(1000),
	//		// Hard coded gas limit to avoid failure on gas estimation because
	//		// of the param
	//		GasLimit: 100000,
	//	}
	//	res, err := s.factory.ExecuteEthTx(signer.Priv, txArgs)
	//	if transferParams.ExpFail {
	//		Expect(err).NotTo(BeNil())
	//		Expect(err.Error()).To(ContainSubstring("does not have permission to perform a call"))
	//	} else {
	//		Expect(err).To(BeNil())
	//		Expect(res.IsOK()).To(Equal(true), "transaction should have succeeded", res.GetLog())
	//	}
	//
	//	senderKey := s.keyring.GetKey(contractCallParams.SignerIndex)
	//	contractAddress := common.HexToAddress(evmtypes.StakingPrecompileAddress)
	//	validatorAddress := s.network.GetValidators()[1].OperatorAddress
	//	contractABI, err := staking.LoadABI()
	//	Expect(err).To(BeNil())
	//
	//	// If grpc query fails, that means there were no previous delegations
	//	prevDelegation := big.NewInt(0)
	//	prevDelegationRes, err := s.grpcHandler.GetDelegation(senderKey.AccAddr.String(), validatorAddress)
	//	if err == nil {
	//		prevDelegation = prevDelegationRes.DelegationResponse.Balance.Amount.BigInt()
	//	}
	//
	//	amountToDelegate := big.NewInt(200)
	//	totalSupplyTxArgs := evmtypes.EvmTxArgs{
	//		To: &contractAddress,
	//	}
	//
	//	// Perform a delegate transaction to the staking precompile
	//	delegateArgs := factory.CallArgs{
	//		ContractABI: contractABI,
	//		MethodName:  staking.DelegateMethod,
	//		Args:        []interface{}{senderKey.Addr, validatorAddress, amountToDelegate},
	//	}
	//	delegateResponse, err := s.factory.ExecuteContractCall(senderKey.Priv, totalSupplyTxArgs, delegateArgs)
	//	if contractCallParams.ExpFail {
	//		Expect(err).NotTo(BeNil())
	//		Expect(err.Error()).To(ContainSubstring("does not have permission to perform a call"))
	//	} else {
	//		Expect(err).To(BeNil())
	//		Expect(delegateResponse.IsOK()).To(Equal(true), "transaction should have succeeded", delegateResponse.GetLog())
	//
	//		err = s.network.NextBlock()
	//		Expect(err).To(BeNil())
	//
	//		// Perform query to check the delegation was successful
	//		queryDelegationArgs := factory.CallArgs{
	//			ContractABI: contractABI,
	//			MethodName:  staking.DelegationMethod,
	//			Args:        []interface{}{senderKey.Addr, validatorAddress},
	//		}
	//		queryDelegationResponse, err := s.factory.ExecuteContractCall(senderKey.Priv, totalSupplyTxArgs, queryDelegationArgs)
	//		Expect(err).To(BeNil())
	//		Expect(queryDelegationResponse.IsOK()).To(Equal(true), "transaction should have succeeded", queryDelegationResponse.GetLog())
	//
	//		// Make sure the delegation amount is correct
	//		var delegationOutput staking.DelegationOutput
	//		err = integrationutils.DecodeContractCallResponse(&delegationOutput, queryDelegationArgs, queryDelegationResponse)
	//		Expect(err).To(BeNil())
	//
	//		expectedDelegationAmt := amountToDelegate.Add(amountToDelegate, prevDelegation)
	//		Expect(delegationOutput.Balance.Amount.String()).To(Equal(expectedDelegationAmt.String()))
	//	}
	//},
	//	// Entry("transfer and call fail with CALL permission policy set to restricted", func() evmtypes.Params {
	//	// 	// Set params to default values
	//	// 	defaultParams := evmtypes.DefaultParams()
	//	// 	defaultParams.AccessControl.Call = evmtypes.AccessControlType{
	//	// 		AccessType:        evmtypes.AccessTypeRestricted,
	//	// 	}
	//	// 	return defaultParams
	//	// },
	//	// 	OpcodeTestTable{ExpFail: true, SignerIndex: 0},
	//	// 	OpcodeTestTable{ExpFail: true, SignerIndex: 0},
	//	// ),
	//	Entry("transfer and call succeed with CALL permission policy set to default and CREATE permission policy set to restricted", func() evmtypes.Params {
	//		blockedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Create = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypeRestricted,
	//			AccessControlList: []string{s.keyring.GetAddr(blockedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//	),
	//	Entry("transfer and call are successful with CALL permission policy set to permissionless and address not blocked", func() evmtypes.Params {
	//		blockedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Call = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissionless,
	//			AccessControlList: []string{s.keyring.GetAddr(blockedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//	),
	//	Entry("transfer fails with signer blocked and call succeeds with signer NOT blocked permission policy set to permissionless", func() evmtypes.Params {
	//		blockedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Call = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissionless,
	//			AccessControlList: []string{s.keyring.GetAddr(blockedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: true, SignerIndex: 1},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//	),
	//	Entry("transfer succeeds with signer NOT blocked and call fails with signer blocked permission policy set to permissionless", func() evmtypes.Params {
	//		blockedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Call = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissionless,
	//			AccessControlList: []string{s.keyring.GetAddr(blockedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//		PermissionsTableTest{ExpFail: true, SignerIndex: 1},
	//	),
	//	Entry("transfer and call succeeds with CALL permission policy set to permissioned and signer whitelisted on both", func() evmtypes.Params {
	//		blockedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Call = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissioned,
	//			AccessControlList: []string{s.keyring.GetAddr(blockedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 1},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 1},
	//	),
	//	Entry("transfer and call fails with CALL permission policy set to permissioned and signer not whitelisted on both", func() evmtypes.Params {
	//		blockedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Call = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissioned,
	//			AccessControlList: []string{s.keyring.GetAddr(blockedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: true, SignerIndex: 0},
	//		PermissionsTableTest{ExpFail: true, SignerIndex: 0},
	//	),
	//)
	//
	//DescribeTable("Performs contract deployment and contract call with AccessControl", func(getTestParams func() evmtypes.Params, createParams, callParams PermissionsTableTest) {
	//	params := getTestParams()
	//	err := integrationutils.UpdateEvmParams(
	//		integrationutils.UpdateParamsInput{
	//			Tf:      s.factory,
	//			Network: s.network,
	//			Pk:      s.keyring.GetPrivKey(0),
	//			Params:  params,
	//		},
	//	)
	//	Expect(err).To(BeNil())
	//
	//	err = s.network.NextBlock()
	//	Expect(err).To(BeNil())
	//
	//	createSigner := s.keyring.GetPrivKey(createParams.SignerIndex)
	//	constructorArgs := []interface{}{"coin", "token", uint8(18)}
	//	compiledContract := contracts.ERC20MinterBurnerDecimalsContract
	//
	//	contractAddr, err := s.factory.DeployContract(
	//		createSigner,
	//		evmtypes.EvmTxArgs{}, // Default values
	//		factory.ContractDeploymentData{
	//			Contract:        compiledContract,
	//			ConstructorArgs: constructorArgs,
	//		},
	//	)
	//	if createParams.ExpFail {
	//		Expect(err).NotTo(BeNil())
	//		Expect(err.Error()).To(ContainSubstring("does not have permission to deploy contracts"))
	//		// If contract deployment is expected to fail, we can skip the rest of the test
	//		return
	//	}
	//
	//	Expect(err).To(BeNil())
	//	Expect(contractAddr).ToNot(Equal(common.Address{}))
	//
	//	err = s.network.NextBlock()
	//	Expect(err).To(BeNil())
	//
	//	callSigner := s.keyring.GetPrivKey(callParams.SignerIndex)
	//	totalSupplyTxArgs := evmtypes.EvmTxArgs{
	//		To: &contractAddr,
	//	}
	//	totalSupplyArgs := factory.CallArgs{
	//		ContractABI: compiledContract.ABI,
	//		MethodName:  "totalSupply",
	//		Args:        []interface{}{},
	//	}
	//	res, err := s.factory.ExecuteContractCall(callSigner, totalSupplyTxArgs, totalSupplyArgs)
	//	if callParams.ExpFail {
	//		Expect(err).NotTo(BeNil())
	//		Expect(err.Error()).To(ContainSubstring("does not have permission to perform a call"))
	//	} else {
	//		Expect(err).To(BeNil())
	//		Expect(res.IsOK()).To(Equal(true), "transaction should have succeeded", res.GetLog())
	//	}
	//},
	//	Entry("Create and call is successful with create permission policy set to permissionless and address not blocked ", func() evmtypes.Params {
	//		blockedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Create = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissionless,
	//			AccessControlList: []string{s.keyring.GetAddr(blockedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//	),
	//	Entry("Create fails with create permission policy set to permissionless and signer is blocked ", func() evmtypes.Params {
	//		blockedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Create = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissionless,
	//			AccessControlList: []string{s.keyring.GetAddr(blockedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: true, SignerIndex: 1},
	//		PermissionsTableTest{}, // Call should not be executed
	//	),
	//	Entry("Create and call is successful with call permission policy set to permissionless and address not blocked ", func() evmtypes.Params {
	//		blockedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Call = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissionless,
	//			AccessControlList: []string{s.keyring.GetAddr(blockedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//	),
	//	Entry("Create is successful and call fails with call permission policy set to permissionless and address blocked ", func() evmtypes.Params {
	//		blockedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Call = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissionless,
	//			AccessControlList: []string{s.keyring.GetAddr(blockedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//		PermissionsTableTest{ExpFail: true, SignerIndex: 1},
	//	),
	//	Entry("Create fails create permission policy set to restricted", func() evmtypes.Params {
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Create = evmtypes.AccessControlType{
	//			AccessType: evmtypes.AccessTypeRestricted,
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: true, SignerIndex: 0},
	//		PermissionsTableTest{}, // Call should not be executed
	//	),
	//	Entry("Create succeeds and call fails when call permission policy set to restricted", func() evmtypes.Params {
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Call = evmtypes.AccessControlType{
	//			AccessType: evmtypes.AccessTypeRestricted,
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//		PermissionsTableTest{ExpFail: true, SignerIndex: 0},
	//	),
	//	Entry("Create and call are successful with create permission policy set to permissioned and signer whitelisted", func() evmtypes.Params {
	//		whitelistedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Create = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissioned,
	//			AccessControlList: []string{s.keyring.GetAddr(whitelistedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 1},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//	),
	//	Entry("Create fails with create permission policy set to permissioned and signer NOT whitelisted", func() evmtypes.Params {
	//		whitelistedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Create = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissioned,
	//			AccessControlList: []string{s.keyring.GetAddr(whitelistedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: true, SignerIndex: 0},
	//		PermissionsTableTest{},
	//	),
	//	Entry("Create and call are successful with call permission policy set to permissioned and signer whitelisted", func() evmtypes.Params {
	//		whitelistedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Call = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissioned,
	//			AccessControlList: []string{s.keyring.GetAddr(whitelistedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 1},
	//	),
	//	Entry("Create succeeds and call fails with call permission policy set to permissioned and signer NOT whitelisted", func() evmtypes.Params {
	//		whitelistedSignerIndex := 1
	//		// Set params to default values
	//		defaultParams := evmtypes.DefaultParams()
	//		defaultParams.AccessControl.Call = evmtypes.AccessControlType{
	//			AccessType:        evmtypes.AccessTypePermissioned,
	//			AccessControlList: []string{s.keyring.GetAddr(whitelistedSignerIndex).String()},
	//		}
	//		return defaultParams
	//	},
	//		PermissionsTableTest{ExpFail: false, SignerIndex: 0},
	//		PermissionsTableTest{ExpFail: true, SignerIndex: 0},
	//	),
	//)

	// ==========================================
	// Account Migration Integration Tests
	// ==========================================

	When("testing account migration functionality", Label("Migration"), Ordered, func() {
		BeforeAll(func() {
			keyring := testkeyring.New(8)
			integrationNetwork := network.New(
				network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
			)
			grpcHandler := grpc.NewIntegrationHandler(integrationNetwork)
			txFactory := factory.New(integrationNetwork, grpcHandler)
			s = &IntegrationTestSuite{
				network:     integrationNetwork,
				factory:     txFactory,
				grpcHandler: grpcHandler,
				keyring:     keyring,
			}
		})

		Context("Bank Token Migration", func() {
			It("should successfully migrate bank tokens between accounts", func() {
				originalKey := s.keyring.GetKey(0)
				newKey := s.keyring.GetKey(1)
				denom := s.network.GetBaseDenom()

				// Get initial balances
				originalBalance, err := s.grpcHandler.GetBalanceFromBank(originalKey.AccAddr, denom)
				Expect(err).To(BeNil())
				originalAmount := originalBalance.GetBalance().Amount

				newBalance, err := s.grpcHandler.GetBalanceFromBank(newKey.AccAddr, denom)
				Expect(err).To(BeNil())
				newAmount := newBalance.GetBalance().Amount

				// Execute migration
				msg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: originalKey.AccAddr.String(),
					NewAddress:      newKey.AccAddr.String(),
				}

				res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{msg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "migration should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify balances after migration
				originalBalanceAfter, err := s.grpcHandler.GetBalanceFromBank(originalKey.AccAddr, denom)
				Expect(err).To(BeNil())
				Expect(originalBalanceAfter.GetBalance().Amount.IsZero()).To(BeTrue(), "original account should have zero balance")

				newBalanceAfter, err := s.grpcHandler.GetBalanceFromBank(newKey.AccAddr, denom)
				Expect(err).To(BeNil())
				expectedNewBalance := newAmount.Add(originalAmount)
				Expect(newBalanceAfter.GetBalance().Amount).To(Equal(expectedNewBalance), "new account should have combined balance")
			})

			It("should handle multiple denominations correctly", func() {
				originalKey := s.keyring.GetKey(2)
				newKey := s.keyring.GetKey(3)

				// Execute migration
				msg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: originalKey.AccAddr.String(),
					NewAddress:      newKey.AccAddr.String(),
				}

				res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{msg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "migration should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify all denominations were transferred
				balances, err := s.grpcHandler.GetAllBalances(newKey.AccAddr)
				Expect(err).To(BeNil())
				Expect(len(balances.GetBalances())).To(BeNumerically(">=", 1))
			})
		})

		Context("Delegation Migration", func() {
			It("should successfully migrate delegations", func() {
				originalKey := s.keyring.GetKey(0)
				newKey := s.keyring.GetKey(1)
				validators := s.network.GetValidators()
				Expect(len(validators)).To(BeNumerically(">=", 1))

				validatorAddr := validators[0].OperatorAddress
				delegationAmount := math.NewInt(1000000) // 1 token with 6 decimals

				// Create delegation from original account
				delegateMsg := &stakingtypes.MsgDelegate{
					DelegatorAddress: originalKey.AccAddr.String(),
					ValidatorAddress: validatorAddr,
					Amount:           sdktypes.NewCoin(s.network.GetBaseDenom(), delegationAmount),
				}

				res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{delegateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "delegation should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify delegation exists
				delegation, err := s.grpcHandler.GetDelegation(originalKey.AccAddr.String(), validatorAddr)
				Expect(err).To(BeNil())
				Expect(delegation.DelegationResponse.Balance.Amount).To(Equal(delegationAmount))

				// Execute migration
				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: originalKey.AccAddr.String(),
					NewAddress:      newKey.AccAddr.String(),
				}

				res, err = s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "migration should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify delegation moved to new account
				_, err = s.grpcHandler.GetDelegation(originalKey.AccAddr.String(), validatorAddr)
				Expect(err).ToNot(BeNil(), "original account should have no delegations")

				newDelegation, err := s.grpcHandler.GetDelegation(newKey.AccAddr.String(), validatorAddr)
				Expect(err).To(BeNil())
				Expect(newDelegation.DelegationResponse.Balance.Amount).To(Equal(delegationAmount))
			})

			It("should handle multiple delegations across validators", func() {
				originalKey := s.keyring.GetKey(2)
				newKey := s.keyring.GetKey(3)
				validators := s.network.GetValidators()
				Expect(len(validators)).To(BeNumerically(">=", 2))

				delegationAmount := math.NewInt(500000)

				// Create delegations to multiple validators
				for i, validator := range validators[:2] {
					delegateMsg := &stakingtypes.MsgDelegate{
						DelegatorAddress: originalKey.AccAddr.String(),
						ValidatorAddress: validator.OperatorAddress,
						Amount:           sdktypes.NewCoin(s.network.GetBaseDenom(), delegationAmount),
					}

					res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
						Msgs: []sdktypes.Msg{delegateMsg},
					})
					Expect(err).To(BeNil(), "delegation %d should have succeeded", i)
					Expect(res.IsOK()).To(Equal(true), "delegation should have succeeded", res.GetLog())
				}

				err := s.network.NextBlock()
				Expect(err).To(BeNil())

				// Execute migration
				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: originalKey.AccAddr.String(),
					NewAddress:      newKey.AccAddr.String(),
				}

				res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "migration should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify all delegations moved
				for _, validator := range validators[:2] {
					newDelegation, err := s.grpcHandler.GetDelegation(newKey.AccAddr.String(), validator.OperatorAddress)
					Expect(err).To(BeNil())
					Expect(newDelegation.DelegationResponse.Balance.Amount).To(Equal(delegationAmount))
				}
			})
		})

		Context("Fee Grant Migration", func() {
			It("should successfully migrate fee grants", func() {
				granterKey := s.keyring.GetKey(0)
				granteeKey := s.keyring.GetKey(1)
				newGranterKey := s.keyring.GetKey(2)

				// Create fee grant
				allowance := &feegrant.BasicAllowance{
					SpendLimit: sdktypes.NewCoins(sdktypes.NewCoin(s.network.GetBaseDenom(), math.NewInt(1000000))),
				}

				grantMsg := &feegrant.MsgGrantAllowance{
					Granter:   granterKey.AccAddr.String(),
					Grantee:   granteeKey.AccAddr.String(),
					Allowance: codectypes.UnsafePackAny(allowance),
				}

				res, err := s.factory.ExecuteCosmosTx(granterKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{grantMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "fee grant should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify grant exists
				grant, err := s.grpcHandler.Allowance(granterKey.AccAddr.String(), granteeKey.AccAddr.String())
				Expect(err).To(BeNil())
				Expect(grant.Allowance).ToNot(BeNil())

				// Execute migration
				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: granterKey.AccAddr.String(),
					NewAddress:      newGranterKey.AccAddr.String(),
				}

				res, err = s.factory.ExecuteCosmosTx(granterKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "migration should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify grant moved to new granter
				_, err = s.grpcHandler.Allowance(granterKey.AccAddr.String(), granteeKey.AccAddr.String())
				Expect(err).ToNot(BeNil(), "original granter should have no grants")

				newGrant, err := s.grpcHandler.Allowance(newGranterKey.AccAddr.String(), granteeKey.AccAddr.String())
				Expect(err).To(BeNil())
				Expect(newGrant.Allowance).ToNot(BeNil())
			})

			It("should handle multiple fee grants", func() {
				granterKey := s.keyring.GetKey(0)
				grantee1Key := s.keyring.GetKey(1)
				grantee2Key := s.keyring.GetKey(2)
				newGranterKey := s.keyring.GetKey(3)

				// Create multiple fee grants
				grantees := []testkeyring.Key{grantee1Key, grantee2Key}
				for i, grantee := range grantees {
					allowance := &feegrant.BasicAllowance{
						SpendLimit: sdktypes.NewCoins(sdktypes.NewCoin(s.network.GetBaseDenom(), math.NewInt(int64((i+1)*100000)))),
					}

					grantMsg := &feegrant.MsgGrantAllowance{
						Granter:   granterKey.AccAddr.String(),
						Grantee:   grantee.AccAddr.String(),
						Allowance: codectypes.UnsafePackAny(allowance),
					}

					res, err := s.factory.ExecuteCosmosTx(granterKey.Priv, factory2.CosmosTxArgs{
						Msgs: []sdktypes.Msg{grantMsg},
					})
					Expect(err).To(BeNil(), "fee grant %d should have succeeded", i)
					Expect(res.IsOK()).To(Equal(true), "fee grant should have succeeded", res.GetLog())
				}

				err := s.network.NextBlock()
				Expect(err).To(BeNil())

				// Execute migration
				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: granterKey.AccAddr.String(),
					NewAddress:      newGranterKey.AccAddr.String(),
				}

				res, err := s.factory.ExecuteCosmosTx(granterKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "migration should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify all grants moved
				for _, grantee := range grantees {
					newGrant, err := s.grpcHandler.Allowance(newGranterKey.AccAddr.String(), grantee.AccAddr.String())
					Expect(err).To(BeNil())
					Expect(newGrant.Allowance).ToNot(BeNil())
				}
			})
		})

		Context("Authorization (Authz) Migration", func() {
			It("should successfully migrate authz grants", func() {
				granterKey := s.keyring.GetKey(0)
				granteeKey := s.keyring.GetKey(1)
				newGranterKey := s.keyring.GetKey(2)

				// Create authz grant
				authorization := &banktypes.SendAuthorization{
					SpendLimit: sdktypes.NewCoins(sdktypes.NewCoin(s.network.GetBaseDenom(), math.NewInt(1000000))),
				}

				grantMsg := &authz.MsgGrant{
					Granter: granterKey.AccAddr.String(),
					Grantee: granteeKey.AccAddr.String(),
					Grant: authz.Grant{
						Authorization: codectypes.UnsafePackAny(authorization),
						Expiration:    &time.Time{}, // No expiration
					},
				}

				res, err := s.factory.ExecuteCosmosTx(granterKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{grantMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "authz grant should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify grant exists
				grants, err := s.grpcHandler.GetGrants(granterKey.AccAddr.String(), granteeKey.AccAddr.String())
				Expect(err).To(BeNil())
				Expect(len(grants)).To(BeNumerically(">", 0))

				// Execute migration
				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: granterKey.AccAddr.String(),
					NewAddress:      newGranterKey.AccAddr.String(),
				}

				res, err = s.factory.ExecuteCosmosTx(granterKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "migration should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify grant moved to new granter
				_, err = s.grpcHandler.GetGrants(granterKey.AccAddr.String(), granteeKey.AccAddr.String())
				Expect(err).ToNot(BeNil(), "original granter should have no grants")

				newGrants, err := s.grpcHandler.GetGrants(newGranterKey.AccAddr.String(), granteeKey.AccAddr.String())
				Expect(err).To(BeNil())
				Expect(len(newGrants)).To(BeNumerically(">", 0))
			})
		})

		Context("Complete Migration Scenarios", func() {
			It("should migrate all account data in one transaction", func() {
				originalKey := s.keyring.GetKey(0)
				newKey := s.keyring.GetKey(1)
				granteeKey := s.keyring.GetKey(2)

				// Setup account with multiple types of data
				validators := s.network.GetValidators()
				validatorAddr := validators[0].OperatorAddress
				baseDenom := s.network.GetBaseDenom()

				// 1. Create delegation
				delegateMsg := &stakingtypes.MsgDelegate{
					DelegatorAddress: originalKey.AccAddr.String(),
					ValidatorAddress: validatorAddr,
					Amount:           sdktypes.NewCoin(baseDenom, math.NewInt(1000000)),
				}
				res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{delegateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true))

				// 2. Create fee grant
				allowance := &feegrant.BasicAllowance{
					SpendLimit: sdktypes.NewCoins(sdktypes.NewCoin(baseDenom, math.NewInt(500000))),
				}
				grantMsg := &feegrant.MsgGrantAllowance{
					Granter:   originalKey.AccAddr.String(),
					Grantee:   granteeKey.AccAddr.String(),
					Allowance: codectypes.UnsafePackAny(allowance),
				}
				res, err = s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{grantMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true))

				// 3. Create authz grant
				authorization := &banktypes.SendAuthorization{
					SpendLimit: sdktypes.NewCoins(sdktypes.NewCoin(baseDenom, math.NewInt(300000))),
				}
				authzGrantMsg := &authz.MsgGrant{
					Granter: originalKey.AccAddr.String(),
					Grantee: granteeKey.AccAddr.String(),
					Grant: authz.Grant{
						Authorization: codectypes.UnsafePackAny(authorization),
					},
				}
				res, err = s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{authzGrantMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true))

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Get original balance
				originalBalance, err := s.grpcHandler.GetBalanceFromBank(originalKey.AccAddr, baseDenom)
				Expect(err).To(BeNil())

				// Execute migration
				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: originalKey.AccAddr.String(),
					NewAddress:      newKey.AccAddr.String(),
				}

				res, err = s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "complete migration should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify all data migrated
				// 1. Check delegation
				newDelegation, err := s.grpcHandler.GetDelegation(newKey.AccAddr.String(), validatorAddr)
				Expect(err).To(BeNil())
				Expect(newDelegation.DelegationResponse.Balance.Amount).To(Equal(math.NewInt(1000000)))

				// 2. Check fee grant
				newFeeGrant, err := s.grpcHandler.Allowance(newKey.AccAddr.String(), granteeKey.AccAddr.String())
				Expect(err).To(BeNil())
				Expect(newFeeGrant.Allowance).ToNot(BeNil())

				// 3. Check authz grant
				newAuthzGrants, err := s.grpcHandler.GetGrants(newKey.AccAddr.String(), granteeKey.AccAddr.String())
				Expect(err).To(BeNil())
				Expect(len(newAuthzGrants)).To(BeNumerically(">", 0))

				// 4. Check bank balance
				newBalance, err := s.grpcHandler.GetBalanceFromBank(newKey.AccAddr, baseDenom)
				Expect(err).To(BeNil())
				Expect(newBalance.GetBalance().Amount.GT(originalBalance.GetBalance().Amount)).To(BeTrue())
			})
		})

		Context("Error Scenarios", func() {
			It("should fail with invalid original address", func() {
				newKey := s.keyring.GetKey(1)

				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: "invalid-address",
					NewAddress:      newKey.AccAddr.String(),
				}

				_, err := s.factory.ExecuteCosmosTx(s.keyring.GetPrivKey(0), factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid address"))
			})

			It("should fail with invalid new address", func() {
				originalKey := s.keyring.GetKey(0)

				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: originalKey.AccAddr.String(),
					NewAddress:      "invalid-address",
				}

				_, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("invalid address"))
			})

			It("should fail when original address has no permissions", func() {
				unauthorizedKey := s.keyring.GetKey(4) // Different key than the original
				originalKey := s.keyring.GetKey(0)
				newKey := s.keyring.GetKey(1)

				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: originalKey.AccAddr.String(),
					NewAddress:      newKey.AccAddr.String(),
				}

				_, err := s.factory.ExecuteCosmosTx(unauthorizedKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).ToNot(BeNil())
				Expect(err.Error()).To(ContainSubstring("unauthorized"))
			})
		})

		DescribeTable("Migration with different signer scenarios", func(migrationTest MigrationTestCase) {
			originalKey := s.keyring.GetKey(migrationTest.OriginalKeyIndex)
			newKey := s.keyring.GetKey(migrationTest.NewKeyIndex)
			signerKey := s.keyring.GetKey(migrationTest.SignerKeyIndex)

			// Setup some data to migrate
			if migrationTest.SetupData {
				validators := s.network.GetValidators()
				if len(validators) > 0 {
					delegateMsg := &stakingtypes.MsgDelegate{
						DelegatorAddress: originalKey.AccAddr.String(),
						ValidatorAddress: validators[0].OperatorAddress,
						Amount:           sdktypes.NewCoin(s.network.GetBaseDenom(), math.NewInt(100000)),
					}
					res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
						Msgs: []sdktypes.Msg{delegateMsg},
					})
					Expect(err).To(BeNil())
					Expect(res.IsOK()).To(Equal(true))

					err = s.network.NextBlock()
					Expect(err).To(BeNil())
				}
			}

			migrateMsg := &evmtypes.MsgMigrateAccount{
				OriginalAddress: originalKey.AccAddr.String(),
				NewAddress:      newKey.AccAddr.String(),
			}

			res, err := s.factory.ExecuteCosmosTx(signerKey.Priv, factory2.CosmosTxArgs{
				Msgs: []sdktypes.Msg{migrateMsg},
			})

			if migrationTest.ExpectError {
				Expect(err).ToNot(BeNil())
				if migrationTest.ErrorSubstring != "" {
					Expect(err.Error()).To(ContainSubstring(migrationTest.ErrorSubstring))
				}
			} else {
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "migration should have succeeded", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify migration occurred
				if migrationTest.SetupData {
					validators := s.network.GetValidators()
					if len(validators) > 0 {
						newDelegation, err := s.grpcHandler.GetDelegation(newKey.AccAddr.String(), validators[0].OperatorAddress)
						Expect(err).To(BeNil())
						Expect(newDelegation.DelegationResponse.Balance.Amount).To(Equal(math.NewInt(100000)))
					}
				}
			}
		},
			Entry("successful migration with original account signer", MigrationTestCase{
				OriginalKeyIndex: 0,
				NewKeyIndex:      1,
				SignerKeyIndex:   0,
				SetupData:        true,
				ExpectError:      false,
			}),
			Entry("successful migration with new account signer", MigrationTestCase{
				OriginalKeyIndex: 0,
				NewKeyIndex:      1,
				SignerKeyIndex:   1,
				SetupData:        true,
				ExpectError:      false,
			}),
			Entry("failed migration with unauthorized signer", MigrationTestCase{
				OriginalKeyIndex: 0,
				NewKeyIndex:      1,
				SignerKeyIndex:   2,
				SetupData:        false,
				ExpectError:      true,
				ErrorSubstring:   "unauthorized",
			}),
			Entry("migration with same original and new address should fail", MigrationTestCase{
				OriginalKeyIndex: 0,
				NewKeyIndex:      0, // Same as original
				SignerKeyIndex:   0,
				SetupData:        false,
				ExpectError:      true,
				ErrorSubstring:   "same address",
			}),
		)

		Context("Advanced Migration Scenarios", func() {
			It("should handle migration with high gas requirements", func() {
				originalKey := s.keyring.GetKey(0)
				newKey := s.keyring.GetKey(1)

				// Setup account with multiple entities
				validators := s.network.GetValidators()
				baseDenom := s.network.GetBaseDenom()

				// Create multiple delegations
				maxDelegations := 3 // Use smaller number for test performance
				for i := 0; i < maxDelegations && i < len(validators); i++ {
					delegateMsg := &stakingtypes.MsgDelegate{
						DelegatorAddress: originalKey.AccAddr.String(),
						ValidatorAddress: validators[i].OperatorAddress,
						Amount:           sdktypes.NewCoin(baseDenom, math.NewInt(100000)),
					}
					res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
						Msgs: []sdktypes.Msg{delegateMsg},
					})
					Expect(err).To(BeNil())
					Expect(res.IsOK()).To(Equal(true))
				}

				// Create multiple fee grants
				maxFeeGrants := 2
				var grantMsgs []sdktypes.Msg

				for i := 0; i < maxFeeGrants; i++ {
					granteeKey := s.keyring.GetKey((i % 6) + 2) // Rotate through available keys
					allowance := &feegrant.BasicAllowance{
						SpendLimit: sdktypes.NewCoins(sdktypes.NewCoin(baseDenom, math.NewInt(int64(10000*(i+1))))),
					}
					grantMsg := &feegrant.MsgGrantAllowance{
						Granter:   originalKey.AccAddr.String(),
						Grantee:   granteeKey.AccAddr.String(),
						Allowance: codectypes.UnsafePackAny(allowance), // Changed from UnsafePackAny to MustPackAny
					}
					grantMsgs = append(grantMsgs, grantMsg)
				}

				// Execute all fee grants in a single transaction
				res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: grantMsgs,
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true))

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Execute migration with custom gas settings
				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: originalKey.AccAddr.String(),
					NewAddress:      newKey.AccAddr.String(),
				}

				// Use higher gas limit for complex migration
				res, err = s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "high-complexity migration should succeed", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify all entities migrated
				for i := 0; i < maxDelegations && i < len(validators); i++ {
					delegation, err := s.grpcHandler.GetDelegation(newKey.AccAddr.String(), validators[i].OperatorAddress)
					Expect(err).To(BeNil())
					Expect(delegation.DelegationResponse.Balance.Amount).To(Equal(math.NewInt(100000)))
				}
			})

			It("should handle migration when target account already has data", func() {
				originalKey := s.keyring.GetKey(0)
				newKey := s.keyring.GetKey(1)
				validators := s.network.GetValidators()
				baseDenom := s.network.GetBaseDenom()

				// Setup both accounts with existing data
				if len(validators) > 0 {
					// Original account delegation
					delegateMsg1 := &stakingtypes.MsgDelegate{
						DelegatorAddress: originalKey.AccAddr.String(),
						ValidatorAddress: validators[0].OperatorAddress,
						Amount:           sdktypes.NewCoin(baseDenom, math.NewInt(100000)),
					}
					res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
						Msgs: []sdktypes.Msg{delegateMsg1},
					})
					Expect(err).To(BeNil())
					Expect(res.IsOK()).To(Equal(true))

					// New account already has some delegation
					delegateMsg2 := &stakingtypes.MsgDelegate{
						DelegatorAddress: newKey.AccAddr.String(),
						ValidatorAddress: validators[0].OperatorAddress,
						Amount:           sdktypes.NewCoin(baseDenom, math.NewInt(50000)),
					}
					res, err = s.factory.ExecuteCosmosTx(newKey.Priv, factory2.CosmosTxArgs{
						Msgs: []sdktypes.Msg{delegateMsg2},
					})
					Expect(err).To(BeNil())
					Expect(res.IsOK()).To(Equal(true))

					err = s.network.NextBlock()
					Expect(err).To(BeNil())
				}

				// Get pre-migration state
				var preMigrationDelegation math.Int
				if len(validators) > 0 {
					delegation, err := s.grpcHandler.GetDelegation(newKey.AccAddr.String(), validators[0].OperatorAddress)
					Expect(err).To(BeNil())
					preMigrationDelegation = delegation.DelegationResponse.Balance.Amount
				}

				// Execute migration
				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: originalKey.AccAddr.String(),
					NewAddress:      newKey.AccAddr.String(),
				}

				res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "migration to account with existing data should succeed", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify combined delegation
				if len(validators) > 0 {
					finalDelegation, err := s.grpcHandler.GetDelegation(newKey.AccAddr.String(), validators[0].OperatorAddress)
					Expect(err).To(BeNil())
					expectedTotal := preMigrationDelegation.Add(math.NewInt(100000))
					Expect(finalDelegation.DelegationResponse.Balance.Amount).To(Equal(expectedTotal))
				}
			})

			It("should maintain state consistency during migration", func() {
				originalKey := s.keyring.GetKey(0)
				newKey := s.keyring.GetKey(1)

				// Setup initial state
				validators := s.network.GetValidators()
				baseDenom := s.network.GetBaseDenom()

				// Get initial balances
				originalBalance, err := s.grpcHandler.GetBalanceFromBank(originalKey.AccAddr, baseDenom)
				Expect(err).To(BeNil())
				initialAmount := originalBalance.GetBalance().Amount

				newBalance, err := s.grpcHandler.GetBalanceFromBank(newKey.AccAddr, baseDenom)
				Expect(err).To(BeNil())
				newInitialAmount := newBalance.GetBalance().Amount

				// Create delegation
				if len(validators) > 0 {
					delegateMsg := &stakingtypes.MsgDelegate{
						DelegatorAddress: originalKey.AccAddr.String(),
						ValidatorAddress: validators[0].OperatorAddress,
						Amount:           sdktypes.NewCoin(baseDenom, math.NewInt(100000)),
					}
					res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
						Msgs: []sdktypes.Msg{delegateMsg},
					})
					Expect(err).To(BeNil())
					Expect(res.IsOK()).To(Equal(true))

					err = s.network.NextBlock()
					Expect(err).To(BeNil())
				}

				// Execute migration
				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: originalKey.AccAddr.String(),
					NewAddress:      newKey.AccAddr.String(),
				}

				res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true))

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify state consistency
				// 1. Original account should have minimal/zero balance
				finalOriginalBalance, err := s.grpcHandler.GetBalanceFromBank(originalKey.AccAddr, baseDenom)
				Expect(err).To(BeNil())
				Expect(finalOriginalBalance.GetBalance().Amount.LTE(initialAmount)).To(BeTrue())

				// 2. New account should have the migrated funds
				finalNewBalance, err := s.grpcHandler.GetBalanceFromBank(newKey.AccAddr, baseDenom)
				Expect(err).To(BeNil())
				Expect(finalNewBalance.GetBalance().Amount.GT(newInitialAmount)).To(BeTrue())
			})

			It("should handle migration of account with zero balance", func() {
				// Create new accounts with zero balance
				originalKey := s.keyring.GetKey(6)
				newKey := s.keyring.GetKey(7)

				// Ensure original account has zero balance by transferring all funds out
				originalBalance, err := s.grpcHandler.GetBalanceFromBank(originalKey.AccAddr, s.network.GetBaseDenom())
				Expect(err).To(BeNil())

				if originalBalance.GetBalance().Amount.GT(math.ZeroInt()) {
					// Transfer all funds to another account to create zero balance
					transferKey := s.keyring.GetKey(0)
					sendMsg := &banktypes.MsgSend{
						FromAddress: originalKey.AccAddr.String(),
						ToAddress:   transferKey.AccAddr.String(),
						Amount:      sdktypes.NewCoins(*originalBalance.GetBalance()),
					}
					res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
						Msgs: []sdktypes.Msg{sendMsg},
					})
					Expect(err).To(BeNil())
					Expect(res.IsOK()).To(Equal(true))

					err = s.network.NextBlock()
					Expect(err).To(BeNil())
				}

				// Verify zero balance
				zeroBalance, err := s.grpcHandler.GetBalanceFromBank(originalKey.AccAddr, s.network.GetBaseDenom())
				Expect(err).To(BeNil())
				Expect(zeroBalance.GetBalance().Amount.IsZero()).To(BeTrue())

				// Execute migration
				migrateMsg := &evmtypes.MsgMigrateAccount{
					OriginalAddress: originalKey.AccAddr.String(),
					NewAddress:      newKey.AccAddr.String(),
				}

				res, err := s.factory.ExecuteCosmosTx(originalKey.Priv, factory2.CosmosTxArgs{
					Msgs: []sdktypes.Msg{migrateMsg},
				})
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(Equal(true), "zero balance migration should succeed", res.GetLog())

				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				// Verify migration completed
				_, err = s.grpcHandler.GetBalanceFromBank(newKey.AccAddr, s.network.GetBaseDenom())
				Expect(err).To(BeNil())
				// New account might have some dust from the migration process
			})
		})
	})
})

type PermissionsTableTest struct {
	ExpFail     bool
	SignerIndex int
}

type MigrationTestCase struct {
	OriginalKeyIndex int
	NewKeyIndex      int
	SignerKeyIndex   int
	SetupData        bool
	ExpectError      bool
	ErrorSubstring   string
}

func checkMintTopics(res abcitypes.ExecTxResult) error {
	// Check contract call response has the expected topics for a mint
	// call within an ERC20 contract
	expectedTopics := []string{
		"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
		"0x0000000000000000000000000000000000000000000000000000000000000000",
	}
	return integrationutils.CheckTxTopics(res, expectedTopics)
}
