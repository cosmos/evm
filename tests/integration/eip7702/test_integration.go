package eip7702

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func TestEIP7702IntegrationTestSuite(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	var (
		s *IntegrationTestSuite

		validChainID   uint64
		invalidChainID uint64

		user0 keyring.Key
		user1 keyring.Key
		user2 keyring.Key
	)

	BeforeEach(func() {
		s = NewIntegrationTestSuite(create, options...)
		s.SetupTest()

		validChainID = evmtypes.GetChainConfig().GetChainId()
		invalidChainID = 1234

		user0 = s.keyring.GetKey(0)
		user1 = s.keyring.GetKey(1)
		user2 = s.keyring.GetKey(2)
	})

	Describe("test SetCode tx with diverse SetCodeAuthorization", func() {
		Context("if ChainID is invalid", func() {
			It("should fail", func() {
				acc0, err := s.grpcHandler.GetEvmAccount(user0.Addr)
				Expect(err).To(BeNil())

				authorization := s.createSetCodeAuthorization(invalidChainID, acc0.GetNonce()+1, s.smartWalletAddr)
				signedAuthorization, err := s.signSetCodeAuthorization(user0, authorization)
				Expect(err).To(BeNil())

				err = s.sendSetCodeTx(user0, signedAuthorization)
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

				err = s.sendSetCodeTx(user0, signedAuthorization)
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

				err = s.sendSetCodeTx(user0, signedAuthorization)
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

				err = s.sendSetCodeTx(user0, signedAuthorization)
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

				err = s.sendSetCodeTx(user0, signedAuthorization)
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

					err = s.sendSetCodeTx(user0, signedAuthorization)
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

					err = s.sendSetCodeTx(user0, signedAuthorization)
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

					err = s.sendSetCodeTx(user0, signedAuthorization)
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

					err = s.sendSetCodeTx(user0, signedAuthorization)
					Expect(err).To(BeNil(), "error is expected while sending SetCode tx")
					Expect(s.network.NextBlock()).To(BeNil())

					s.checkSetCode(user1, s.smartWalletAddr, false)
				})
			})
		})
	})

	Describe("test simple user operation using smart wallet set by eip7702 SetCode", func() {
		var (
			user0Balance *big.Int
			user1Balance *big.Int
			user2Balance *big.Int
		)

		BeforeEach(func() {
			s.SetupSmartWallet()

			user0Balance = s.getERC20Balance(user0.Addr)
			user1Balance = s.getERC20Balance(user1.Addr)
			user2Balance = s.getERC20Balance(user2.Addr)
		})

		type TestCase struct {
			makeUserOps func() []UserOperation
			getLogCheck func() testutil.LogCheckArgs
			postCheck   func()
		}

		DescribeTable("test single/batch UserOperations", func(tc TestCase) {
			// get userOperations and expected events
			userOps := tc.makeUserOps()
			eventCheck := tc.getLogCheck()

			// send tx
			_, _, err := s.handleUserOps(user0, userOps, eventCheck)
			Expect(err).To(BeNil(), "error while calling handleOps")
			Expect(s.network.NextBlock()).To(BeNil())

			tc.postCheck()
		},
			Entry("single user operation signed by tx sender", TestCase{
				makeUserOps: func() []UserOperation {
					transferAmount := big.NewInt(1000)
					calldata, err := s.erc20Contract.ABI.Pack(
						"transfer", user1.Addr, transferAmount,
					)
					Expect(err).To(BeNil(), "error while abi packing erc20 transfer calldata")

					value := big.NewInt(0)
					swCalldata, err := s.smartWalletContract.ABI.Pack("execute", s.erc20Addr, value, calldata)
					Expect(err).To(BeNil(), "error while abi packing smart wallet execute calldata")

					// Get Nonce
					acc, err := s.grpcHandler.GetEvmAccount(user0.Addr)
					Expect(err).To(BeNil(), "failed to get account")

					// Make UserOperation
					userOp := NewUserOperation(user0.Addr, acc.GetNonce(), swCalldata)
					userOp, err = SignUserOperation(userOp, s.entryPointAddr, user0.Priv)
					Expect(err).To(BeNil(), "failed to sign UserOperation")

					return []UserOperation{*userOp}
				},
				getLogCheck: func() testutil.LogCheckArgs {
					return logCheck.WithExpEvents("UserOperationEvent", "Transfer")
				},
				postCheck: func() {
					transferAmount := big.NewInt(1000)
					expUser0Balance := new(big.Int).Sub(new(big.Int).Set(user0Balance), transferAmount)
					expUser1Balance := new(big.Int).Add(new(big.Int).Set(user1Balance), transferAmount)

					s.checkERC20Balance(user0.Addr, expUser0Balance)
					s.checkERC20Balance(user1.Addr, expUser1Balance)
				},
			}),
			Entry("single user operation signed by other user", TestCase{
				makeUserOps: func() []UserOperation {
					transferAmount := big.NewInt(1000)
					calldata, err := s.erc20Contract.ABI.Pack(
						"transfer", user2.Addr, transferAmount,
					)
					Expect(err).To(BeNil(), "error while abi packing erc20 transfer calldata")

					value := big.NewInt(0)
					swCalldata, err := s.smartWalletContract.ABI.Pack("execute", s.erc20Addr, value, calldata)
					Expect(err).To(BeNil(), "error while abi packing smart wallet execute calldata")

					// Get Nonce
					acc1, err := s.grpcHandler.GetEvmAccount(user1.Addr)
					Expect(err).To(BeNil(), "failed to get account")

					// Make UserOperation
					userOp := NewUserOperation(user1.Addr, acc1.GetNonce(), swCalldata)
					userOp, err = SignUserOperation(userOp, s.entryPointAddr, user1.Priv)
					Expect(err).To(BeNil(), "failed to sign UserOperation")

					return []UserOperation{*userOp}
				},
				getLogCheck: func() testutil.LogCheckArgs {
					return logCheck.WithExpEvents("UserOperationEvent", "Transfer")
				},
				postCheck: func() {
					transferAmount := big.NewInt(1000)
					expUser1Balance := new(big.Int).Sub(user1Balance, transferAmount)
					expUser2Balance := new(big.Int).Add(user2Balance, transferAmount)

					s.checkERC20Balance(user1.Addr, expUser1Balance)
					s.checkERC20Balance(user2.Addr, expUser2Balance)
				},
			}),
			Entry("batch of user operations signed by tx sender", TestCase{
				makeUserOps: func() []UserOperation {
					transferAmount := big.NewInt(1000)
					calldata, err := s.erc20Contract.ABI.Pack(
						"transfer", user1.Addr, transferAmount,
					)
					Expect(err).To(BeNil(), "error while abi packing erc20 transfer calldata")

					value := big.NewInt(0)
					swCalldata, err := s.smartWalletContract.ABI.Pack("execute", s.erc20Addr, value, calldata)
					Expect(err).To(BeNil(), "error while abi packing smart wallet execute calldata")

					// Get Nonce
					acc, err := s.grpcHandler.GetEvmAccount(user0.Addr)
					Expect(err).To(BeNil(), "failed to get account")
					nonce := acc.GetNonce()

					// Make UserOperations
					userOp1 := NewUserOperation(user0.Addr, nonce, swCalldata)
					userOp1, err = SignUserOperation(userOp1, s.entryPointAddr, user0.Priv)
					Expect(err).To(BeNil(), "failed to sign UserOperation")

					nonce++
					userOp2 := NewUserOperation(user0.Addr, nonce, swCalldata)
					userOp2, err = SignUserOperation(userOp2, s.entryPointAddr, user0.Priv)
					Expect(err).To(BeNil(), "failed to sign UserOperation")

					return []UserOperation{*userOp1, *userOp2}
				},
				getLogCheck: func() testutil.LogCheckArgs {
					return logCheck.WithExpEvents(
						"UserOperationEvent", "Transfer",
						"UserOperationEvent", "Transfer",
					)
				},
				postCheck: func() {
					transferAmountX2 := big.NewInt(2000)
					expUser0Balance := new(big.Int).Sub(new(big.Int).Set(user0Balance), transferAmountX2)
					expUser1Balance := new(big.Int).Add(new(big.Int).Set(user1Balance), transferAmountX2)

					s.checkERC20Balance(user0.Addr, expUser0Balance)
					s.checkERC20Balance(user1.Addr, expUser1Balance)
				},
			}),
			Entry("batch of user operations signed by other users", TestCase{
				makeUserOps: func() []UserOperation {
					transferAmount := big.NewInt(1000)
					calldata, err := s.erc20Contract.ABI.Pack(
						"transfer", user0.Addr, transferAmount,
					)
					Expect(err).To(BeNil(), "error while abi packing erc20 transfer calldata")

					value := big.NewInt(0)
					swCalldata, err := s.smartWalletContract.ABI.Pack("execute", s.erc20Addr, value, calldata)
					Expect(err).To(BeNil(), "error while abi packing smart wallet execute calldata")

					// Make UserOperations
					// user1 -> user0
					acc1, err := s.grpcHandler.GetEvmAccount(user1.Addr)
					Expect(err).To(BeNil(), "failed to get account")

					userOp1 := NewUserOperation(user1.Addr, acc1.GetNonce(), swCalldata)
					userOp1, err = SignUserOperation(userOp1, s.entryPointAddr, user1.Priv)
					Expect(err).To(BeNil(), "failed to sign UserOperation")

					// user2 -> user0
					acc2, err := s.grpcHandler.GetEvmAccount(user2.Addr)
					Expect(err).To(BeNil(), "failed to get account")

					userOp2 := NewUserOperation(user2.Addr, acc2.GetNonce(), swCalldata)
					userOp2, err = SignUserOperation(userOp2, s.entryPointAddr, user2.Priv)
					Expect(err).To(BeNil(), "failed to sign UserOperation")

					return []UserOperation{*userOp1, *userOp2}
				},
				getLogCheck: func() testutil.LogCheckArgs {
					return logCheck.WithExpEvents(
						"UserOperationEvent", "Transfer",
						"UserOperationEvent", "Transfer",
					)
				},
				postCheck: func() {
					transferAmount := big.NewInt(1000)
					transferAmountX2 := big.NewInt(2000)
					expUser0Balance := new(big.Int).Add(new(big.Int).Set(user0Balance), transferAmountX2)
					expUser1Balance := new(big.Int).Sub(new(big.Int).Set(user1Balance), transferAmount)
					expUser2Balance := new(big.Int).Sub(new(big.Int).Set(user2Balance), transferAmount)

					s.checkERC20Balance(user0.Addr, expUser0Balance)
					s.checkERC20Balance(user1.Addr, expUser1Balance)
					s.checkERC20Balance(user2.Addr, expUser2Balance)
				},
			}),
		)
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "EIP7702 Integration Test Suite")
}
