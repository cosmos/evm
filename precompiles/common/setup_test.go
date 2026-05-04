package common_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	debugprecompile "github.com/cosmos/evm/evmd/tests/testdata/debug"
	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	testkeyring "github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"

	storetypes "cosmossdk.io/store/types"
)

type PrecompileTestSuite struct {
	suite.Suite

	network         *network.UnitTestNetwork
	factory         factory.TxFactory
	grpcHandler     grpc.Handler
	keyring         testkeyring.Keyring
	debugPrecompile *debugprecompile.Precompile
	bondDenom       string
}

func TestPrecompileUnitTestSuite(t *testing.T) {
	suite.Run(t, new(PrecompileTestSuite))
}

func (s *PrecompileTestSuite) SetupTest() {
	keyring := testkeyring.New(2)
	nw := network.NewUnitTestNetwork(
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	)
	grpcHandler := grpc.NewIntegrationHandler(nw)
	txFactory := factory.New(nw, grpcHandler)

	ctx := nw.GetContext()

	// Get bond denom
	sk := nw.App.StakingKeeper
	bondDenom, err := sk.BondDenom(ctx)
	if err != nil {
		panic(err)
	}

	// Create and register debug precompile
	debugPrec := debugprecompile.NewPrecompile(nw.App.BankKeeper, nw.App.EVMKeeper)
	nw.App.EVMKeeper.RegisterStaticPrecompile(debugPrec.Address(), debugPrec)
	err = nw.App.EVMKeeper.EnableStaticPrecompiles(ctx, debugPrec.Address())
	if err != nil {
		panic(err)
	}
	// We must directly commit keeper calls to state, otherwise they get
	// fully wiped when the next block finalizes.
	store := nw.GetContext().MultiStore()
	if cms, ok := store.(storetypes.CacheMultiStore); ok {
		cms.Write()
	} else {
		panic("store is not a CacheMultiStore")
	}

	s.network = nw
	s.factory = txFactory
	s.grpcHandler = grpcHandler
	s.keyring = keyring
	s.debugPrecompile = debugPrec
	s.bondDenom = bondDenom
}
