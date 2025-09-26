package eip7702

import (
	"math/big"
	"testing"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func TestEIP7702IntegrationTestSuite(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	_ = Describe("Send transaction using smart wallet set by eip7702 SetCode", func() {
		var (
			s     *IntegrationTestSuite
			user0 keyring.Key
			user1 keyring.Key
		)

		BeforeEach(func() {
			s = NewIntegrationTestSuite(create, options...)
			s.SetupTest()

			user0 = s.keyring.GetKey(0)
			user1 = s.keyring.GetKey(1)
		})

		Context("Calling erc20 contract methods", func() {
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
				txArgs = evmtypes.EvmTxArgs{
					To:       &s.entryPointAddr,
					Nonce:    acc.GetNonce(),
					GasLimit: DefaultGasLimit,
				}
				callArgs = testutiltypes.CallArgs{
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
						var balance0, balance1 *big.Int
						transferAmount := big.NewInt(1000)
						initialBalance := new(big.Int).Mul(big.NewInt(1e6), (big.NewInt(int64(1e18))))

						// Check User0 Balance
						txArgs = evmtypes.EvmTxArgs{
							To: &s.erc20Addr,
						}
						callArgs = testutiltypes.CallArgs{
							ContractABI: s.erc20Contract.ABI,
							MethodName:  "balanceOf",
							Args:        []interface{}{user0.Addr},
						}
						ethRes, err := s.factory.QueryContract(txArgs, callArgs, DefaultGasLimit)
						Expect(err).To(BeNil(), "error while calling erc20 balanceOf")

						err = s.erc20Contract.ABI.UnpackIntoInterface(&balance0, "balanceOf", ethRes.Ret)
						Expect(err).To(BeNil(), "error while unpacking return data of erc20 balanceOf")
						Expect(balance0).To(Equal(new(big.Int).Sub(initialBalance, transferAmount)))
						Expect(s.network.NextBlock()).To(BeNil())

						// Check User1 Balance
						callArgs.Args = []interface{}{user1.Addr}
						ethRes, err = s.factory.QueryContract(txArgs, callArgs, DefaultGasLimit)
						Expect(err).To(BeNil(), "error while calling erc20 balanceOf")

						err = s.erc20Contract.ABI.UnpackIntoInterface(&balance1, "balanceOf", ethRes.Ret)
						Expect(err).To(BeNil(), "error while unpacking return data of erc20 balanceOf")
						Expect(balance1.String()).To(Equal(transferAmount.String()))
						Expect(s.network.NextBlock()).To(BeNil())
					},
				}),
			)
		})
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "EIP7702 Integration Test Suite")
}
