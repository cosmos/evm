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

type TxInfo struct {
	DstNodeID string
	TxType    string
	TxHash    string
}

func NewTxInfo(nodeID, txHash, txType string) *TxInfo {
	return &TxInfo{
		DstNodeID: nodeID,
		TxHash:    txHash,
		TxType:    txType,
	}
}
