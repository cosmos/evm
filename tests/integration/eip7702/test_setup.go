package eip7702

import (
	"github.com/stretchr/testify/suite"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
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
	chainID := evmtypes.GetChainConfig().GetChainId()
	user0 := s.keyring.GetKey(0)
	acc0, err := s.grpcHandler.GetEvmAccount(user0.Addr)
	Expect(err).To(BeNil())

	// Make authorization (user0 -> smart wallet)
	authorization := s.createSetCodeAuthorization(chainID, acc0.GetNonce()+1, s.smartWalletAddr)
	signedAuthorization, err := s.signSetCodeAuthorization(user0, authorization)
	Expect(err).To(BeNil())

	// SetCode tx
	_, err = s.sendSetCodeTx(user0, signedAuthorization)
	Expect(err).To(BeNil(), "error while calling set code tx")
	Expect(s.network.NextBlock()).To(BeNil())
	s.checkSetCode(user0, s.smartWalletAddr, true)

	// Initialize EntryPoint
	_, _, err = s.initSmartWallet(user0, s.entryPointAddr)
	Expect(err).To(BeNil(), "error while initializing smart wallet")
	Expect(s.network.NextBlock()).To(BeNil())
	s.checkInitEntrypoint(user0, s.entryPointAddr)
}
