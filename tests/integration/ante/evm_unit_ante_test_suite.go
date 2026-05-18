package ante

import (
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/network"
)

// EvmUniAnteTestSuite aims to test all EVM ante handler unit functions.
// NOTE: the suite only holds properties related to global execution parameters
// (what type of tx to run the tests with) not independent tests values.
type EvmUnitAnteTestSuite struct {
	suite.Suite

	create network.CreateEvmApp

	// To make sure that every tests is run with all the tx types
	EthTxType  int
	ChainID    string
	EvmChainID uint64
}

func NewEvmUnitAnteTestSuite(
	create network.CreateEvmApp,
) *EvmUnitAnteTestSuite {
	return &EvmUnitAnteTestSuite{
		create:     create,
		ChainID:    constants.ExampleChainID.ChainID,
		EvmChainID: constants.ExampleChainID.EVMChainID,
	}
}
