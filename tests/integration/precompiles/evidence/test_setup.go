package evidence

import (
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/precompiles/evidence"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
)

type PrecompileTestSuite struct {
	suite.Suite

	create      network.CreateEvmApp
	opts        []network.ConfigOption
	network     *network.UnitTestNetwork
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     testkeyring.Keyring

	precompile *evidence.Precompile
}

func NewPrecompileTestSuite(create network.CreateEvmApp, opts ...network.ConfigOption) *PrecompileTestSuite {
	return &PrecompileTestSuite{
		create: create,
		opts:   opts,
	}
}

func (s *PrecompileTestSuite) SetupTest() {
	keyring := testkeyring.New(2)
	var err error
	s.opts = append(s.opts, network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...))
	nw := network.NewUnitTestNetwork(s.create, s.opts...)

	grpcHandler := grpc.NewIntegrationHandler(nw)
	txFactory := factory.New(nw, grpcHandler)

	s.network = nw
	s.factory = txFactory
	s.grpcHandler = grpcHandler
	s.keyring = keyring

	if s.precompile, err = evidence.NewPrecompile(
		*s.network.App.GetEvidenceKeeper(),
	); err != nil {
		panic(err)
	}
}
