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
	_ = Describe("", func() {
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

		Context("", func() {
			type TestCase struct {
				makeCalldata func() []byte
				postCheck    func()
			}

			DescribeTable("", func(tc TestCase) {
				calldata := tc.makeCalldata()
				nonce := s.network.App.GetEVMKeeper().GetNonce(s.network.GetContext(), user0.Addr)
				userOp := NewUserOperation(user0.Addr, nonce, calldata)

				// Sign UserOperation
				userOp, err := SignUserOperation(userOp, s.entryPointAddr, user0.Priv)
				Expect(err).To(BeNil())

				// Handle UserOperation
				txArgs = evmtypes.EvmTxArgs{
					To:       &s.entryPointAddr,
					GasLimit: DefaultGasLimit,
				}
				callArgs = testutiltypes.CallArgs{
					ContractABI: s.entryPointContract.ABI,
					MethodName:  "handleOps",
					Args: []interface{}{
						[]UserOperation{*userOp},
					},
				}
				_, _, err = s.factory.CallContractAndCheckLogs(user0.Priv, txArgs, callArgs, passCheck)
				Expect(err).To(BeNil(), "error while calling handleOps")
				Expect(s.network.NextBlock()).To(BeNil())

				tc.postCheck()
			},
				Entry("", TestCase{
					makeCalldata: func() []byte {
						transferAmount := big.NewInt(1000)
						erc20TransferCalldata, err := s.erc20Contract.ABI.Pack(
							"transfer", user1.Addr, transferAmount,
						)
						Expect(err).To(BeNil(), "error while abi packing erc20 transfer calldata")

						value := big.NewInt(0)
						calldata, err := s.smartWalletContract.ABI.Pack(
							"execute", s.erc20Addr, value, erc20TransferCalldata,
						)
						Expect(err).To(BeNil(), "error while abi packing smart wallet execute calldata")

						return calldata
					},
					postCheck: func() {
						transferAmount := big.NewInt(1000)

						// Check Balance
						txArgs = evmtypes.EvmTxArgs{
							To: &s.erc20Addr,
						}
						callArgs = testutiltypes.CallArgs{
							ContractABI: s.erc20Contract.ABI,
							MethodName:  "balanceOf",
							Args: []interface{}{
								user1.Addr,
							},
						}
						_, ethRes, err := s.factory.CallContractAndCheckLogs(user0.Priv, txArgs, callArgs, passCheck)
						Expect(err).To(BeNil(), "error while calling erc20 balanceOf")
						Expect(ethRes.Ret).NotTo(BeNil())

						var balance *big.Int
						err = s.erc20Contract.ABI.UnpackIntoInterface(balance, "balanceOf", ethRes.Ret)
						Expect(err).To(BeNil(), "error while unpacking return data of erc20 balanceOf")
						Expect(balance).To(Equal(transferAmount))
					},
				}),
			)
		})
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "EIP7702 Integration Test Suite")
}
