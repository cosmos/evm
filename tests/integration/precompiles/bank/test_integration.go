package bank

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	bank2 "github.com/cosmos/evm/precompiles/bank"
	"github.com/cosmos/evm/precompiles/bank/testdata"
	"github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	"github.com/cosmos/evm/testutil/keyring"
	utiltx "github.com/cosmos/evm/testutil/tx"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

// IntegrationTestSuite is the implementation of the TestSuite interface for Bank precompile
// unit testis.
type IntegrationTestSuite struct {
	suite.Suite

	bondDenom, tokenDenom   string
	cosmosEVMAddr, xmplAddr common.Address

	create      network.CreateEvmApp
	options     []network.ConfigOption
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     keyring.Keyring

	precompile *bank2.Precompile
}

func NewIntegrationTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *IntegrationTestSuite {
	return &IntegrationTestSuite{
		create:  create,
		options: options,
	}
}

func (is *IntegrationTestSuite) SetupTest() {
	// Mint and register a second coin for testing purposes
	// FIXME the RegisterCoin logic will need to be refactored
	// once logic is integrated
	// with the protocol via genesis and/or a transaction
	is.tokenDenom = xmplDenom
	keyring := keyring.New(2)
	genesis := utils.CreateGenesisWithTokenPairs(keyring)

	options := []network.ConfigOption{
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		network.WithOtherDenoms([]string{is.tokenDenom}), // set some funds of other denom to the prefunded accounts
		network.WithCustomGenesis(genesis),
	}
	options = append(options, is.options...)
	integrationNetwork := network.NewUnitTestNetwork(is.create, options...)
	grpcHandler := grpc.NewIntegrationHandler(integrationNetwork)
	txFactory := factory.New(integrationNetwork, grpcHandler)

	ctx := integrationNetwork.GetContext()
	sk := integrationNetwork.App.GetStakingKeeper()
	bondDenom, err := sk.BondDenom(ctx)
	Expect(err).ToNot(HaveOccurred())
	Expect(bondDenom).ToNot(BeEmpty(), "bond denom cannot be empty")

	is.bondDenom = bondDenom
	is.factory = txFactory
	is.grpcHandler = grpcHandler
	is.keyring = keyring
	is.network = integrationNetwork

	tokenPairID := is.network.App.GetErc20Keeper().GetTokenPairID(is.network.GetContext(), is.bondDenom)
	tokenPair, found := is.network.App.GetErc20Keeper().GetTokenPair(is.network.GetContext(), tokenPairID)
	Expect(found).To(BeTrue(), "failed to register token erc20 extension")
	is.cosmosEVMAddr = common.HexToAddress(tokenPair.Erc20Address)

	// Mint and register a second coin for testing purposes
	err = is.network.App.GetBankKeeper().MintCoins(is.network.GetContext(), minttypes.ModuleName, sdk.Coins{{Denom: is.tokenDenom, Amount: math.NewInt(1e18)}})
	Expect(err).ToNot(HaveOccurred(), "failed to mint coin")

	tokenPairID = is.network.App.GetErc20Keeper().GetTokenPairID(is.network.GetContext(), is.tokenDenom)
	tokenPair, found = is.network.App.GetErc20Keeper().GetTokenPair(is.network.GetContext(), tokenPairID)
	Expect(found).To(BeTrue(), "failed to register token erc20 extension")
	is.xmplAddr = common.HexToAddress(tokenPair.Erc20Address)
	is.precompile = is.setupBankPrecompile()
}

