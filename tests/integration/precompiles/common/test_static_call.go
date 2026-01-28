package common

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/precompiles/werc20"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

type StaticCallTestSuite struct {
	suite.Suite

	create       network.CreateEvmApp
	network      *network.UnitTestNetwork
	factory      factory.TxFactory
	grpcHandler  grpc.Handler
	keyring      testkeyring.Keyring
	contractAddr common.Address
	werc20Addr   common.Address
}

func NewStaticCallTestSuite(create network.CreateEvmApp) *StaticCallTestSuite {
	return &StaticCallTestSuite{
		create: create,
	}
}

func (s *StaticCallTestSuite) SetupTest() {
	keyring := testkeyring.New(2)
	nw := network.NewUnitTestNetwork(
		s.create,
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	)
	grpcHandler := grpc.NewIntegrationHandler(nw)
	txFactory := factory.New(nw, grpcHandler)

	s.network = nw
	s.factory = txFactory
	s.grpcHandler = grpcHandler
	s.keyring = keyring

	werc20Hex := network.GetWEVMOSContractHex(testconstants.ExampleChainID)
	s.werc20Addr = common.HexToAddress(werc20Hex)

	// Deploy StaticCallTester contract
	contractAddr, err := s.factory.DeployContract(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        contracts.StaticCallTesterContract,
			ConstructorArgs: []interface{}{s.werc20Addr},
		},
	)
	s.Require().NoError(err, "failed to deploy StaticCallTester")
	s.Require().NoError(s.network.NextBlock())
	s.contractAddr = contractAddr

	// Fund the contract with WERC20 tokens.
	// Step 1: Deposit native tokens into WERC20 for keyring[0].
	depositAmount := big.NewInt(1e18)
	_, err = s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{
			To:     &s.werc20Addr,
			Amount: depositAmount,
		},
		testutiltypes.CallArgs{
			ContractABI: werc20.ABI,
			MethodName:  "deposit",
		},
	)
	s.Require().NoError(err, "failed to deposit into WERC20")
	s.Require().NoError(s.network.NextBlock())

	// Step 2: Transfer WERC20 tokens from keyring[0] to the deployed contract.
	_, err = s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{To: &s.werc20Addr},
		testutiltypes.CallArgs{
			ContractABI: werc20.ABI,
			MethodName:  "transfer",
			Args:        []interface{}{s.contractAddr, depositAmount},
		},
	)
	s.Require().NoError(err, "failed to transfer WERC20 to contract")
	s.Require().NoError(s.network.NextBlock())
}

func (s *StaticCallTestSuite) TestNonStaticPrecompileCall() {
	transferAmount := big.NewInt(1e17)

	res, err := s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{To: &s.contractAddr},
		testutiltypes.CallArgs{
			ContractABI: contracts.StaticCallTesterContract.ABI,
			MethodName:  "nonStaticPrecompileTransfer",
			Args:        []interface{}{s.keyring.GetAddr(1), transferAmount},
		},
	)
	s.Require().NoError(err, "non-static precompile transfer should succeed")

	ethRes, err := evmtypes.DecodeTxResponse(res.Data)
	s.Require().NoError(err)
	s.Require().False(ethRes.Failed(), "tx should not have VM error")

	// Decode return value — should be false (staticcall blocked by write protection)
	out, err := contracts.StaticCallTesterContract.ABI.Unpack("nonStaticPrecompileTransfer", ethRes.Ret)
	s.Require().NoError(err)
	s.Require().Len(out, 1)
	success, ok := out[0].(bool)
	s.Require().True(ok)
	s.Require().True(success, "call to precompile transfer should return true")
}

func (s *StaticCallTestSuite) TestStaticPrecompileCall() {
	transferAmount := big.NewInt(1e17)

	res, err := s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{To: &s.contractAddr},
		testutiltypes.CallArgs{
			ContractABI: contracts.StaticCallTesterContract.ABI,
			MethodName:  "staticPrecompileTransfer",
			Args:        []interface{}{s.keyring.GetAddr(1), transferAmount},
		},
	)
	s.Require().NoError(err, "outer tx should succeed")

	ethRes, err := evmtypes.DecodeTxResponse(res.Data)
	s.Require().NoError(err)
	s.Require().False(ethRes.Failed())

	// Decode return value — should be false (staticcall blocked by write protection)
	out, err := contracts.StaticCallTesterContract.ABI.Unpack("staticPrecompileTransfer", ethRes.Ret)
	s.Require().NoError(err)
	s.Require().Len(out, 1)
	success, ok := out[0].(bool)
	s.Require().True(ok)
	s.Require().False(success, "staticcall to precompile transfer should return false")
}

