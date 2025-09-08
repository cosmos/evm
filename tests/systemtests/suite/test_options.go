package suite

func (s *SystemTestSuite) DefaultTestOption() []TestOption {
	return []TestOption{
		{
			TestType:          "EVM LegacyTx send to single node",
			ApplyDynamicFeeTx: false,
			TxType:            TxTypeEVM,
		},
		{
			TestType:          "EVM DynamicFeeTx send to single node",
			TxType:            TxTypeEVM,
			ApplyDynamicFeeTx: true,
		},
		{
			TestType:          "Cosmos LegacyTx send to single node",
			TxType:            TxTypeCosmos,
			ApplyDynamicFeeTx: false,
		},
	}
}
