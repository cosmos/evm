package statedb_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"

	//nolint:revive // okay to use dot imports for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // okay to use dot imports for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/contracts"
	testcontracts "github.com/cosmos/evm/precompiles/testutil/contracts"
	testfactory "github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	testkeyring "github.com/cosmos/evm/testutil/integration/os/keyring"
	testnetwork "github.com/cosmos/evm/testutil/integration/os/network"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestNestedEVMExtensionCall(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nested EVM Extension Call Test Suite")
}

type testCase struct {
	method                  string
	expDelegation           bool
	expSenderERC20Balance   *uint256.Int
	expContractERC20Balance *uint256.Int
}

// This test is a demonstration of the flash loan exploit that was reported.
// This happens when interacting with EVM extensions in smart contract methods,
// where a resulting state change has the same value as the original state value.
//
// Before the fix, this would result in state changes not being persisted after the EVM extension call,
// therefore leaving the loaned funds in the contract.
var _ = Describe("testing the flash loan exploit", Ordered, func() {
	var (
		keyring testkeyring.Keyring
		// NOTE: we need to use the unit test network here because we need it to instantiate the staking precompile correctly
		network *testnetwork.UnitTestNetwork
		handler grpc.Handler
		factory testfactory.TxFactory

		deployer testkeyring.Key

		erc20Addr         common.Address
		flashLoanAddr     common.Address
		flashLoanContract evmtypes.CompiledContract

		validatorToDelegateTo string

		delegatedAmountPre math.Int
	)

	mintAmount := uint256.NewInt(2e18)
	delegateAmount := big.NewInt(1e18)

	BeforeAll(func() {
		keyring = testkeyring.New(2)
		network = testnetwork.NewUnitTestNetwork(
			testnetwork.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		)
		handler = grpc.NewIntegrationHandler(network)
		factory = testfactory.New(network, handler)

		deployer = keyring.GetKey(0)

		var err error

		// Load the flash loan contract from the compiled JSON data.
		flashLoanContract, err = testcontracts.LoadFlashLoanContract()
		Expect(err).ToNot(HaveOccurred(), "failed to load flash loan contract")
	})

	BeforeEach(func() {
		valsRes, err := handler.GetBondedValidators()
		Expect(err).ToNot(HaveOccurred(), "failed to get bonded validators")

		validatorToDelegateTo = valsRes.Validators[0].OperatorAddress

		// Initial delegation of flash loan contract to the validator is 0.
		delegatedAmountPre = math.NewInt(0)

		// Deploy an ERC-20 token contract.
		erc20Addr, err = factory.DeployContract(
			deployer.Priv,
			evmtypes.EvmTxArgs{},
			testfactory.ContractDeploymentData{
				Contract:        contracts.ERC20MinterBurnerDecimalsContract,
				ConstructorArgs: []interface{}{"TestToken", "TT", uint8(18)},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to deploy ERC-20 contract")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Mint some tokens to the deployer.
		_, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "mint",
				Args: []interface{}{
					deployer.Addr, mintAmount,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to mint tokens")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Check the balance of the deployer on the ERC20 contract.
		res, err := factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "balanceOf",
				Args: []interface{}{
					deployer.Addr,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get balance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err := evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode balance of tx response")

		unpacked, err := contracts.ERC20MinterBurnerDecimalsContract.ABI.Unpack(
			"balanceOf",
			ethRes.Ret,
		)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack balance")

		balance, ok := unpacked[0].(*uint256.Int)
		Expect(ok).To(BeTrue(), "failed to convert balance to uint256.Int")
		Expect(balance.String()).To(Equal(mintAmount.String()), "balance is not correct")

		// Deploy the flash loan contract.
		flashLoanAddr, err = factory.DeployContract(
			deployer.Priv,
			evmtypes.EvmTxArgs{},
			testfactory.ContractDeploymentData{
				Contract: flashLoanContract,
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to deploy flash loan contract")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Approve the flash loan contract to spend tokens. This is required because
		// the contract will get funds from the caller to perform actions.
		_, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "approve",
				Args: []interface{}{
					flashLoanAddr, mintAmount,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to approve flash loan contract")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Check the allowance.
		res, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "allowance",
				Args: []interface{}{
					deployer.Addr, flashLoanAddr,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get allowance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err = evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode allowance tx response")

		unpacked, err = contracts.ERC20MinterBurnerDecimalsContract.ABI.Unpack(
			"allowance",
			ethRes.Ret,
		)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack allowance")

		var allowance *uint256.Int
		allowance, ok = unpacked[0].(*uint256.Int)
		Expect(ok).To(BeTrue(), "failed to convert allowance to uint256.Int")
		Expect(allowance.String()).To(Equal(mintAmount.String()), "allowance is not correct")
	})

	DescribeTable("call the flashLoan contract", func(tc testCase) {
		_, err := factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{
				To:       &flashLoanAddr,
				GasPrice: big.NewInt(900_000_000),
				GasLimit: 400_000,
				Amount:   delegateAmount,
			},
			testfactory.CallArgs{
				ContractABI: flashLoanContract.ABI,
				MethodName:  tc.method,
				Args: []interface{}{
					erc20Addr,
					validatorToDelegateTo,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to execute flash loan")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		falshLoanAccAddr := sdk.AccAddress(flashLoanAddr.Bytes())

		if tc.expDelegation {
			delRes, err := handler.GetDelegation(falshLoanAccAddr.String(), validatorToDelegateTo)
			Expect(err).ToNot(HaveOccurred(), "failed to get delegation")
			delAmtPost := delRes.DelegationResponse.Balance.Amount
			Expect(delAmtPost).To(Equal(
				delegatedAmountPre.Add(math.NewIntFromBigInt(delegateAmount))),
				"delegated amount is not correct",
			)
		} else {
			_, err := handler.GetDelegation(falshLoanAccAddr.String(), validatorToDelegateTo)
			Expect(err).To(HaveOccurred(), "failed to get delegation")
			Expect(err.Error()).To(ContainSubstring(
				fmt.Sprintf("delegation with delegator %s not found for validator %s",
					falshLoanAccAddr.String(),
					validatorToDelegateTo),
			), "delegation should not exist")
		}

		// Check the ERC20 token balance of the deployer.
		res, err := factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "balanceOf",
				Args: []interface{}{
					deployer.Addr,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get balance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err := evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode balance of tx response")

		unpacked, err := contracts.ERC20MinterBurnerDecimalsContract.ABI.Unpack(
			"balanceOf",
			ethRes.Ret,
		)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack balance")

		balance, ok := unpacked[0].(*uint256.Int)
		Expect(ok).To(BeTrue(), "failed to convert balance to uint256.Int")
		Expect(balance.String()).To(Equal(tc.expSenderERC20Balance.String()), "balance is not correct")

		// Check FlashLoan smart contract ERC20 token balance.
		res, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "balanceOf",
				Args: []interface{}{
					flashLoanAddr,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get balance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err = evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode balance of tx response")

		unpacked, err = contracts.ERC20MinterBurnerDecimalsContract.ABI.Unpack(
			"balanceOf",
			ethRes.Ret,
		)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack balance")

		balance, ok = unpacked[0].(*uint256.Int)
		Expect(ok).To(BeTrue(), "failed to convert balance to uint256.Int")
		Expect(balance.String()).To(Equal(tc.expContractERC20Balance.String()), "balance is not correct")
	},
		Entry("flashLoan method & expect delegation", testCase{
			method:                  "flashLoan",
			expDelegation:           true,
			expSenderERC20Balance:   mintAmount,
			expContractERC20Balance: uint256.NewInt(0),
		}),
		Entry("flashLoanWithRevert method - delegation reverted", testCase{
			method:                  "flashLoanWithRevert",
			expDelegation:           false,
			expSenderERC20Balance:   uint256.MustFromBig(delegateAmount),
			expContractERC20Balance: uint256.MustFromBig(delegateAmount),
		}),
	)
})
