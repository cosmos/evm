package eips

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"

	"github.com/cosmos/evm/eips"
	"github.com/cosmos/evm/eips/testdata"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	"github.com/cosmos/evm/testutil/keyring"
	types2 "github.com/cosmos/evm/testutil/types"
	types3 "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/crypto/types"
)

// RunTests
// Below tests are divided in 3 steps:
//  1. Deploy and interact with contracts to compute the gas used BEFORE enabling
//     the EIP.
//  2. Activate the EIP under test.
//  3. Deploy and interact with contracts to compute the gas used AFTER enabling
//     the EIP.
func RunTests(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	t.Helper()
	_ = ginkgo.Describe("EIP-0000 - ", ginkgo.Ordered, func() {
		var (
			in network.Network
			tf factory.TxFactory
			gh grpc.Handler
			k  keyring.Keyring

			senderPriv  types.PrivKey
			senderPriv2 types.PrivKey
			senderAddr2 common.Address

			// Gas used before enabling the EIP.
			gasUsedPre int64
		)

		// Multiplier used to modify the opcodes associated with EIP-0.
		eipMultiplier := uint64(5)

		// The factory counter is used because it will create a new instance of
		// the counter contract, allowing to test the CREATE opcode.
		counterFactoryContract, err := testdata.LoadCounterFactoryContract()
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to load Counter Factory contract")

		deploymentData := types2.ContractDeploymentData{
			Contract:        counterFactoryContract,
			ConstructorArgs: []interface{}{},
		}

		ginkgo.BeforeAll(func() {
			k = keyring.New(2)
			opts := []network.ConfigOption{
				network.WithPreFundedAccounts(k.GetAllAccAddrs()...),
			}
			opts = append(opts, options...)
			in = network.New(create, opts...)
			gh = grpc.NewIntegrationHandler(in)
			tf = factory.New(in, gh)

			// Account used to deploy the contract before enabling the EIP.
			senderPriv = k.GetPrivKey(0)
			// Account used to deploy the contract after enabling the EIP. A second
			// account is used to avoid possible additional gas costs due to the change
			// in the Nonce.
			senderPriv2 = k.GetPrivKey(1)
			senderAddr2 = k.GetAddr(1)

			// Set extra EIPs to empty to allow testing a single modifier.
			defaultParams := types3.DefaultParams()
			defaultParams.ExtraEIPs = []int64{}

			err := utils.UpdateEvmParams(
				utils.UpdateParamsInput{
					Tf:      tf,
					Network: in,
					Pk:      senderPriv,
					Params:  defaultParams,
				},
			)
			gomega.Expect(err).To(gomega.BeNil(), "failed during update of evm params")
			gomega.Expect(in.NextBlock()).To(gomega.BeNil())
		})

		ginkgo.It("should deploy the contract before enabling the EIP", func() {
			deploymentTxArgs, err := tf.GenerateDeployContractArgs(senderAddr2, types3.EvmTxArgs{}, deploymentData)
			gomega.Expect(err).To(gomega.BeNil(), "failed to create deployment tx args")

			res, err := tf.ExecuteEthTx(senderPriv2, deploymentTxArgs)
			gomega.Expect(err).To(gomega.BeNil(), "failed during contract deployment")
			gasUsedPre = res.GasUsed
		})

		ginkgo.It("should enable the new EIP", func() {
			eips.Multiplier = eipMultiplier
			newEIP := 0o000

			qRes, err := gh.GetEvmParams()
			gomega.Expect(err).To(gomega.BeNil(), "failed during query to evm params")
			qRes.Params.ExtraEIPs = append(qRes.Params.ExtraEIPs, int64(newEIP))
			err = utils.UpdateEvmParams(
				utils.UpdateParamsInput{
					Tf:      tf,
					Network: in,
					Pk:      senderPriv,
					Params:  qRes.Params,
				},
			)
			gomega.Expect(err).To(gomega.BeNil(), "failed during update of evm params")

			gomega.Expect(in.NextBlock()).To(gomega.BeNil())

			qRes, err = gh.GetEvmParams()
			gomega.Expect(err).To(gomega.BeNil(), "failed during query to evm params")
			gomega.Expect(qRes.Params.ExtraEIPs).To(gomega.ContainElement(int64(newEIP)), "expected to have EIP 0000 in evm params")
		})

		ginkgo.It("should change CREATE opcode constant gas after enabling EIP", func() {
			qRes, err := gh.GetEvmParams()
			gomega.Expect(err).To(gomega.BeNil(), "failed to get evm params")
			_ = qRes.Params
			gasCostPre := params.CreateGas

			deploymentTxArgs, err := tf.GenerateDeployContractArgs(senderAddr2, types3.EvmTxArgs{}, deploymentData)
			gomega.Expect(err).To(gomega.BeNil(), "failed to create deployment tx args")

			res, err := tf.ExecuteEthTx(senderPriv2, deploymentTxArgs)
			gomega.Expect(err).To(gomega.BeNil(), "failed during contract deployment")
			// commit block to update sender nonce
			gomega.Expect(in.NextBlock()).To(gomega.BeNil())

			gasUsedPost := res.GasUsed

			// The difference in gas is the new cost of the opcode, minus the cost of the
			// opcode before enabling the new eip.
			gasUsedDiff := eipMultiplier*gasCostPre - gasCostPre
			expectedGas := gasUsedPre + int64(gasUsedDiff) //#nosec G115 -- int overflow is not a concern here
			gomega.Expect(gasUsedPost).To(gomega.Equal(expectedGas))
		})
	})

	_ = ginkgo.Describe("EIP0001 - ", ginkgo.Ordered, func() {
		var (
			in network.Network
			tf factory.TxFactory
			gh grpc.Handler
			k  keyring.Keyring

			senderPriv types.PrivKey

			// Gas used before enabling the EIP.
			gasUsedPre int64

			// The address of the factory counter.
			counterFactoryAddr common.Address
		)

		// Multiplier used to modify the opcodes associated with EIP_0001.
		eipMultiplier := uint64(5)
		initialCounterValue := 1

		// The counter factory contract is used to deploy a counter contract and
		// perform state transition using the CALL opcode.
		counterFactoryContract, err := testdata.LoadCounterFactoryContract()
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to load Counter Factory contract")

		ginkgo.BeforeAll(func() {
			k = keyring.New(1)
			opts := []network.ConfigOption{
				network.WithPreFundedAccounts(k.GetAllAccAddrs()...),
			}
			opts = append(opts, options...)
			in = network.New(create, opts...)
			gh = grpc.NewIntegrationHandler(in)
			tf = factory.New(in, gh)

			senderPriv = k.GetPrivKey(0)

			// Set extra EIPs to empty to allow testing a single modifier.
			defaultParams := types3.DefaultParams()
			defaultParams.ExtraEIPs = []int64{}
			err = utils.UpdateEvmParams(
				utils.UpdateParamsInput{
					Tf:      tf,
					Network: in,
					Pk:      senderPriv,
					Params:  defaultParams,
				},
			)
			gomega.Expect(err).To(gomega.BeNil(), "failed during update of evm params")

			gomega.Expect(in.NextBlock()).To(gomega.BeNil())
		})

		ginkgo.It("should deploy the contract before enabling the EIP", func() {
			counterFactoryAddr, err = tf.DeployContract(
				senderPriv,
				types3.EvmTxArgs{},
				types2.ContractDeploymentData{
					Contract:        counterFactoryContract,
					ConstructorArgs: []interface{}{},
				},
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to deploy counter factory contract")
			gomega.Expect(in.NextBlock()).To(gomega.BeNil())

			res, err := tf.ExecuteContractCall(
				senderPriv,
				types3.EvmTxArgs{To: &counterFactoryAddr},
				types2.CallArgs{
					ContractABI: counterFactoryContract.ABI,
					MethodName:  "incrementCounter",
					Args:        []interface{}{},
				},
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to increment counter value")
			gasUsedPre = res.GasUsed

			gomega.Expect(in.NextBlock()).To(gomega.BeNil())

			// Query the counter value to check proper state transition later.
			res, err = tf.ExecuteContractCall(
				senderPriv,
				types3.EvmTxArgs{To: &counterFactoryAddr},
				types2.CallArgs{
					ContractABI: counterFactoryContract.ABI,
					MethodName:  "getCounterValue",
					Args:        []interface{}{},
				},
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to get counter value")
			gomega.Expect(in.NextBlock()).To(gomega.BeNil())

			ethRes, err := types3.DecodeTxResponse(res.Data)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to decode tx response")

			unpacked, err := counterFactoryContract.ABI.Unpack(
				"getCounterValue",
				ethRes.Ret,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to unpack counter value")

			counter, ok := unpacked[0].(*big.Int)
			gomega.Expect(ok).To(gomega.BeTrue(), "failed to convert counter to big.Int")
			gomega.Expect(counter.String()).To(gomega.Equal(fmt.Sprintf("%d", initialCounterValue+1)), "counter is not correct")
		})
		ginkgo.It("should enable the new EIP", func() {
			eips.Multiplier = eipMultiplier
			newEIP := 0o001

			qRes, err := gh.GetEvmParams()
			gomega.Expect(err).To(gomega.BeNil(), "failed during query to evm params")
			qRes.Params.ExtraEIPs = append(qRes.Params.ExtraEIPs, int64(newEIP))

			err = utils.UpdateEvmParams(
				utils.UpdateParamsInput{
					Tf:      tf,
					Network: in,
					Pk:      senderPriv,
					Params:  qRes.Params,
				},
			)
			gomega.Expect(err).To(gomega.BeNil(), "failed during update of evm params")

			gomega.Expect(in.NextBlock()).To(gomega.BeNil())

			qRes, err = gh.GetEvmParams()
			gomega.Expect(err).To(gomega.BeNil(), "failed during query to evm params")
			gomega.Expect(qRes.Params.ExtraEIPs).To(gomega.ContainElement(int64(newEIP)), "expected to have eip 0001 in evm params")
		})
		ginkgo.It("should change CALL opcode constant gas after enabling EIP", func() {
			// Constant gas cost used before enabling the new EIP.
			gasCostPre := params.WarmStorageReadCostEIP2929

			res, err := tf.ExecuteContractCall(
				senderPriv,
				types3.EvmTxArgs{To: &counterFactoryAddr},
				types2.CallArgs{
					ContractABI: counterFactoryContract.ABI,
					MethodName:  "incrementCounter",
					Args:        []interface{}{},
				},
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to increment counter value")
			gasUsedPost := res.GasUsed
			gomega.Expect(in.NextBlock()).To(gomega.BeNil())

			res, err = tf.ExecuteContractCall(
				senderPriv,
				types3.EvmTxArgs{To: &counterFactoryAddr},
				types2.CallArgs{
					ContractABI: counterFactoryContract.ABI,
					MethodName:  "getCounterValue",
					Args:        []interface{}{},
				},
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to get counter value")
			gomega.Expect(in.NextBlock()).To(gomega.BeNil())

			ethRes, err := types3.DecodeTxResponse(res.Data)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to decode tx response")

			unpacked, err := counterFactoryContract.ABI.Unpack(
				"getCounterValue",
				ethRes.Ret,
			)
			gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to unpack counter value")

			counter, ok := unpacked[0].(*big.Int)
			gomega.Expect(ok).To(gomega.BeTrue(), "failed to convert counter to big.Int")
			gomega.Expect(counter.String()).To(gomega.Equal(fmt.Sprintf("%d", initialCounterValue+2)), "counter is not updated correctly")

			// The difference in gas is the new cost of the opcode, minus the cost of the
			// opcode before enabling the new eip.
			gasUsedDiff := eipMultiplier*gasCostPre - gasCostPre
			expectedGas := gasUsedPre + int64(gasUsedDiff) //#nosec G115 -- int overflow is not a concern here
			gomega.Expect(gasUsedPost).To(gomega.Equal(expectedGas))
		})
	})

	_ = ginkgo.Describe("EIP0002 - ", ginkgo.Ordered, func() {
		var (
			in network.Network
			tf factory.TxFactory
			gh grpc.Handler
			k  keyring.Keyring

			senderPriv  types.PrivKey
			senderAddr  common.Address
			senderPriv2 types.PrivKey
			senderAddr2 common.Address
			gasUsedPre  int64
		)
		// Constant gas used to modify the opcodes associated with EIP_0002.
		constantGas := uint64(500)

		counterContract, err := testdata.LoadCounterContract()
		gomega.Expect(err).ToNot(gomega.HaveOccurred(), "failed to load Counter contract")

		deploymentData := types2.ContractDeploymentData{
			Contract:        counterContract,
			ConstructorArgs: []interface{}{},
		}
		ginkgo.BeforeAll(func() {
			k = keyring.New(2)
			opts := []network.ConfigOption{
				network.WithPreFundedAccounts(k.GetAllAccAddrs()...),
			}
			opts = append(opts, options...)
			in = network.New(create, opts...)
			gh = grpc.NewIntegrationHandler(in)
			tf = factory.New(in, gh)

			// Account used to deploy the contract before enabling the EIP.
			senderPriv = k.GetPrivKey(0)
			senderAddr = k.GetAddr(0)
			// Account used to deploy the contract after enabling the EIP. A second
			// account is used to avoid possible additional gas costs due to the change
			// in the Nonce.
			senderPriv2 = k.GetPrivKey(0)
			senderAddr2 = k.GetAddr(0)

			// Set extra EIPs to empty to allow testing a single modifier.
			defaultParams := types3.DefaultParams()
			defaultParams.ExtraEIPs = []int64{}

			err = utils.UpdateEvmParams(
				utils.UpdateParamsInput{
					Tf:      tf,
					Network: in,
					Pk:      senderPriv,
					Params:  defaultParams,
				},
			)
			gomega.Expect(err).To(gomega.BeNil(), "failed during update of evm params")

			gomega.Expect(in.NextBlock()).To(gomega.BeNil())
		})

		ginkgo.It("should deploy the contract before enabling the EIP", func() {
			deploymentTxArgs, err := tf.GenerateDeployContractArgs(senderAddr, types3.EvmTxArgs{}, deploymentData)
			gomega.Expect(err).To(gomega.BeNil(), "failed to create deployment tx args")

			res, err := tf.ExecuteEthTx(senderPriv, deploymentTxArgs)
			gomega.Expect(err).To(gomega.BeNil(), "failed during contract deployment")
			gomega.Expect(in.NextBlock()).To(gomega.BeNil())

			gasUsedPre = res.GasUsed
		})

		ginkgo.It("should enable the new EIP", func() {
			eips.SstoreConstantGas = constantGas
			newEIP := 0o002

			qRes, err := gh.GetEvmParams()
			gomega.Expect(err).To(gomega.BeNil(), "failed during query to evm params")
			qRes.Params.ExtraEIPs = append(qRes.Params.ExtraEIPs, int64(newEIP))
			err = utils.UpdateEvmParams(
				utils.UpdateParamsInput{
					Tf:      tf,
					Network: in,
					Pk:      senderPriv,
					Params:  qRes.Params,
				},
			)
			gomega.Expect(err).To(gomega.BeNil(), "failed during update of evm params")

			gomega.Expect(in.NextBlock()).To(gomega.BeNil())

			qRes, err = gh.GetEvmParams()
			gomega.Expect(err).To(gomega.BeNil(), "failed during query to evm params")
			gomega.Expect(qRes.Params.ExtraEIPs).To(gomega.ContainElement(int64(newEIP)), "expected to have eip 0002 in evm params")
		})

		ginkgo.It("should change SSTORE opcode constant gas after enabling EIP", func() {
			deploymentTxArgs, err := tf.GenerateDeployContractArgs(senderAddr2, types3.EvmTxArgs{}, deploymentData)
			gomega.Expect(err).To(gomega.BeNil(), "failed to create deployment tx args")

			res, err := tf.ExecuteEthTx(senderPriv2, deploymentTxArgs)
			gomega.Expect(err).To(gomega.BeNil(), "failed during contract deployment")
			gomega.Expect(in.NextBlock()).To(gomega.BeNil())

			gasUsedPost := res.GasUsed

			// The expected gas is previous gas plus the constant gas because
			// previous this eip, SSTORE was using only the dynamic gas.
			expectedGas := gasUsedPre + int64(constantGas) //#nosec G115 -- int overflow is not a concern here
			gomega.Expect(gasUsedPost).To(gomega.Equal(expectedGas))
		})
	})

	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "EIPs Suite")
}
