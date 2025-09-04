package suite

const (
	TxTypeEVM    = "EVMTx"
	TxTypeCosmos = "CosmosTx"
)

type TestOption struct {
	TestType          string
	TxType            string
	ApplyDynamicFeeTx bool
	NodeEntries       []string
}
