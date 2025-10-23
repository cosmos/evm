package bank

import (
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

			cosmosEVMTotalSupply, _ = new(big.Int).SetString("200003000000000000000000", 10)
			xmplTotalSupply, _      = new(big.Int).SetString("200000000000000000000000", 10)
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
				contractAddr:   bankCallerContractAddr,
				contractABI:    bankCallerContract.ABI,
			}

			passCheck = testutil.LogCheckArgs{}.WithExpPass(true)

			err = is.network.NextBlock()
			Expect(err).ToNot(HaveOccurred(), "failed to advance block")
		})

		Context("Direct precompile queries", func() {
			Context("balances query", func() {
				It("should return the correct balance", func() {
					// New account with 0 balances (does not exist on the chain yet)
					receiver := utiltx.GenerateAddress()

					err := is.factory.FundAccount(sender, receiver.Bytes(), sdk.NewCoins(sdk.NewCoin(is.tokenDenom, math.NewIntFromBigInt(amount))))
					Expect(err).ToNot(HaveOccurred(), "error while funding account")
					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")

					queryArgs := getTxAndCallArgs(directCall, contractData)
					args := &bank2.BalancesCall{Account: receiver}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, args, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.BalancesReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					balanceAfter, err := is.grpcHandler.GetBalanceFromBank(receiver.Bytes(), is.tokenDenom)
					Expect(err).ToNot(HaveOccurred(), "failed to get balance")

					Expect(math.NewInt(ret.Balances[0].Amount.Int64())).To(Equal(balanceAfter.Balance.Amount))
					Expect(*ret.Balances[0].Amount).To(Equal(*amount))
				})

				It("should return a single token balance", func() {
					// New account with 0 balances (does not exist on the chain yet)
					receiver := utiltx.GenerateAddress()

					err := utils.FundAccountWithBaseDenom(is.factory, is.network, sender, receiver.Bytes(), math.NewIntFromBigInt(amount))
					Expect(err).ToNot(HaveOccurred(), "error while funding account")
					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")

					queryArgs := getTxAndCallArgs(directCall, contractData)
					balancesArgs := &bank2.BalancesCall{Account: receiver}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.BalancesReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					balanceAfter, err := is.grpcHandler.GetBalanceFromBank(receiver.Bytes(), is.network.GetBaseDenom())
					Expect(err).ToNot(HaveOccurred(), "failed to get balance")

					Expect(math.NewInt(ret.Balances[0].Amount.Int64())).To(Equal(balanceAfter.Balance.Amount))
					Expect(*ret.Balances[0].Amount).To(Equal(*amount))
				})

				It("should return no balance for new account", func() {
					queryArgs := getTxAndCallArgs(directCall, contractData)
					balancesArgs := &bank2.BalancesCall{Account: utiltx.GenerateAddress()}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.BalancesReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(ret.Balances).To(BeEmpty())
				})

				It("should consume the correct amount of gas", func() {
					queryArgs := getTxAndCallArgs(directCall, contractData)
					balancesArgs := &bank2.BalancesCall{Account: sender.Addr}
					res, err := is.factory.ExecuteContractCall(sender.Priv, queryArgs, balancesArgs)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					ethRes, err := evmtypes.DecodeTxResponse(res.Data)
					Expect(err).ToNot(HaveOccurred(), "failed to decode tx response")

					var ret bank2.BalancesReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					gasUsed := Max(bank2.GasBalances, len(ret.Balances)*bank2.GasBalances)
					// Here increasing the GasBalanceOf will increase the use of gas so they will never be equal
					Expect(gasUsed).To(BeNumerically("<=", ethRes.GasUsed))
				})
			})

			Context("totalSupply query", func() {
				It("should return the correct total supply", func() {
					queryArgs := getTxAndCallArgs(directCall, contractData)
					supplyArgs := &bank2.TotalSupplyCall{}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.TotalSupplyReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(ret.TotalSupply[0].Amount.String()).To(Equal(cosmosEVMTotalSupply.String()))
					Expect(ret.TotalSupply[1].Amount.String()).To(Equal(xmplTotalSupply.String()))
				})
			})

			Context("supplyOf query", func() {
				It("should return the supply of Cosmos EVM", func() {
					queryArgs := getTxAndCallArgs(directCall, contractData)
					supplyArgs := &bank2.SupplyOfCall{Erc20Address: is.cosmosEVMAddr}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.SupplyOfReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(ret.TotalSupply.String()).To(Equal(cosmosEVMTotalSupply.String()))
				})

				It("should return the supply of XMPL", func() {
					queryArgs := getTxAndCallArgs(directCall, contractData)
					supplyArgs := &bank2.SupplyOfCall{Erc20Address: is.xmplAddr}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.SupplyOfReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(ret.TotalSupply.String()).To(Equal(xmplTotalSupply.String()))
				})

				It("should return a supply of 0 for a non existing token", func() {
					queryArgs := getTxAndCallArgs(directCall, contractData)
					supplyArgs := &bank2.SupplyOfCall{Erc20Address: utiltx.GenerateAddress()}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.SupplyOfReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(ret.TotalSupply.Int64()).To(Equal(big.NewInt(0).Int64()))
				})

				It("should consume the correct amount of gas", func() {
					queryArgs := getTxAndCallArgs(directCall, contractData)
					supplyArgs := &bank2.SupplyOfCall{Erc20Address: is.xmplAddr}
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

					queryArgs := getTxAndCallArgs(contractCall, contractData)
					balancesArgs := &bank2.BalancesCall{Account: receiver}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.BalancesReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					balanceAfter, err := is.grpcHandler.GetBalanceFromBank(receiver.Bytes(), is.tokenDenom)
					Expect(err).ToNot(HaveOccurred(), "failed to get balance")

					Expect(math.NewInt(ret.Balances[0].Amount.Int64())).To(Equal(balanceAfter.Balance.Amount))
					Expect(*ret.Balances[0].Amount).To(Equal(*amount))
				})

				It("should return a single token balance", func() {
					// New account with 0 balances (does not exist on the chain yet)
					receiver := utiltx.GenerateAddress()

					err := utils.FundAccountWithBaseDenom(is.factory, is.network, sender, receiver.Bytes(), math.NewIntFromBigInt(amount))
					Expect(err).ToNot(HaveOccurred(), "error while funding account")
					Expect(is.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")

					queryArgs := getTxAndCallArgs(contractCall, contractData)
					balancesArgs := &bank2.BalancesCall{Account: receiver}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.BalancesReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					balanceAfter, err := is.grpcHandler.GetBalanceFromBank(receiver.Bytes(), is.network.GetBaseDenom())
					Expect(err).ToNot(HaveOccurred(), "failed to get balance")

					Expect(math.NewInt(ret.Balances[0].Amount.Int64())).To(Equal(balanceAfter.Balance.Amount))
					Expect(*ret.Balances[0].Amount).To(Equal(*amount))
				})

				It("should return no balance for new account", func() {
					queryArgs := getTxAndCallArgs(contractCall, contractData)
					balancesArgs := &bank2.BalancesCall{Account: utiltx.GenerateAddress()}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, balancesArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.BalancesReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(ret.Balances).To(BeEmpty())
				})

				It("should consume the correct amount of gas", func() {
					queryArgs := getTxAndCallArgs(contractCall, contractData)
					balancesArgs := &bank2.BalancesCall{Account: sender.Addr}
					res, err := is.factory.ExecuteContractCall(sender.Priv, queryArgs, balancesArgs)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					ethRes, err := evmtypes.DecodeTxResponse(res.Data)
					Expect(err).ToNot(HaveOccurred(), "failed to decode tx response")

					var ret bank2.BalancesReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					gasUsed := Max(bank2.GasBalances, len(ret.Balances)*bank2.GasBalances)
					// Here increasing the GasBalanceOf will increase the use of gas so they will never be equal
					Expect(gasUsed).To(BeNumerically("<=", ethRes.GasUsed))
				})
			})

			Context("totalSupply query", func() {
				It("should return the correct total supply", func() {
					queryArgs := getTxAndCallArgs(contractCall, contractData)
					supplyArgs := &bank2.TotalSupplyCall{}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.TotalSupplyReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(ret.TotalSupply[0].Amount.String()).To(Equal(cosmosEVMTotalSupply.String()))
					Expect(ret.TotalSupply[1].Amount.String()).To(Equal(xmplTotalSupply.String()))
				})
			})

			Context("supplyOf query", func() {
				It("should return the supply of Cosmos EVM", func() {
					queryArgs := getTxAndCallArgs(contractCall, contractData)
					supplyArgs := &bank2.SupplyOfCall{Erc20Address: is.cosmosEVMAddr}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.SupplyOfReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(ret.TotalSupply.String()).To(Equal(cosmosEVMTotalSupply.String()))
				})

				It("should return the supply of XMPL", func() {
					queryArgs := getTxAndCallArgs(contractCall, contractData)
					supplyArgs := &bank2.SupplyOfCall{Erc20Address: is.xmplAddr}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.SupplyOfReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(ret.TotalSupply.String()).To(Equal(xmplTotalSupply.String()))
				})

				It("should return a supply of 0 for a non existing token", func() {
					queryArgs := getTxAndCallArgs(contractCall, contractData)
					supplyArgs := &bank2.SupplyOfCall{Erc20Address: utiltx.GenerateAddress()}
					_, ethRes, err := is.factory.CallContractAndCheckLogs(sender.Priv, queryArgs, supplyArgs, passCheck)
					Expect(err).ToNot(HaveOccurred(), "unexpected result calling contract")

					var ret bank2.SupplyOfReturn
					_, err = ret.Decode(ethRes.Ret)
					Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")

					Expect(ret.TotalSupply.Int64()).To(Equal(big.NewInt(0).Int64()))
				})

				It("should consume the correct amount of gas", func() {
					queryArgs := getTxAndCallArgs(contractCall, contractData)
					supplyArgs := &bank2.SupplyOfCall{Erc20Address: is.xmplAddr}
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
