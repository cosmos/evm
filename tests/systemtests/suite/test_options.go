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
		{
			TestType:          "EVM LegacyTx send to multi node",
			TxType:            TxTypeEVM,
			ApplyDynamicFeeTx: false,
			NodeEntries:       []string{"node0", "node1", "node2", "node3", "node0", "node1", "node2", "node3"},
		},
		{
			TestType:          "EVM DynamicFeeTx to multi node",
			TxType:            TxTypeEVM,
			ApplyDynamicFeeTx: true,
			NodeEntries:       []string{"node0", "node1", "node2", "node3", "node0", "node1", "node2", "node3"},
		},
		{
			TestType:          "Cosmos LegacyTx to multi node",
			TxType:            TxTypeCosmos,
			ApplyDynamicFeeTx: false,
			NodeEntries:       []string{"node0", "node1", "node2", "node3", "node0", "node1", "node2", "node3"},
		},
	}
}
