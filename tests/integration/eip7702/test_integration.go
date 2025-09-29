package eip7702

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	utiltx "github.com/cosmos/evm/testutil/tx"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func TestEIP7702IntegrationTestSuite(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	var (
		s *IntegrationTestSuite

		validChainID   uint64
		invalidChainID uint64

		user0 keyring.Key
		user1 keyring.Key
	)

	BeforeEach(func() {
		s = NewIntegrationTestSuite(create, options...)
		s.SetupTest()

		validChainID = evmtypes.GetChainConfig().GetChainId()
		invalidChainID = 1234

		user0 = s.keyring.GetKey(0)
		user1 = s.keyring.GetKey(1)
	})

	Describe("test SetCode tx with diverse SetCodeAuthorization", func() {
		Context("if ChainID is invalid", func() {
			It("should fail", func() {
				acc0, err := s.grpcHandler.GetEvmAccount(user0.Addr)
				Expect(err).To(BeNil())

				authorization := s.createSetCodeAuthorization(invalidChainID, acc0.GetNonce()+1, s.smartWalletAddr)
				signedAuthorization, err := s.signSetCodeAuthorization(user0, authorization)
				Expect(err).To(BeNil())

				_, err = s.sendSetCodeTx(user0, signedAuthorization)
				Expect(err).To(BeNil(), "error while sending SetCode tx")
				Expect(s.network.NextBlock()).To(BeNil())

				s.checkSetCode(user0, s.smartWalletAddr, false)
			})
		})

		// Even if we create SetCodeAuthorization with invalid contract address, SetCode tx succeeds.
		// It just fails when sending tx with method call input.
		Context("if input address is invalid address", func() {
			It("should succeed", func() {
				acc0, err := s.grpcHandler.GetEvmAccount(user0.Addr)
				Expect(err).To(BeNil())

				invalidAddr := common.BytesToAddress([]byte("invalid"))

				authorization := s.createSetCodeAuthorization(validChainID, acc0.GetNonce()+1, invalidAddr)
				signedAuthorization, err := s.signSetCodeAuthorization(user0, authorization)
				Expect(err).To(BeNil())

				_, err = s.sendSetCodeTx(user0, signedAuthorization)
				Expect(err).To(BeNil(), "error while sending SetCode tx")
				Expect(s.network.NextBlock()).To(BeNil())

				s.checkSetCode(user0, invalidAddr, true)
			})
		})

		Context("if input address is inexisting acount address", func() {
			It("should succeed", func() {
				acc0, err := s.grpcHandler.GetEvmAccount(user0.Addr)
				Expect(err).To(BeNil())

				inexistingAddr := utiltx.GenerateAddress()

				authorization := s.createSetCodeAuthorization(validChainID, acc0.GetNonce()+1, inexistingAddr)
				signedAuthorization, err := s.signSetCodeAuthorization(user0, authorization)
				Expect(err).To(BeNil())

				_, err = s.sendSetCodeTx(user0, signedAuthorization)
				Expect(err).To(BeNil(), "error while sending SetCode tx")
				Expect(s.network.NextBlock()).To(BeNil())

				s.checkSetCode(user0, inexistingAddr, true)
			})
		})

		Context("if input address is EoA address", func() {
			It("should succeed", func() {
				acc0, err := s.grpcHandler.GetEvmAccount(user0.Addr)
				Expect(err).To(BeNil())

				authorization := s.createSetCodeAuthorization(validChainID, acc0.GetNonce()+1, user1.Addr)
				signedAuthorization, err := s.signSetCodeAuthorization(user0, authorization)
				Expect(err).To(BeNil())

				_, err = s.sendSetCodeTx(user0, signedAuthorization)
				Expect(err).To(BeNil(), "error while sending SetCode tx")
				Expect(s.network.NextBlock()).To(BeNil())

				s.checkSetCode(user0, user1.Addr, true)
			})
		})

		Context("if input address is SELFDESTRUCTED address", func() {
			It("should succeed", func() {
				stateDB := s.network.GetStateDB()
				sdAddr := utiltx.GenerateAddress()
				stateDB.CreateAccount(sdAddr)
				stateDB.SetCode(sdAddr, []byte{0x60, 0x00})
				stateDB.SelfDestruct(sdAddr)
				Expect(stateDB.Commit()).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				acc0, err := s.grpcHandler.GetEvmAccount(user0.Addr)
				Expect(err).To(BeNil())

				authorization := s.createSetCodeAuthorization(validChainID, acc0.GetNonce()+1, sdAddr)
				signedAuthorization, err := s.signSetCodeAuthorization(user0, authorization)
				Expect(err).To(BeNil())

				_, err = s.sendSetCodeTx(user0, signedAuthorization)
				Expect(err).To(BeNil(), "error while sending SetCode tx")
				Expect(s.network.NextBlock()).To(BeNil())

				s.checkSetCode(user0, sdAddr, true)
			})
		})

		When("sender of SetCodeTx is same with signer of SetCodeAuthorization", func() {
			Context("if current nonce is set to SetCodeAuthorization", func() {
				It("should fail", func() {
					acc0, err := s.grpcHandler.GetEvmAccount(user0.Addr)
					Expect(err).To(BeNil())

					authorization := s.createSetCodeAuthorization(validChainID, acc0.GetNonce(), s.smartWalletAddr)
					signedAuthorization, err := s.signSetCodeAuthorization(user0, authorization)
					Expect(err).To(BeNil())

					_, err = s.sendSetCodeTx(user0, signedAuthorization)
					Expect(err).To(BeNil(), "error while sending SetCode tx")
					Expect(s.network.NextBlock()).To(BeNil())

					s.checkSetCode(user0, s.smartWalletAddr, false)
				})
			})

			Context("if current nonce + 1 is set to SetCodeAuthorization", func() {
				It("should succeed", func() {
					acc0, err := s.grpcHandler.GetEvmAccount(user0.Addr)
					Expect(err).To(BeNil())

					authorization := s.createSetCodeAuthorization(validChainID, acc0.GetNonce()+1, s.smartWalletAddr)
					signedAuthorization, err := s.signSetCodeAuthorization(user0, authorization)
					Expect(err).To(BeNil())

					_, err = s.sendSetCodeTx(user0, signedAuthorization)
					Expect(err).To(BeNil(), "error is expected while sending SetCode tx")
					Expect(s.network.NextBlock()).To(BeNil())

					s.checkSetCode(user0, s.smartWalletAddr, true)
				})
			})
		})

		When("sender of SetCodeTx is different with singer of SetCodeAuthorization", func() {
			Context("if current nonce is set to SetCodeAuthorization", func() {
				It("should succeed", func() {
					acc1, err := s.grpcHandler.GetEvmAccount(user1.Addr)
					Expect(err).To(BeNil())

					authorization := s.createSetCodeAuthorization(validChainID, acc1.GetNonce(), s.smartWalletAddr)
					signedAuthorization, err := s.signSetCodeAuthorization(user1, authorization)
					Expect(err).To(BeNil())

					_, err = s.sendSetCodeTx(user0, signedAuthorization)
					Expect(err).To(BeNil(), "error is expected while sending SetCode tx")
					Expect(s.network.NextBlock()).To(BeNil())

					s.checkSetCode(user1, s.smartWalletAddr, true)
				})
			})

			Context("if current nonce + 1 is set to SetCodeAuthorization", func() {
				It("should fail", func() {
					acc1, err := s.grpcHandler.GetEvmAccount(user1.Addr)
					Expect(err).To(BeNil())

					authorization := s.createSetCodeAuthorization(validChainID, acc1.GetNonce()+1, s.smartWalletAddr)
					signedAuthorization, err := s.signSetCodeAuthorization(user0, authorization)
					Expect(err).To(BeNil())

					_, err = s.sendSetCodeTx(user0, signedAuthorization)
					Expect(err).To(BeNil(), "error is expected while sending SetCode tx")
					Expect(s.network.NextBlock()).To(BeNil())

					s.checkSetCode(user1, s.smartWalletAddr, false)
				})
			})
		})
	})

	Describe("test simple user operation using smart wallet set by eip7702 SetCode", func() {
		BeforeEach(func() {
			s.SetupSmartWallet()
		})

		Context("calling erc20 contract methods", func() {
			type TestCase struct {
				makeCalldata func() []byte
				postCheck    func()
			}

			DescribeTable("", func(tc TestCase) {
				// Make contract call calldata
				calldata := tc.makeCalldata()

				// Make smart wallet execute method calldata
				value := big.NewInt(0)
				swCalldata, err := s.smartWalletContract.ABI.Pack(
					"execute", s.erc20Addr, value, calldata,
				)
				Expect(err).To(BeNil(), "error while abi packing smart wallet execute calldata")

				// Get Nonce
				acc, err := s.grpcHandler.GetEvmAccount(user0.Addr)
				Expect(err).To(BeNil(), "failed to get account")

				// Make UserOperation
				userOp := NewUserOperation(user0.Addr, acc.GetNonce(), swCalldata)

				// Sign UserOperation
				userOp, err = SignUserOperation(userOp, s.entryPointAddr, user0.Priv)
				Expect(err).To(BeNil(), "failed to sign UserOperation")

				// Handle UserOperation
				txArgs := evmtypes.EvmTxArgs{
					To:       &s.entryPointAddr,
					Nonce:    acc.GetNonce(),
					GasLimit: DefaultGasLimit,
				}
				callArgs := testutiltypes.CallArgs{
					ContractABI: s.entryPointContract.ABI,
					MethodName:  "handleOps",
					Args: []interface{}{
						[]UserOperation{*userOp},
					},
				}
				eventCheck := logCheck.WithExpEvents("UserOperationEvent", "Transfer")
				_, _, err = s.factory.CallContractAndCheckLogs(user0.Priv, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil(), "error while calling handleOps")
				Expect(s.network.NextBlock()).To(BeNil())

				// verify state after execution of UserOperation
				tc.postCheck()
			},
				Entry("", TestCase{
					makeCalldata: func() []byte {
						transferAmount := big.NewInt(1000)
						calldata, err := s.erc20Contract.ABI.Pack(
							"transfer", user1.Addr, transferAmount,
						)
						Expect(err).To(BeNil(), "error while abi packing erc20 transfer calldata")
						return calldata
					},
					postCheck: func() {
						transferAmount := big.NewInt(1000)
						initialBalance := new(big.Int).Mul(big.NewInt(1e6), (big.NewInt(int64(1e18))))

						s.checkERC20Balance(user0.Addr, new(big.Int).Sub(initialBalance, transferAmount))
						s.checkERC20Balance(user1.Addr, transferAmount)
					},
				}),
			)
		})
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "EIP7702 Integration Test Suite")
}
