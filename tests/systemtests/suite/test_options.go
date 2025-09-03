package suite

import systemtests "github.com/cosmos/evm/tests/systemtests/types"

func (s *SystemTestSuite) DefaultTestOption() []systemtests.TestOption {
	return []systemtests.TestOption{
		{
			TxType:       "LegacyTx",
			TransferFunc: s.TransferLegacyTx,
		},
		{
			TxType:       "DynamicFeeTx",
			TransferFunc: s.TransferDynamicFeeTx,
		},
	}
}
