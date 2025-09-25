package eip7702

import (
	//nolint:revive // dot imports are fine for Ginkgo
	"math/big"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/tests/contracts"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const InitialTestBalance = 1000000000000000000 // 1 atom

var (
	// callArgs is the default arguments for calling the smart contract.
	//
	// NOTE: this has to be populated in a BeforeEach block because the contractAddr would otherwise be a nil address.
	callArgs testutiltypes.CallArgs
	// txArgs are the EVM transaction arguments to use in the transactions
	txArgs evmtypes.EvmTxArgs
	// defaultLogCheck instantiates a log check arguments struct with the precompile ABI events populated.
	defaultLogCheck testutil.LogCheckArgs
	// passCheck defines the arguments to check if the precompile returns no error
	passCheck testutil.LogCheckArgs
	// outOfGasCheck defines the arguments to check if the precompile returns out of gas error
	outOfGasCheck testutil.LogCheckArgs
)

type EIP7702IntegrationTestSuite struct {
	suite.Suite

	create      network.CreateEvmApp
	options     []network.ConfigOption
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     testkeyring.Keyring

	customGenesis bool

	erc20Contract       evmtypes.CompiledContract
	erc20Addr           common.Address
	entryPointContract  evmtypes.CompiledContract
	entryPointAddr      common.Address
	smartWalletContract evmtypes.CompiledContract
	smartWalletAddr     common.Address
}

func NewEIP7702IntegrationTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *EIP7702IntegrationTestSuite {
	return &EIP7702IntegrationTestSuite{
		create:  create,
		options: options,
	}
}

func (s *EIP7702IntegrationTestSuite) SetupTest() {
	s.setupTestSuite()
	s.loadContracts()
	s.deployContracts()
	s.setupContracts()
}

func (s *EIP7702IntegrationTestSuite) setupTestSuite() {
	keyring := testkeyring.New(2)
	customGenesis := network.CustomGenesisState{}
	// mint some coin to fee collector
	coins := sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(InitialTestBalance)))
	balances := []banktypes.Balance{
		{
			Address: authtypes.NewModuleAddress(authtypes.FeeCollectorName).String(),
			Coins:   coins,
		},
	}
	bankGenesis := banktypes.DefaultGenesisState()
	bankGenesis.Balances = balances
	customGenesis[banktypes.ModuleName] = bankGenesis
	opts := []network.ConfigOption{
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	}
	if s.customGenesis {
		opts = append(opts, network.WithCustomGenesis(customGenesis))
	}
	opts = append(opts, s.options...)
	nw := network.NewUnitTestNetwork(s.create, opts...)
	grpcHandler := grpc.NewIntegrationHandler(nw)
	txFactory := factory.New(nw, grpcHandler)

	s.factory = txFactory
	s.grpcHandler = grpcHandler
	s.keyring = keyring
	s.network = nw
}

func (s *EIP7702IntegrationTestSuite) loadContracts() {
	erc20Contract, err := contracts.LoadSimpleERC20()
	s.Require().NoError(err, "failed to load SimpleERC20 contract")
	s.erc20Contract = erc20Contract

	entryPointContract, err := contracts.LoadSimpleEntryPoint()
	s.Require().NoError(err, "failed to load SimpleEntryPoint contract")
	s.entryPointContract = entryPointContract

	smartWalletContract, err := contracts.LoadSimpleSmartWallet()
	s.Require().NoError(err, "failed to load SimpleSmartWallet contract")
	s.smartWalletContract = smartWalletContract
}

func (s *EIP7702IntegrationTestSuite) deployContracts() {
	user0 := s.keyring.GetKey(0)

	// Deploy an ERC20 token
	erc20Addr, err := s.factory.DeployContract(
		user0.Priv,
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract: s.erc20Contract,
		},
	)
	s.Require().NoError(err, "failed to deploy erc20 contract")
	s.erc20Addr = erc20Addr

	// Deploy an entry point contract
	entryPointAddr, err := s.factory.DeployContract(
		user0.Priv,
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract: s.entryPointContract,
		},
	)
	s.Require().NoError(err, "failed to deploy erc20 contract")
	s.entryPointAddr = entryPointAddr

	// Deploy a smart wallet contract
	smartWalletAddr, err := s.factory.DeployContract(
		user0.Priv,
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract: s.smartWalletContract,
		},
	)
	s.Require().NoError(err, "failed to deploy erc20 contract")
	s.smartWalletAddr = smartWalletAddr
}

