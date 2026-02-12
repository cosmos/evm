package ics20

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	evmaddress "github.com/cosmos/evm/encoding/address"
	"github.com/cosmos/evm/precompiles/ics20"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
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

	evmAppA := s.chainA.App.(*evmd.EVMD)
	poaAdapterA := evmd.NewPOAStakingAdapter(
		evmAppA.POAKeeper,
		evmtypes.GetEVMCoinDenom(),
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
	)
	s.chainAPrecompile = ics20.NewPrecompile(
		evmAppA.BankKeeper,
		poaAdapterA,
		evmAppA.TransferKeeper,
		evmAppA.IBCKeeper.ChannelKeeper,
	)
	s.chainABondDenom = evmtypes.GetEVMCoinDenom()
	evmAppB := s.chainB.App.(*evmd.EVMD)
	poaAdapterB := evmd.NewPOAStakingAdapter(
		evmAppB.POAKeeper,
		evmtypes.GetEVMCoinDenom(),
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ConsensusAddrPrefix()),
		evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32ValidatorAddrPrefix()),
	)
	s.chainBPrecompile = ics20.NewPrecompile(
		evmAppB.BankKeeper,
		poaAdapterB,
		evmAppB.TransferKeeper,
		evmAppB.IBCKeeper.ChannelKeeper,
	)
	s.chainBBondDenom = evmtypes.GetEVMCoinDenom()
}