func (s *StaticCallTestSuite) TestNonStaticContractCall() {
	res, err := s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{To: &s.contractAddr},
		testutiltypes.CallArgs{
			ContractABI: contracts.StaticCallTesterContract.ABI,
			MethodName:  "nonStaticIncrement",
		},
	)
	s.Require().NoError(err, "non-static increment should succeed")

	ethRes, err := evmtypes.DecodeTxResponse(res.Data)
	s.Require().NoError(err)
	s.Require().False(ethRes.Failed())
	s.Require().NoError(s.network.NextBlock())

	// Query counter — should be 1
	queryRes, err := s.factory.QueryContract(
		evmtypes.EvmTxArgs{To: &s.contractAddr},
		testutiltypes.CallArgs{
			ContractABI: contracts.StaticCallTesterContract.ABI,
			MethodName:  "getCounter",
		},
		0,
	)
	s.Require().NoError(err)

	out, err := contracts.StaticCallTesterContract.ABI.Unpack("getCounter", queryRes.Ret)
	s.Require().NoError(err)
	s.Require().Len(out, 1)
	counter, ok := out[0].(*big.Int)
	s.Require().True(ok)
	s.Require().Equal(int64(1), counter.Int64(), "counter should be 1 after increment")
}

func (s *StaticCallTestSuite) TestStaticContractCall() {
	res, err := s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{To: &s.contractAddr},
		testutiltypes.CallArgs{
			ContractABI: contracts.StaticCallTesterContract.ABI,
			MethodName:  "staticIncrement",
		},
	)
	s.Require().NoError(err, "outer tx should succeed")

	ethRes, err := evmtypes.DecodeTxResponse(res.Data)
	s.Require().NoError(err)
	s.Require().False(ethRes.Failed())

	// Decode return value — should be false (staticcall blocked by SSTORE protection)
	out, err := contracts.StaticCallTesterContract.ABI.Unpack("staticIncrement", ethRes.Ret)
	s.Require().NoError(err)
	s.Require().Len(out, 1)
	success, ok := out[0].(bool)
	s.Require().True(ok)
	s.Require().False(success, "staticcall to increment should return false")
	s.Require().NoError(s.network.NextBlock())

	// Query counter — should still be 0
	queryRes, err := s.factory.QueryContract(
		evmtypes.EvmTxArgs{To: &s.contractAddr},
		testutiltypes.CallArgs{
			ContractABI: contracts.StaticCallTesterContract.ABI,
			MethodName:  "getCounter",
		},
		0,
	)
	s.Require().NoError(err)

	out, err = contracts.StaticCallTesterContract.ABI.Unpack("getCounter", queryRes.Ret)
	s.Require().NoError(err)
	s.Require().Len(out, 1)
	counter, ok := out[0].(*big.Int)
	s.Require().True(ok)
	s.Require().Equal(int64(0), counter.Int64(), "counter should be unchanged after static increment")
}

