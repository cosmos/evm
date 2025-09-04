package suite

func (s *SystemTestSuite) DefaultTestOption() []TestOption {
	return []TestOption{
		{
			TestType: "EVM LegacyTx",
			TxType:   TxTypeEVM,
			SendTx:   s.SendEthTx,
		},
		{
			TestType: "EVM DynamicFeeTx",
			TxType:   TxTypeEVM,
			SendTx:   s.SendEthDynamicFeeTx,
		},
		{
			TestType: "Cosmos LegacyTx",
			TxType:   TxTypeCosmos,
			SendTx:   s.SendCosmosTx,
		},
	}
}