func TestIntegrationSuite(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	var is *IntegrationTestSuite

	_ = Describe("Bank Extension -", func() {
		var (
			bankCallerContractAddr common.Address
			bankCallerContract     evmtypes.CompiledContract

			err    error
			sender keyring.Key
			amount *big.Int

			// contractData is a helper struct to hold the addresses and ABIs for the
			// different contract instances that are subject to testing here.
			contractData ContractData
			passCheck    testutil.LogCheckArgs

			cosmosEVMTotalSupply *big.Int
			xmplTotalSupply      *big.Int
		)

		BeforeEach(func() {
			is = NewIntegrationTestSuite(create, options...)
			is.SetupTest()

			// Default sender, amount
			sender = is.keyring.GetKey(0)
			amount = big.NewInt(1e18)

			bankCallerContract, err = testdata.LoadBankCallerContract()
			Expect(err).ToNot(HaveOccurred(), "failed to load BankCaller contract")

			bankCallerContractAddr, err = is.factory.DeployContract(
				sender.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: bankCallerContract,
				},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to deploy ERC20 minter burner contract")

			contractData = ContractData{
				ownerPriv:      sender.Priv,
				precompileAddr: is.precompile.Address(),
				precompileABI:  is.precompile.ABI,
				contractAddr:   bankCallerContractAddr,
				contractABI:    bankCallerContract.ABI,
			}

			passCheck = testutil.LogCheckArgs{}.WithExpPass(true)

			err = is.network.NextBlock()
			Expect(err).ToNot(HaveOccurred(), "failed to advance block")

			// Get total supply from bank keeper
			cosmosSupply := is.network.App.GetBankKeeper().GetSupply(is.network.GetContext(), is.bondDenom)
			cosmosEVMTotalSupply = new(big.Int).Set(cosmosSupply.Amount.BigInt())

			xmplSupply := is.network.App.GetBankKeeper().GetSupply(is.network.GetContext(), is.tokenDenom)
			xmplTotalSupply = new(big.Int).Set(xmplSupply.Amount.BigInt())
		})

		Context("Direct precompile queries", func() {
			Context("balances query", func() {
				It("should return the correct balance", func() {
					// New account with 0 balances (does not exist on the chain yet)
					receiver := utiltx.GenerateAddress()

					err := is.factory.FundAccount(sender, receiver.Bytes(), sdk.NewCoins(sdk.NewCoin(is.tokenDenom, math.NewIntFromBigInt(amount))))
					Expect(err).ToNot(HaveOccurred(), "error while funding account")
					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")

					queryArgs, balancesArgs := getTxAndCallArgs(directCall, contractData, bank2.BalancesMethod, receiver)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var balances []bank2.Balance
					err = is.precompile.UnpackIntoInterface(&balances, bank2.BalancesMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					balanceAfter, err := is.grpcHandler.GetBalanceFromBank(receiver.Bytes(), is.tokenDenom)
					Expect(err).ToNot(HaveOccurred(), "failed to get balance")

					Expect(math.NewInt(balances[0].Amount.Int64())).To(Equal(balanceAfter.Balance.Amount))
					Expect(*balances[0].Amount).To(Equal(*amount))
				})

				It("should return a single token balance", func() {
					// New account with 0 balances (does not exist on the chain yet)
					receiver := utiltx.GenerateAddress()

					err := utils.FundAccountWithBaseDenom(is.factory, is.network, sender, receiver.Bytes(), math.NewIntFromBigInt(amount))
					Expect(err).ToNot(HaveOccurred(), "error while funding account")
					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")

					queryArgs, balancesArgs := getTxAndCallArgs(directCall, contractData, bank2.BalancesMethod, receiver)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var balances []bank2.Balance
					err = is.precompile.UnpackIntoInterface(&balances, bank2.BalancesMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					balanceAfter, err := is.grpcHandler.GetBalanceFromBank(receiver.Bytes(), is.network.GetBaseDenom())
					Expect(err).ToNot(HaveOccurred(), "failed to get balance")

					Expect(math.NewInt(balances[0].Amount.Int64())).To(Equal(balanceAfter.Balance.Amount))
					Expect(*balances[0].Amount).To(Equal(*amount))
				})

				It("should return no balance for new account", func() {
					queryArgs, balancesArgs := getTxAndCallArgs(directCall, contractData, bank2.BalancesMethod, utiltx.GenerateAddress())
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var balances []bank2.Balance
					err = is.precompile.UnpackIntoInterface(&balances, bank2.BalancesMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(balances).To(BeEmpty())
				})

				It("should consume the correct amount of gas", func() {
					queryArgs, balancesArgs := getTxAndCallArgs(directCall, contractData, bank2.BalancesMethod, sender.Addr)
					res, err := is.factory.ExecuteContractCall(sender.Priv, queryArgs, balancesArgs)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					ethRes, err := evmtypes.DecodeTxResponse(res.Data)
					Expect(err).ToNot(HaveOccurred(), "failed to decode tx response")

					var balances []bank2.Balance
					err = is.precompile.UnpackIntoInterface(&balances, bank2.BalancesMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					gasUsed := Max(bank2.GasBalances, len(balances)*bank2.GasBalances)
					// Here increasing the GasBalanceOf will increase the use of gas so they will never be equal
					Expect(gasUsed).To(BeNumerically("<=", ethRes.GasUsed))
				})
			})

			Context("totalSupply query", func() {
				It("should return the correct total supply", func() {
					queryArgs, supplyArgs := getTxAndCallArgs(directCall, contractData, bank2.TotalSupplyMethod)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var balances []bank2.Balance
					err = is.precompile.UnpackIntoInterface(&balances, bank2.TotalSupplyMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(balances[0].Amount.String()).To(Equal(cosmosEVMTotalSupply.String()))
					Expect(balances[1].Amount.String()).To(Equal(xmplTotalSupply.String()))
				})

				It("should properly handle OOG in precompile and consume all gas", func() {
					numDenoms := 5000
					// Mint denoms to make TotalSupply expensive enough to OOG.
					ctx := is.network.GetContext()
					for i := 0; i < numDenoms; i++ {
						denom := fmt.Sprintf("token%d", i)
						err = is.network.App.GetBankKeeper().MintCoins(
							ctx, minttypes.ModuleName,
							sdk.Coins{{Denom: denom, Amount: math.NewInt(1e18)}},
						)
						Expect(err).ToNot(HaveOccurred(), "failed to mint coin %s", denom)
					}
					// Commit keeper changes directly to state.
					store := is.network.GetContext().MultiStore()
					cms, ok := store.(storetypes.CacheMultiStore)
					Expect(ok).To(BeTrue())
					cms.Write()

					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "failed to advance block")

					// Use callTotalSupplyWithGas to measure inner call gas consumption.
					// Forward enough gas for the precompile to OOG iterating 5000+ denoms.
					gasForward := big.NewInt(9_000_000)

					txArgs := evmtypes.EvmTxArgs{
						To:       &bankCallerContractAddr,
						GasLimit: 20_000_000,
					}
					callArgs := testutiltypes.CallArgs{
						ContractABI: bankCallerContract.ABI,
						MethodName:  "callTotalSupplyWithGas",
						Args:        []interface{}{gasForward},
					}

					res, execErr := is.factory.ExecuteContractCall(sender.Priv, txArgs, callArgs)
					Expect(execErr).ToNot(HaveOccurred(), "failed to execute callTotalSupplyWithGas")

					ethRes, decErr := evmtypes.DecodeTxResponse(res.Data)
					Expect(decErr).ToNot(HaveOccurred(), "failed to decode eth tx response")
					Expect(ethRes.VmError).To(BeEmpty(), "outer call should not revert")

					// Unpack: (bool success, uint256 innerGasUsed)
					out, unpackErr := bankCallerContract.ABI.Unpack("callTotalSupplyWithGas", ethRes.Ret)
					Expect(unpackErr).ToNot(HaveOccurred(), "failed to unpack result")

					innerSuccess := out[0].(bool)
					innerGasUsed := out[1].(*big.Int)

					fmt.Printf("\nInner call: success=%v, innerGasUsed=%s, gasForwarded=%s, txGasUsed=%d\n",
						innerSuccess, innerGasUsed.String(), gasForward.String(), ethRes.GasUsed)

					// The inner call should have failed (OOG with 5000 denoms).
					Expect(innerSuccess).To(BeFalse(), "inner call should OOG with insufficient gas for 5000 denoms")

					// The inner call should have consumed all (or nearly all) forwarded gas.
					// With correct gas accounting, innerGasUsed ≈ gasForward.
					// With the bug, innerGasUsed would be ~3k (only EVM overhead, no precompile charge).
					Expect(innerGasUsed.Uint64()).To(BeNumerically(">=", gasForward.Uint64()*90/100),
						"inner OOG call should consume all forwarded gas, got %s out of %s", innerGasUsed.String(), gasForward.String())
				})
			})

			Context("supplyOf query", func() {
				It("should return the supply of Cosmos EVM", func() {
					queryArgs, supplyArgs := getTxAndCallArgs(directCall, contractData, bank2.SupplyOfMethod, is.cosmosEVMAddr)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					out, err := is.precompile.Unpack(bank2.SupplyOfMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(out[0].(*big.Int).String()).To(Equal(cosmosEVMTotalSupply.String()))
				})

				It("should return the supply of XMPL", func() {
					queryArgs, supplyArgs := getTxAndCallArgs(directCall, contractData, bank2.SupplyOfMethod, is.xmplAddr)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					out, err := is.precompile.Unpack(bank2.SupplyOfMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(out[0].(*big.Int).String()).To(Equal(xmplTotalSupply.String()))
				})

				It("should return a supply of 0 for a non existing token", func() {
					queryArgs, supplyArgs := getTxAndCallArgs(directCall, contractData, bank2.SupplyOfMethod, utiltx.GenerateAddress())
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					out, err := is.precompile.Unpack(bank2.SupplyOfMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(out[0].(*big.Int).Int64()).To(Equal(big.NewInt(0).Int64()))
				})

				It("should consume the correct amount of gas", func() {
					queryArgs, supplyArgs := getTxAndCallArgs(directCall, contractData, bank2.SupplyOfMethod, is.xmplAddr)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					// Here increasing the GasSupplyOf will increase the use of gas so they will never be equal
					Expect(bank2.GasSupplyOf).To(BeNumerically("<=", ethRes.GasUsed))
				})
			})
		})

		Context("Calls from a contract", func() {
			const (
				BalancesFunction = "callBalances"
				TotalSupplyOf    = "callTotalSupply"
				SupplyOfFunction = "callSupplyOf"
			)

			Context("balances query", func() {
				It("should return the correct balance", func() {
					receiver := utiltx.GenerateAddress()

					err := is.factory.FundAccount(sender, receiver.Bytes(), sdk.NewCoins(sdk.NewCoin(is.tokenDenom, math.NewIntFromBigInt(amount))))
					Expect(err).ToNot(HaveOccurred(), "error while funding account")
					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")

					queryArgs, balancesArgs := getTxAndCallArgs(contractCall, contractData, BalancesFunction, receiver)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var balances []bank2.Balance
					err = is.precompile.UnpackIntoInterface(&balances, bank2.BalancesMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					balanceAfter, err := is.grpcHandler.GetBalanceFromBank(receiver.Bytes(), is.tokenDenom)
					Expect(err).ToNot(HaveOccurred(), "failed to get balance")

					Expect(math.NewInt(balances[0].Amount.Int64())).To(Equal(balanceAfter.Balance.Amount))
					Expect(*balances[0].Amount).To(Equal(*amount))
				})

				It("should return a single token balance", func() {
					// New account with 0 balances (does not exist on the chain yet)
					receiver := utiltx.GenerateAddress()

					err := utils.FundAccountWithBaseDenom(is.factory, is.network, sender, receiver.Bytes(), math.NewIntFromBigInt(amount))
					Expect(err).ToNot(HaveOccurred(), "error while funding account")
					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")

					queryArgs, balancesArgs := getTxAndCallArgs(contractCall, contractData, BalancesFunction, receiver)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var balances []bank2.Balance
					err = is.precompile.UnpackIntoInterface(&balances, bank2.BalancesMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					balanceAfter, err := is.grpcHandler.GetBalanceFromBank(receiver.Bytes(), is.network.GetBaseDenom())
					Expect(err).ToNot(HaveOccurred(), "failed to get balance")

					Expect(math.NewInt(balances[0].Amount.Int64())).To(Equal(balanceAfter.Balance.Amount))
					Expect(*balances[0].Amount).To(Equal(*amount))
				})

				It("should return no balance for new account", func() {
					queryArgs, balancesArgs := getTxAndCallArgs(contractCall, contractData, BalancesFunction, utiltx.GenerateAddress())
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var balances []bank2.Balance
					err = is.precompile.UnpackIntoInterface(&balances, bank2.BalancesMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(balances).To(BeEmpty())
				})

				It("should consume the correct amount of gas", func() {
					queryArgs, balancesArgs := getTxAndCallArgs(contractCall, contractData, BalancesFunction, sender.Addr)
					res, err := is.factory.ExecuteContractCall(sender.Priv, queryArgs, balancesArgs)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					ethRes, err := evmtypes.DecodeTxResponse(res.Data)
					Expect(err).ToNot(HaveOccurred(), "failed to decode tx response")

					var balances []bank2.Balance
					err = is.precompile.UnpackIntoInterface(&balances, bank2.BalancesMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					gasUsed := Max(bank2.GasBalances, len(balances)*bank2.GasBalances)
					// Here increasing the GasBalanceOf will increase the use of gas so they will never be equal
					Expect(gasUsed).To(BeNumerically("<=", ethRes.GasUsed))
				})
			})

			Context("totalSupply query", func() {
				It("should return the correct total supply", func() {
					queryArgs, supplyArgs := getTxAndCallArgs(contractCall, contractData, TotalSupplyOf)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var balances []bank2.Balance
					err = is.precompile.UnpackIntoInterface(&balances, bank2.TotalSupplyMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(balances[0].Amount.String()).To(Equal(cosmosEVMTotalSupply.String()))
					Expect(balances[1].Amount.String()).To(Equal(xmplTotalSupply.String()))
				})
			})

			Context("supplyOf query", func() {
				It("should return the supply of Cosmos EVM", func() {
					queryArgs, supplyArgs := getTxAndCallArgs(contractCall, contractData, SupplyOfFunction, is.cosmosEVMAddr)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					out, err := is.precompile.Unpack(bank2.SupplyOfMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(out[0].(*big.Int).String()).To(Equal(cosmosEVMTotalSupply.String()))
				})

				It("should return the supply of XMPL", func() {
					queryArgs, supplyArgs := getTxAndCallArgs(contractCall, contractData, SupplyOfFunction, is.xmplAddr)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					out, err := is.precompile.Unpack(bank2.SupplyOfMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(out[0].(*big.Int).String()).To(Equal(xmplTotalSupply.String()))
				})

				It("should return a supply of 0 for a non existing token", func() {
					queryArgs, supplyArgs := getTxAndCallArgs(contractCall, contractData, SupplyOfFunction, utiltx.GenerateAddress())
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					out, err := is.precompile.Unpack(bank2.SupplyOfMethod, ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(out[0].(*big.Int).Int64()).To(Equal(big.NewInt(0).Int64()))
				})

				It("should consume the correct amount of gas", func() {
					queryArgs, supplyArgs := getTxAndCallArgs(contractCall, contractData, SupplyOfFunction, is.xmplAddr)
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					// Here increasing the GasSupplyOf will increase the use of gas so they will never be equal
					Expect(bank2.GasSupplyOf).To(BeNumerically("<=", ethRes.GasUsed))
				})
			})
		})
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bank Extension Suite")
}