func (s *EIP7702IntegrationTestSuite) setupContracts() {
	var err error
	user0 := s.keyring.GetKey(0)

	// Mint erc20 tokens to user0
	txArgs = evmtypes.EvmTxArgs{
		To: &s.erc20Addr,
	}
	callArgs = testutiltypes.CallArgs{
		ContractABI: s.erc20Contract.ABI,
		MethodName:  "mint",
		Args: []interface{}{
			user0.Addr,
			big.NewInt(1e18),
		},
	}
	_, _, err = s.factory.CallContractAndCheckLogs(user0.Priv, txArgs, callArgs, passCheck)
	s.Require().NoError(err, "error while calling erc20 contract")

	// Make authorization (user0 -> smart wallet)
	ecdsaPrivKey, err := user0.Priv.(*ethsecp256k1.PrivKey).ToECDSA()
	s.Require().NoError(err, "failed to get ecdsa private key")

	authorization, err := ethtypes.SignSetCode(ecdsaPrivKey, ethtypes.SetCodeAuthorization{
		ChainID: *uint256.NewInt(evmtypes.GetChainConfig().ChainId),
		Address: s.smartWalletAddr,
		Nonce:   s.network.App.GetEVMKeeper().GetNonce(s.network.GetContext(), user0.Addr),
	})
	s.Require().NoError(err, "failed to sign set code authorization")

	// SetCode tx
	txArgs = evmtypes.EvmTxArgs{
		To: &common.Address{},
		AuthorizationList: []ethtypes.SetCodeAuthorization{
			authorization,
		},
	}
	_, err = s.factory.ExecuteEthTx(user0.Priv, txArgs)
	s.Require().NoError(err, "error while calling set code tx")

	// Check set code
	codeHash := s.network.App.GetEVMKeeper().GetCodeHash(s.network.GetContext(), user0.Addr)
	code := s.network.App.GetEVMKeeper().GetCode(s.network.GetContext(), codeHash)
	addr, ok := ethtypes.ParseDelegation(code)
	s.Require().True(ok)
	s.Require().Equal(s.smartWalletAddr, addr)

	// Initialize smart wallet
	txArgs = evmtypes.EvmTxArgs{
		To: &s.smartWalletAddr,
	}
	callArgs = testutiltypes.CallArgs{
		ContractABI: s.smartWalletContract.ABI,
		MethodName:  "initialize",
		Args: []interface{}{
			user0.Addr,
			s.entryPointAddr,
		},
	}
	_, _, err = s.factory.CallContractAndCheckLogs(user0.Priv, txArgs, callArgs, passCheck)
	s.Require().NoError(err, "error while initializing smart wallet")

	// Get smart wallet owner
	txArgs = evmtypes.EvmTxArgs{
		To: &s.smartWalletAddr,
	}
	callArgs = testutiltypes.CallArgs{
		ContractABI: s.smartWalletContract.ABI,
		MethodName:  "owner",
		Args:        []interface{}{},
	}
	_, ethRes, err := s.factory.CallContractAndCheckLogs(user0.Priv, txArgs, callArgs, passCheck)
	s.Require().NoError(err, "error while querying owner of smart wallet")
	s.Require().NotEmpty(ethRes.Ret)

	// Check smart wallet owner
	var owner common.Address
	err = s.smartWalletContract.ABI.UnpackIntoInterface(&owner, "owner", ethRes.Ret)
	s.Require().NoError(err, "error while unpacking returned data")
	s.Require().Equal(user0.Addr, owner)

	// Get entry point
	txArgs = evmtypes.EvmTxArgs{
		To: &s.smartWalletAddr,
	}
	callArgs = testutiltypes.CallArgs{
		ContractABI: s.smartWalletContract.ABI,
		MethodName:  "entryPoint",
		Args:        []interface{}{},
	}
	_, ethRes, err = s.factory.CallContractAndCheckLogs(user0.Priv, txArgs, callArgs, passCheck)
	s.Require().NoError(err, "error while querying owner of smart wallet")
	s.Require().NotEmpty(ethRes.Ret)

	// Check entry point
	var entryPoint common.Address
	err = s.smartWalletContract.ABI.UnpackIntoInterface(&owner, "entryPoint", ethRes.Ret)
	s.Require().NoError(err, "error while unpacking returned data")
	s.Require().Equal(s.entryPointAddr, entryPoint)
}
