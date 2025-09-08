package suite

func (s *SystemTestSuite) DefaultTestOption() []TestOption {
	return []TestOption{
		{
			TestType:          "EVM LegacyTx send",
			ApplyDynamicFeeTx: false,
			TxType:            TxTypeEVM,
		},
		{
			TestType:          "EVM DynamicFeeTx send",
			TxType:            TxTypeEVM,
			ApplyDynamicFeeTx: true,
		},
		{
			TestType:          "Cosmos LegacyTx send",
			TxType:            TxTypeCosmos,
			ApplyDynamicFeeTx: false,
		},
	}
}
