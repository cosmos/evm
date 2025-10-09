package ics20

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm"
	"github.com/cosmos/evm/precompiles/ics20"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"
)

type PrecompileTestSuite struct {
	suite.Suite
	internalT   *testing.T
	coordinator *evmibctesting.Coordinator

	create           ibctesting.AppCreator
	chainA           *evmibctesting.TestChain
	chainAPrecompile *ics20.Precompile
	chainABondDenom  string
	chainB           *evmibctesting.TestChain
	chainBPrecompile *ics20.Precompile
	chainBBondDenom  string
}

//nolint:thelper // NewPrecompileTestSuite is not a helper function; it's an instantiation function for the test suite.
func NewPrecompileTestSuite(t *testing.T, create ibctesting.AppCreator) *PrecompileTestSuite {
	return &PrecompileTestSuite{
		internalT: t,
		create:    create,
	}
}

func (s *PrecompileTestSuite) SetupTest() {
	// Setup IBC
	if s.internalT == nil {
		s.internalT = s.T()
	}
	s.coordinator = evmibctesting.NewCoordinator(s.internalT, 2, 0, s.create)
	s.chainA = s.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	s.chainB = s.coordinator.GetChain(evmibctesting.GetEvmChainID(2))

	evmAppA := s.chainA.App.(evm.EvmApp)
	s.chainAPrecompile = ics20.NewPrecompile(
		evmAppA.GetBankKeeper(),
		*evmAppA.GetStakingKeeper(),
		evmAppA.GetTransferKeeper(),
		evmAppA.GetIBCKeeper().ChannelKeeper,
	)
	s.chainABondDenom, _ = evmAppA.GetStakingKeeper().BondDenom(s.chainA.GetContext())
	evmAppB := s.chainB.App.(evm.EvmApp)
	s.chainBPrecompile = ics20.NewPrecompile(
		evmAppB.GetBankKeeper(),
		*evmAppB.GetStakingKeeper(),
		evmAppB.GetTransferKeeper(),
		evmAppB.GetIBCKeeper().ChannelKeeper,
	)
	s.chainBBondDenom, _ = evmAppB.GetStakingKeeper().BondDenom(s.chainB.GetContext())
}
