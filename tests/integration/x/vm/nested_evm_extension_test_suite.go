package vm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/contracts"
	testcontracts "github.com/cosmos/evm/precompiles/testutil/contracts"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"
)

// NestedEVMExtensionCallSuite covers the flash loan exploit scenarios.
type NestedEVMExtensionCallSuite struct {
	suite.Suite

	create  network.CreateEvmApp
	options []network.ConfigOption

	keyring           testkeyring.Keyring
	flashLoanContract evmtypes.CompiledContract
	mintAmount        *big.Int
	delegateAmount    *big.Int

	network               *network.UnitTestNetwork
	handler               grpc.Handler
	factory               factory.TxFactory
	deployer              testkeyring.Key
	erc20Addr             common.Address
	flashLoanAddr         common.Address
	validatorToDelegateTo string
	delegatedAmountPre    math.Int
}

func NewNestedEVMExtensionCallSuite(create network.CreateEvmApp, options ...network.ConfigOption) *NestedEVMExtensionCallSuite {
	return &NestedEVMExtensionCallSuite{
		create:  create,
		options: options,
	}
}

// SetupSuite loads static data before any test
func (s *NestedEVMExtensionCallSuite) SetupSuite() {
	// load keyring with two accounts
	s.keyring = testkeyring.New(2)

	// Load the flash loan contract definition
	var err error
	s.flashLoanContract, err = testcontracts.LoadFlashLoanContract()
	s.Require().NoError(err, "failed to load flash loan contract")

	// Set amounts
	s.mintAmount = big.NewInt(0).Mul(big.NewInt(2), big.NewInt(1e18))
	s.delegateAmount = big.NewInt(1e18)
}

// SetupTest resets blockchain state before each test case or entry
func (s *NestedEVMExtensionCallSuite) SetupTest() {
	if s.options == nil {
		s.options = []network.ConfigOption{}
	}
	opts := []network.ConfigOption{
		network.WithPreFundedAccounts(s.keyring.GetAllAccAddrs()...),
	}
	opts = append(opts, s.options...)
	// fresh network, handler, factory
	s.network = network.NewUnitTestNetwork(s.create, opts...)
	s.handler = grpc.NewIntegrationHandler(s.network)
	s.factory = factory.New(s.network, s.handler)

	// deployer is first key
	s.deployer = s.keyring.GetKey(0)

	// find a validator to delegate to
	valsRes, err := s.handler.GetBondedValidators()
	s.Require().NoError(err, "failed to get bonded validators")
	s.validatorToDelegateTo = valsRes.Validators[0].OperatorAddress

	// initial delegation is zero
	s.delegatedAmountPre = math.NewInt(0)

	// Deploy an ERC20 token
	var errDeploy error
	s.erc20Addr, errDeploy = s.factory.DeployContract(
		s.deployer.Priv,
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        contracts.ERC20MinterBurnerDecimalsContract,
			ConstructorArgs: []interface{}{"TestToken", "TT", uint8(18)},
		},
	)
	s.Require().NoError(errDeploy, "failed to deploy ERC20 contract")
	s.Require().NoError(s.network.NextBlock(), "failed to commit block")

	// Mint tokens to deployer
	_, err = s.factory.ExecuteContractCall(
		s.deployer.Priv,
		evmtypes.EvmTxArgs{To: &s.erc20Addr},
		testutiltypes.CallArgs{
			ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
			MethodName:  "mint",
			Args:        []interface{}{s.deployer.Addr, s.mintAmount},
		},
	)
	s.Require().NoError(err, "failed to mint tokens")
	s.Require().NoError(s.network.NextBlock(), "failed to commit block")

	// Deploy the flash loan contract
	s.flashLoanAddr, err = s.factory.DeployContract(
		s.deployer.Priv,
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{Contract: s.flashLoanContract},
	)
	s.Require().NoError(err, "failed to deploy flash loan contract")
	// commit
	s.Require().NoError(s.network.NextBlock(), "failed to commit block")

	// Approve flash loan contract to spend tokens
	_, err = s.factory.ExecuteContractCall(
		s.deployer.Priv,
		evmtypes.EvmTxArgs{To: &s.erc20Addr},
		testutiltypes.CallArgs{
			ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
			MethodName:  "approve",
			Args:        []interface{}{s.flashLoanAddr, s.mintAmount},
		},
	)
	s.Require().NoError(err, "failed to approve flash loan contract")
	s.Require().NoError(s.network.NextBlock(), "failed to commit block")

	// Verify allowance
	res, err := s.factory.ExecuteContractCall(
		s.deployer.Priv,
		evmtypes.EvmTxArgs{To: &s.erc20Addr},
		testutiltypes.CallArgs{
			ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
			MethodName:  "allowance",
			Args:        []interface{}{s.deployer.Addr, s.flashLoanAddr},
		},
	)
	s.Require().NoError(err, "failed to get allowance")
	s.Require().NoError(s.network.NextBlock(), "failed to commit block")

	ethRes, err := evmtypes.DecodeTxResponse(res.Data)
	s.Require().NoError(err, "failed to decode allowance response")
	unpacked, err := contracts.ERC20MinterBurnerDecimalsContract.ABI.Unpack("allowance", ethRes.Ret)
	s.Require().NoError(err, "failed to unpack allowance")
	allowance, ok := unpacked[0].(*big.Int)
	s.Require().True(ok, "allowance is not *big.Int")
	s.Require().Equal(s.mintAmount.String(), allowance.String(), "allowance mismatch")
}
