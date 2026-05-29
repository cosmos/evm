package eip7702

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

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

const (
	DefaultGasLimit    = uint64(1_000_000)
	InitialTestBalance = 1000000000000000000 // 1 atom
)

var logCheck testutil.LogCheckArgs

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

	walletConfigured map[common.Address]bool
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
	s.fundERC20Tokens()
}

func (s *IntegrationTestSuite) SetupSmartWallet() {
	keys := s.keyring.GetKeys()
	for _, key := range keys {
		s.setupSmartWalletForKey(key)
	}
}

func (s *IntegrationTestSuite) setupSmartWalletForKey(key testkeyring.Key) {
	if s.walletConfigured[key.Addr] {
		return
	}
	chainID := evmtypes.GetChainConfig().GetChainId()
	acc, err := s.grpcHandler.GetEvmAccount(key.Addr)
	Expect(err).To(BeNil())

	authorization := s.createSetCodeAuthorization(chainID, acc.GetNonce()+1, s.smartWalletAddr)
	signedAuthorization, err := s.signSetCodeAuthorization(key, authorization)
	Expect(err).To(BeNil())

	err = s.sendSetCodeTx(key, signedAuthorization)
	Expect(err).To(BeNil(), "error while calling set code tx")
	Expect(s.network.NextBlock()).To(BeNil())
	s.checkSetCode(key, s.smartWalletAddr, true)

	_, _, err = s.initSmartWallet(key, s.entryPointAddr)
	Expect(err).To(BeNil(), "error while initializing smart wallet")
	Expect(s.network.NextBlock()).To(BeNil())
	s.checkInitEntrypoint(key, s.entryPointAddr)

	s.walletConfigured[key.Addr] = true
}

func (s *IntegrationTestSuite) setupTestSuite() {
	keyring := testkeyring.New(3)
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

	s.walletConfigured = make(map[common.Address]bool)
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

func (s *IntegrationTestSuite) fundERC20Tokens() {
	user0 := s.keyring.GetKey(0)
	user1 := s.keyring.GetKey(1)
	user2 := s.keyring.GetKey(2)
	amount := new(big.Int)
	amount.SetString("1000000000000000000000", 10) // 10^21
	transfer := func(recipient common.Address) {
		txArgs := evmtypes.EvmTxArgs{
			To:       &s.erc20Addr,
			GasLimit: DefaultGasLimit,
		}
		callArgs := testutiltypes.CallArgs{
			ContractABI: s.erc20Contract.ABI,
			MethodName:  "transfer",
			Args: []interface{}{
				recipient,
				amount,
			},
		}
		_, _, err := s.factory.CallContractAndCheckLogs(
			user0.Priv,
			txArgs,
			callArgs,
			logCheck.WithExpEvents("Transfer"),
		)
		Expect(err).To(BeNil(), "failed to transfer ERC20 tokens")
		Expect(s.network.NextBlock()).To(BeNil())
	}
	transfer(user1.Addr)
	transfer(user2.Addr)
}
