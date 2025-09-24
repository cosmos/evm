package werc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/werc20"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
)

type PrecompileUnitTestSuite struct {
	suite.Suite

	create      network.CreateEvmApp
	options     []network.ConfigOption
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     keyring.Keyring

	// WEVMOS related fields
	precompile        *werc20.Precompile
	precompileAddrHex string
}

func NewPrecompileUnitTestSuite(
	create network.CreateEvmApp,
	options ...network.ConfigOption,
) *PrecompileUnitTestSuite {
	return &PrecompileUnitTestSuite{
		create:  create,
		options: options,
	}
}

// SetupTest allows to configure the testing suite embedding a network with a
// custom chainID. This is important to check that the correct address is used
// for the precompile.
func (s *PrecompileUnitTestSuite) SetupTest(chainID testconstants.ChainID) {
	keyring := keyring.New(2)

	options := []network.ConfigOption{
		network.WithChainID(chainID),
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	}
	options = append(options, s.options...)
	integrationNetwork := network.NewUnitTestNetwork(s.create, options...)
	grpcHandler := grpc.NewIntegrationHandler(integrationNetwork)
	txFactory := factory.New(integrationNetwork, grpcHandler)

	s.network = integrationNetwork
	s.factory = txFactory
	s.grpcHandler = grpcHandler
	s.keyring = keyring

	s.precompileAddrHex = network.GetWEVMOSContractHex(chainID)

	ctx := integrationNetwork.GetContext()

	tokenDenom, err := s.network.App.GetErc20Keeper().GetTokenDenom(ctx, common.HexToAddress(s.precompileAddrHex))
	s.Require().NoError(err, "failed to get token denom")
	tokenPairID := s.network.App.GetErc20Keeper().GetTokenPairID(ctx, tokenDenom)
	tokenPair, found := s.network.App.GetErc20Keeper().GetTokenPair(ctx, tokenPairID)
	s.Require().True(found, "expected wevmos precompile to be registered in the tokens map")
	s.Require().Equal(s.precompileAddrHex, tokenPair.Erc20Address, "expected a different address of the contract")

	precompile := werc20.NewPrecompile(
		tokenPair,
		s.network.App.GetBankKeeper(),
		s.network.App.GetErc20Keeper(),
		s.network.App.GetTransferKeeper(),
	)
	s.Require().NotNil(precompile)
	s.precompile = precompile
}

type DepositEvent struct {
	Dst common.Address
	Wad *big.Int
}

type WithdrawalEvent struct {
	Src common.Address
	Wad *big.Int
}

//nolint:dupl
func (s *PrecompileUnitTestSuite) TestEmitDepositEvent() {
	testCases := []struct {
		name    string
		chainID testconstants.ChainID
	}{
		{
			name:    "mainnet",
			chainID: testconstants.ExampleChainID,
		}, {
			name:    "six decimals",
			chainID: testconstants.SixDecimalsChainID,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest(tc.chainID)
			caller := s.keyring.GetAddr(0)
			amount := new(big.Int).SetInt64(1_000)

			stateDB := s.network.GetStateDB()

			err := s.precompile.EmitDepositEvent(
				s.network.GetContext(),
				stateDB,
				caller,
				amount,
			)
			s.Require().NoError(err, "expected deposit event to be emitted successfully")

			log := stateDB.Logs()[0]

			// Check on the address
			s.Require().Equal(log.Address, s.precompile.Address())

			// Check on the topics
			event := s.precompile.Events[werc20.EventTypeDeposit]
			s.Require().Equal(
				crypto.Keccak256Hash([]byte(event.Sig)),
				common.HexToHash(log.Topics[0].Hex()),
			)
			var adddressTopic common.Hash
			copy(adddressTopic[common.HashLength-common.AddressLength:], caller[:])
			s.Require().Equal(adddressTopic, log.Topics[1])

			s.Require().EqualValues(log.BlockNumber, s.network.GetContext().BlockHeight())

			// Verify data
			var depositEvent DepositEvent
			err = cmn.UnpackLog(s.precompile.ABI, &depositEvent, werc20.EventTypeDeposit, *log)
			s.Require().NoError(err, "unable to unpack log into deposit event")

			s.Require().Equal(caller, depositEvent.Dst, "expected different destination address")
			s.Require().Equal(amount, depositEvent.Wad, "expected different amount")
		})
	}
}

//nolint:dupl
func (s *PrecompileUnitTestSuite) TestEmitWithdrawalEvent() {
	testCases := []struct {
		name    string
		chainID testconstants.ChainID
	}{
		{
			name:    "mainnet",
			chainID: testconstants.ExampleChainID,
		}, {
			name:    "six decimals",
			chainID: testconstants.SixDecimalsChainID,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest(tc.chainID)
			caller := s.keyring.GetAddr(0)
			amount := new(big.Int).SetInt64(1_000)

			stateDB := s.network.GetStateDB()

			err := s.precompile.EmitWithdrawalEvent(
				s.network.GetContext(),
				stateDB,
				caller,
				amount,
			)
			s.Require().NoError(err, "expected withdrawal event to be emitted successfully")

			log := stateDB.Logs()[0]

			// Check on the address
			s.Require().Equal(log.Address, s.precompile.Address())

			// Check on the topics
			event := s.precompile.Events[werc20.EventTypeWithdrawal]
			s.Require().Equal(
				crypto.Keccak256Hash([]byte(event.Sig)),
				common.HexToHash(log.Topics[0].Hex()),
			)
			var adddressTopic common.Hash
			copy(adddressTopic[common.HashLength-common.AddressLength:], caller[:])
			s.Require().Equal(adddressTopic, log.Topics[1])

			s.Require().EqualValues(log.BlockNumber, s.network.GetContext().BlockHeight())

			// Verify data
			var withdrawalEvent WithdrawalEvent
			err = cmn.UnpackLog(s.precompile.ABI, &withdrawalEvent, werc20.EventTypeWithdrawal, *log)
			s.Require().NoError(err, "unable to unpack log into withdrawal event")

			s.Require().Equal(caller, withdrawalEvent.Src, "expected different source address")
			s.Require().Equal(amount, withdrawalEvent.Wad, "expected different amount")
		})
	}
}
