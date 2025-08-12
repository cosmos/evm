package erc20

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/testutil/integration/base/factory"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	"github.com/cosmos/evm/x/erc20/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func TestPrecompileIntegrationTestSuite(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	_ = Describe("Performing EVM transactions", Ordered, func() {
		var s *KeeperTestSuite
		BeforeEach(func() {
			s = NewKeeperTestSuite(create, options...)
			s.SetupTest()
		})

		Context("with the ERC20 module disabled", func() {
			BeforeEach(func() {
				params := types.DefaultParams()
				params.EnableErc20 = false
				err := utils.UpdateERC20Params(utils.UpdateParamsInput{
					Tf:      s.factory,
					Network: s.network,
					Pk:      s.keyring.GetPrivKey(0),
					Params:  params,
				})
				Expect(err).To(BeNil())
			})
			It("should be successful", func() {
				_, err := s.DeployContract("coin", "token", erc20Decimals)
				Expect(err).To(BeNil())
			})
		})

		Context("with the ERC20 module and EVM Hook enabled", func() {
			It("should be successful", func() {
				_, err := s.DeployContract("coin", "token", erc20Decimals)
				Expect(err).To(BeNil())
			})
		})
	})

	_ = Describe("ERC20:", Ordered, func() {
		var (
			s         *KeeperTestSuite
			contract  common.Address
			contract2 common.Address

			// moduleAcc is the address of the ERC-20 module account
			moduleAcc = authtypes.NewModuleAddress(types.ModuleName)
			amt       = math.NewInt(100)
		)

		BeforeEach(func() {
			s = NewKeeperTestSuite(create, options...)
			s.SetupTest()
		})

		Describe("Submitting a token pair proposal through governance", func() {
			Context("with deployed contracts", func() {
				BeforeEach(func() {
					var err error
					contract, err = s.DeployContract(erc20Name, erc20Symbol, erc20Decimals)
					Expect(err).To(BeNil())
					contract2, err = s.DeployContract(erc20Name, erc20Symbol, erc20Decimals)
					Expect(err).To(BeNil())
				})

				Describe("for a single ERC20 token", func() {
					BeforeEach(func() {
						// register erc20
						_, err := utils.RegisterERC20(
							s.factory,
							s.network,
							utils.ERC20RegistrationData{
								Addresses:    []string{contract.Hex()},
								ProposerPriv: s.keyring.GetPrivKey(0),
							},
						)
						Expect(err).To(BeNil())
					})

					It("should create a token pair owned by the contract deployer", func() {
						qc := s.network.GetERC20Client()

						res, err := qc.TokenPairs(s.network.GetContext(), &types.QueryTokenPairsRequest{})
						Expect(err).To(BeNil())

						tokenPairs := res.TokenPairs
						Expect(tokenPairs).To(HaveLen(2))
						for i, tokenPair := range tokenPairs {
							if tokenPair.Erc20Address == contract.Hex() {
								Expect(tokenPairs[i].ContractOwner).To(Equal(types.OWNER_EXTERNAL))
							}
						}
					})
				})

				Describe("for multiple ERC20 tokens", func() {
					BeforeEach(func() {
						// register erc20 tokens
						_, err := utils.RegisterERC20(
							s.factory,
							s.network,
							utils.ERC20RegistrationData{
								Addresses:    []string{contract.Hex(), contract2.Hex()},
								ProposerPriv: s.keyring.GetPrivKey(0),
							},
						)
						Expect(err).To(BeNil())
					})

					It("should create a token pairs owned by the contract deployer", func() {
						qc := s.network.GetERC20Client()
						res, err := qc.TokenPairs(s.network.GetContext(), &types.QueryTokenPairsRequest{})
						Expect(err).To(BeNil())

						tokenPairs := res.TokenPairs
						Expect(tokenPairs).To(HaveLen(3))
						for i, tokenPair := range tokenPairs {
							if tokenPair.Erc20Address == contract2.Hex() {
								Expect(tokenPairs[i].ContractOwner).To(Equal(types.OWNER_EXTERNAL))
							}
						}
					})
				})
			})
		})

		Describe("Converting", func() {
			Context("with a registered ERC20", func() {
				BeforeEach(func() {
					var err error
					contract, err = s.setupRegisterERC20Pair(contractMinterBurner)
					Expect(err).To(BeNil())

					res, err := s.MintERC20Token(contract, s.keyring.GetAddr(0), big.NewInt(amt.Int64()))
					Expect(err).To(BeNil())
					Expect(res.IsOK()).To(BeTrue())
				})

				Describe("an ERC20 token into a Cosmos coin", func() {
					BeforeEach(func() {
						// convert ERC20 to cosmos coin
						msg := types.NewMsgConvertERC20(amt, s.keyring.GetAccAddr(0), contract, s.keyring.GetAddr(0))
						res, err := s.factory.CommitCosmosTx(s.keyring.GetPrivKey(0), factory.CosmosTxArgs{Msgs: []sdk.Msg{msg}})
						Expect(err).To(BeNil())
						Expect(res.IsOK()).To(BeTrue())
					})

					It("should decrease tokens on the sender account", func() {
						balanceERC20, err := s.BalanceOf(contract, s.keyring.GetAddr(0))
						Expect(err).To(BeNil())
						Expect(balanceERC20.(*big.Int).Int64()).To(Equal(int64(0)))
					})

					It("should escrow tokens on the module account", func() {
						moduleAddr := common.BytesToAddress(moduleAcc.Bytes())
						balanceERC20, err := s.BalanceOf(contract, moduleAddr)
						Expect(err).To(BeNil())
						Expect(balanceERC20.(*big.Int).Int64()).To(Equal(amt.Int64()))
					})

					It("should send coins to the receiver account", func() {
						balRes, err := s.handler.GetBalanceFromBank(s.keyring.GetAccAddr(0), types.CreateDenom(contract.Hex()))
						Expect(err).To(BeNil())
						balanceCoin := balRes.Balance
						Expect(balanceCoin.Amount).To(Equal(amt))
					})
				})
			})
		})

		Describe("ERC20 bytes32-metadata tokens", Ordered, func() {
			Context("with a bytes32-metadata ERC20", func() {
				BeforeEach(func() {
					var err error
					// Deploy a contract like Bytes32MetadataToken.sol that returns name()/symbol() as bytes32
					contract, err = s.setupRegisterERC20Pair(contractBytes32Metadata)
					Expect(err).To(BeNil())
				})

				It("should query name/symbol as proper UTF-8 strings", func() {
					// Call the Keeper layer directly
					data, err := s.network.App.GetErc20Keeper().QueryERC20(
						s.network.GetContext(),
						contract,
					)
					Expect(err).To(BeNil())
					// Hardcoded name/symbol in Bytes32MetadataToken.sol
					Expect(data.Name).To(Equal(erc20Name))
					Expect(data.Symbol).To(Equal(erc20Symbol))
					Expect(data.Decimals).To(Equal(erc20Decimals))
				})

				It("should convert and transfer correctly", func() {
					// Basic ERC20 test flow: mint, convert, balance check
					amt := math.NewInt(100)
					// Mint tokens
					res, err := s.MintERC20Token(contract, s.keyring.GetAddr(0), big.NewInt(amt.Int64()))
					Expect(err).To(BeNil())
					Expect(res.IsOK()).To(BeTrue())

					// Convert to Cosmos coin
					msg := types.NewMsgConvertERC20(amt, s.keyring.GetAccAddr(0), contract, s.keyring.GetAddr(0))
					cres, err := s.factory.CommitCosmosTx(s.keyring.GetPrivKey(0),
						factory.CosmosTxArgs{Msgs: []sdk.Msg{msg}})
					Expect(err).To(BeNil())
					Expect(cres.IsOK()).To(BeTrue())

					// ERC20 balance should be 0
					balERC, err := s.BalanceOf(contract, common.BytesToAddress(s.keyring.GetAccAddr(0).Bytes()))
					Expect(err).To(BeNil())
					Expect(balERC.(*big.Int).Int64()).To(Equal(int64(0)))

					// Bank balance should increase
					balCoin, err := s.handler.GetBalanceFromBank(s.keyring.GetAccAddr(0), types.CreateDenom(contract.Hex()))
					Expect(err).To(BeNil())
					Expect(balCoin.Balance.Amount).To(Equal(amt))
				})
			})
		})
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "ERC20 Module Integration Tests")
}