// TestCrossContractViewCallWriteProtection tests that precompile state changes
// are blocked when called through a cross-contract STATICCALL chain:
// 1. ViewCaller invokes StaticCallbackTarget.callback() via ICallback interface (STATICCALL)
// 2. Inside callback(), StaticCallbackTarget attempts werc20.transfer() via regular CALL
// 3. The transfer should fail because the static context propagates through the call chain
func (s *StaticCallTestSuite) TestCrossContractViewCallWriteProtection() {
	// Deploy StaticCallbackTarget contract
	targetAddr, err := s.factory.DeployContract(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        contracts.StaticCallbackTargetContract,
			ConstructorArgs: []interface{}{s.werc20Addr},
		},
	)
	s.Require().NoError(err, "failed to deploy StaticCallbackTarget")
	s.Require().NoError(s.network.NextBlock())

	// Deploy ViewCaller contract
	callerAddr, err := s.factory.DeployContract(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract: contracts.ViewCallerContract,
		},
	)
	s.Require().NoError(err, "failed to deploy ViewCaller")
	s.Require().NoError(s.network.NextBlock())

	// Fund the target contract with WERC20 tokens
	fundAmount := big.NewInt(1e18)

	// Deposit native tokens into WERC20
	_, err = s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{
			To:     &s.werc20Addr,
			Amount: fundAmount,
		},
		testutiltypes.CallArgs{
			ContractABI: werc20.ABI,
			MethodName:  "deposit",
		},
	)
	s.Require().NoError(err, "failed to deposit into WERC20")
	s.Require().NoError(s.network.NextBlock())

	// Transfer WERC20 tokens to target contract
	_, err = s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{To: &s.werc20Addr},
		testutiltypes.CallArgs{
			ContractABI: werc20.ABI,
			MethodName:  "transfer",
			Args:        []interface{}{targetAddr, fundAmount},
		},
	)
	s.Require().NoError(err, "failed to transfer WERC20 to target")
	s.Require().NoError(s.network.NextBlock())

	// Set up transfer parameters
	transferAmount := big.NewInt(1e17)
	_, err = s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{To: &targetAddr},
		testutiltypes.CallArgs{
			ContractABI: contracts.StaticCallbackTargetContract.ABI,
			MethodName:  "setTransferParams",
			Args:        []interface{}{s.keyring.GetAddr(1), transferAmount},
		},
	)
	s.Require().NoError(err, "failed to set transfer params")
	s.Require().NoError(s.network.NextBlock())

	// Check target's initial WERC20 balance
	initialBalanceRes, err := s.factory.QueryContract(
		evmtypes.EvmTxArgs{To: &s.werc20Addr},
		testutiltypes.CallArgs{
			ContractABI: werc20.ABI,
			MethodName:  "balanceOf",
			Args:        []interface{}{targetAddr},
		},
		0,
	)
	s.Require().NoError(err)
	initialBalanceOut, err := werc20.ABI.Unpack("balanceOf", initialBalanceRes.Ret)
	s.Require().NoError(err)
	initialBalance := initialBalanceOut[0].(*big.Int)

	// Execute: ViewCaller calls StaticCallbackTarget.callback() via ICallback (STATICCALL)
	// Inside callback(), target attempts werc20.transfer()
	res, err := s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{To: &callerAddr},
		testutiltypes.CallArgs{
			ContractABI: contracts.ViewCallerContract.ABI,
			MethodName:  "callViewAndReturn",
			Args:        []interface{}{targetAddr},
		},
	)
	s.Require().NoError(err, "outer tx should succeed")

	ethRes, err := evmtypes.DecodeTxResponse(res.Data)
	s.Require().NoError(err)
	s.Require().False(ethRes.Failed(), "tx should not have top-level VM error")
	s.Require().NoError(s.network.NextBlock())

	// Decode the return value - should be "TRANSFER_FAILED"
	out, err := contracts.ViewCallerContract.ABI.Unpack("callViewAndReturn", ethRes.Ret)
	s.Require().NoError(err)
	s.Require().Len(out, 1)
	result, ok := out[0].(string)
	s.Require().True(ok)
	s.Require().Equal("TRANSFER_FAILED", result,
		"transfer inside STATICCALL context should fail; got %q", result)

	// Verify the balance didn't change (transfer was blocked)
	finalBalanceRes, err := s.factory.QueryContract(
		evmtypes.EvmTxArgs{To: &s.werc20Addr},
		testutiltypes.CallArgs{
			ContractABI: werc20.ABI,
			MethodName:  "balanceOf",
			Args:        []interface{}{targetAddr},
		},
		0,
	)
	s.Require().NoError(err)
	finalBalanceOut, err := werc20.ABI.Unpack("balanceOf", finalBalanceRes.Ret)
	s.Require().NoError(err)
	finalBalance := finalBalanceOut[0].(*big.Int)

	s.Require().Equal(initialBalance.String(), finalBalance.String(),
		"target balance should be unchanged")
}
