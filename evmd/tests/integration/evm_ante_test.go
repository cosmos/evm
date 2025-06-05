package integration

import (
	"testing"

	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/tests/integration/ante"
	testconstants "github.com/cosmos/evm/testutil/constants"
)

func TestEvmAnteTestSuite(t *testing.T) {
	s := ante.NewEvmAnteTestSuite(CreateEvmd)
	s.WithLondonHardForkEnabled(true)

	suite.Run(t, s)

	s = ante.NewEvmAnteTestSuite(CreateEvmd)
	s.WithLondonHardForkEnabled(true)
	// Re-run the tests with EIP-712 Legacy encodings to ensure backwards compatibility.
	// LegacyEIP712Extension should not be run with current TypedData encodings, since they are not compatible.
	s.UseLegacyEIP712TypedData = true
	suite.Run(t, s)
}

func TestEvmUnitAnteTestSuite(t *testing.T) {
	txTypes := []int{gethtypes.DynamicFeeTxType, gethtypes.LegacyTxType, gethtypes.AccessListTxType}
	chainIDs := []testconstants.ChainID{testconstants.ExampleChainID, testconstants.SixDecimalsChainID}

	evmTestSuite := ante.NewEvmUnitAnteTestSuite(CreateEvmd)

	for _, txType := range txTypes {
		for _, chainID := range chainIDs {
			evmTestSuite.EthTxType = txType
			evmTestSuite.ChainID = chainID.ChainID
			evmTestSuite.EvmChainID = chainID.EVMChainID
			suite.Run(t, evmTestSuite)
		}
	}
}

func TestValidateHandlerOptionsTest(t *testing.T) {
	ante.RunValidateHandlerOptionsTest(t, CreateEvmd)
}
