package evidence

import (
	"context"
	"fmt"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/precompiles/evidence"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"

	"cosmossdk.io/x/evidence/exported"
	"cosmossdk.io/x/evidence/types"
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

	router := types.NewRouter()
	router = router.AddRoute(types.RouteEquivocation, testEquivocationHandler(nw.App.GetEvidenceKeeper()))
	nw.App.GetEvidenceKeeper().SetRouter(router)

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

func testEquivocationHandler(_ interface{}) types.Handler {
	return func(_ context.Context, e exported.Evidence) error {
		if err := e.ValidateBasic(); err != nil {
			return err
		}

		ee, ok := e.(*types.Equivocation)
		if !ok {
			return fmt.Errorf("unexpected evidence type: %T", e)
		}
		if ee.Height%2 == 0 {
			return fmt.Errorf("unexpected even evidence height: %d", ee.Height)
		}

		return nil
	}
}
