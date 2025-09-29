package eip7702

import (
	"fmt"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

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

	abcitypes "github.com/cometbft/cometbft/abci/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	DefaultGasLimit    = uint64(1_000_000)
	InitialTestBalance = 1000000000000000000 // 1 atom
)

var (
	// callArgs is the default arguments for calling the smart contract.
	//
	// NOTE: this has to be populated in a BeforeEach block because the contractAddr would otherwise be a nil address.
	callArgs testutiltypes.CallArgs
	// txArgs are the EVM transaction arguments to use in the transactions
	txArgs evmtypes.EvmTxArgs

	logCheck testutil.LogCheckArgs
)

type IntegrationTestSuite struct {
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

func NewIntegrationTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *IntegrationTestSuite {
	return &IntegrationTestSuite{
		create:  create,
		options: options,
	}
}

func (s *IntegrationTestSuite) SetupTest() {
	s.setupTestSuite()
	s.loadContracts()
	s.deployContracts()
}

func (s *IntegrationTestSuite) SetupSmartWallet() {
	user0 := s.keyring.GetKey(0)
	acc0, err := s.grpcHandler.GetEvmAccount(user0.Addr)
	Expect(err).To(BeNil())

	// Make authorization (user0 -> smart wallet)
	authorization := s.createSetCodeAuthorization(acc0.GetNonce() + 1)
	signedAuthorization, err := s.signSetCodeAuthorization(user0, authorization)
	Expect(err).To(BeNil())

	// SetCode tx
	_, err = s.sendSetCodeTx(user0, signedAuthorization)
	Expect(err).To(BeNil(), "error while calling set code tx")
	Expect(s.network.NextBlock()).To(BeNil())
	s.checkSetCode(user0, true)

	// Initialize EntryPoint
	_, _, err = s.initSmartWallet(user0, s.entryPointAddr)
	Expect(err).To(BeNil(), "error while initializing smart wallet")
	Expect(s.network.NextBlock()).To(BeNil())
	s.checkInitEntrypoint(user0, s.entryPointAddr)
}

func (s *IntegrationTestSuite) setupTestSuite() {
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

func (s *IntegrationTestSuite) loadContracts() {
	erc20Contract, err := contracts.LoadSimpleERC20()
	Expect(err).To(BeNil(), "failed to load SimpleERC20 contract")
	s.erc20Contract = erc20Contract

	entryPointContract, err := contracts.LoadSimpleEntryPoint()
	Expect(err).To(BeNil(), "failed to load SimpleEntryPoint contract")
	s.entryPointContract = entryPointContract

	smartWalletContract, err := contracts.LoadSimpleSmartWallet()
	Expect(err).To(BeNil(), "failed to load SimpleSmartWallet contract")
	s.smartWalletContract = smartWalletContract

	logCheck = logCheck.WithABIEvents(
		s.erc20Contract.ABI.Events,
		s.entryPointContract.ABI.Events,
		s.smartWalletContract.ABI.Events,
	).WithExpPass(true)
}

func (s *IntegrationTestSuite) deployContracts() {
	user0 := s.keyring.GetKey(0)

	// Deploy an ERC20 token
	erc20Addr, err := s.factory.DeployContract(
		user0.Priv,
		evmtypes.EvmTxArgs{
			GasLimit: DefaultGasLimit,
		},
		testutiltypes.ContractDeploymentData{
			Contract: s.erc20Contract,
		},
	)
	Expect(err).To(BeNil(), "failed to deploy erc20 contract")
	Expect(s.network.NextBlock()).To(BeNil())
	s.erc20Addr = erc20Addr

	// Deploy an entry point contract
	entryPointAddr, err := s.factory.DeployContract(
		user0.Priv,
		evmtypes.EvmTxArgs{
			GasLimit: DefaultGasLimit,
		},
		testutiltypes.ContractDeploymentData{
			Contract: s.entryPointContract,
		},
	)
	Expect(err).To(BeNil(), "failed to deploy erc20 contract")
	Expect(s.network.NextBlock()).To(BeNil())
	s.entryPointAddr = entryPointAddr

	// Deploy a smart wallet contract
	smartWalletAddr, err := s.factory.DeployContract(
		user0.Priv,
		evmtypes.EvmTxArgs{
			GasLimit: DefaultGasLimit,
		},
		testutiltypes.ContractDeploymentData{
			Contract: s.smartWalletContract,
		},
	)
	Expect(err).To(BeNil(), "failed to deploy erc20 contract")
	Expect(s.network.NextBlock()).To(BeNil())
	s.smartWalletAddr = smartWalletAddr
}

func (s *IntegrationTestSuite) createSetCodeAuthorization(nonce uint64) ethtypes.SetCodeAuthorization {
	return ethtypes.SetCodeAuthorization{
		ChainID: *uint256.NewInt(evmtypes.GetChainConfig().GetChainId()),
		Address: s.smartWalletAddr,
		Nonce:   nonce,
	}
}

func (s *IntegrationTestSuite) signSetCodeAuthorization(key testkeyring.Key, authorization ethtypes.SetCodeAuthorization) (ethtypes.SetCodeAuthorization, error) {
	// Make authorization (user0 -> smart wallet)
	ecdsaPrivKey, err := key.Priv.(*ethsecp256k1.PrivKey).ToECDSA()
	if err != nil {
		return ethtypes.SetCodeAuthorization{}, fmt.Errorf("failed to get ecdsa private key: %w", err)
	}

	authorization, err = ethtypes.SignSetCode(ecdsaPrivKey, authorization)
	if err != nil {
		return ethtypes.SetCodeAuthorization{}, fmt.Errorf("failed to sign set code authorization: %w", err)
	}

	return authorization, nil
}

func (s *IntegrationTestSuite) sendSetCodeTx(key testkeyring.Key, signedAuthorization ethtypes.SetCodeAuthorization) (abcitypes.ExecTxResult, error) {
	// SetCode tx
	txArgs = evmtypes.EvmTxArgs{
		To:       &common.Address{},
		GasLimit: DefaultGasLimit,
		AuthorizationList: []ethtypes.SetCodeAuthorization{
			signedAuthorization,
		},
	}
	res, err := s.factory.ExecuteEthTx(key.Priv, txArgs)
	if err != nil {
		return abcitypes.ExecTxResult{}, fmt.Errorf("failed to execute eth tx: %w", err)
	}

	return res, nil
}

func (s *IntegrationTestSuite) checkSetCode(key testkeyring.Key, isPass bool) {
	codeHash := s.network.App.GetEVMKeeper().GetCodeHash(s.network.GetContext(), key.Addr)
	code := s.network.App.GetEVMKeeper().GetCode(s.network.GetContext(), codeHash)
	addr, ok := ethtypes.ParseDelegation(code)
	if isPass {
		Expect(ok).To(Equal(true))
		Expect(addr).To(Equal(s.smartWalletAddr))
	} else {
		Expect(ok).To(Equal(false))
	}
}

func (s *IntegrationTestSuite) initSmartWallet(key testkeyring.Key, entryPointAddr common.Address) (abcitypes.ExecTxResult, *evmtypes.MsgEthereumTxResponse, error) {
	// Initialize smart wallet
	txArgs = evmtypes.EvmTxArgs{
		To:       &key.Addr,
		GasLimit: DefaultGasLimit,
	}
	callArgs = testutiltypes.CallArgs{
		ContractABI: s.smartWalletContract.ABI,
		MethodName:  "initialize",
		Args:        []interface{}{key.Addr, entryPointAddr},
	}
	res, ethRes, err := s.factory.CallContractAndCheckLogs(key.Priv, txArgs, callArgs, logCheck)
	if err != nil {
		return abcitypes.ExecTxResult{}, nil, fmt.Errorf("error while initializing smart wallet: %w", err)
	}
	return res, ethRes, nil
}

func (s *IntegrationTestSuite) checkInitEntrypoint(key testkeyring.Key, entryPointAddr common.Address) {
	// Get smart wallet owner
	txArgs = evmtypes.EvmTxArgs{
		To: &key.Addr,
	}
	callArgs = testutiltypes.CallArgs{
		ContractABI: s.smartWalletContract.ABI,
		MethodName:  "owner",
	}
	ethRes, err := s.factory.QueryContract(txArgs, callArgs, DefaultGasLimit)
	Expect(err).To(BeNil(), "error while querying owner of smart wallet")
	Expect(ethRes.Ret).NotTo(BeNil())

	// Check smart wallet owner
	var owner common.Address
	err = s.smartWalletContract.ABI.UnpackIntoInterface(&owner, "owner", ethRes.Ret)
	Expect(err).To(BeNil(), "error while unpacking returned data")
	Expect(owner).To(Equal(key.Addr))

	// Get entry point
	txArgs = evmtypes.EvmTxArgs{
		To: &key.Addr,
	}
	callArgs = testutiltypes.CallArgs{
		ContractABI: s.smartWalletContract.ABI,
		MethodName:  "entryPoint",
	}
	ethRes, err = s.factory.QueryContract(txArgs, callArgs, DefaultGasLimit)
	Expect(err).To(BeNil(), "error while querying owner of smart wallet")
	Expect(ethRes.Ret).NotTo(BeNil())

	// Check entry point
	var entryPoint common.Address
	err = s.smartWalletContract.ABI.UnpackIntoInterface(&entryPoint, "entryPoint", ethRes.Ret)
	Expect(err).To(BeNil(), "error while unpacking returned data")
	Expect(entryPoint).To(Equal(entryPointAddr))
}
