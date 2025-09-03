package suite

import systemtests "github.com/cosmos/evm/tests/systemtests/types"

func (s *SystemTestSuite) DefaultTestOption() []systemtests.TestOption {
	return []systemtests.TestOption{
		{
			TestType:      "EVM LegacyTx",
			Transfer:      s.SendEthTx,
			WaitForCommit: s.WaitForEthCommmit,
		},
		{
			TestType:      "EVM DynamicFeeTx",
			Transfer:      s.SendEthDynamicFeeTx,
			WaitForCommit: s.WaitForEthCommmit,
		},
		{
			TestType:      "Cosmos LegacyTx",
			Transfer:      s.SendCosmosTx,
			WaitForCommit: s.WaitForCosmosCommmit,
		},
	}
}
