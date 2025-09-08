package suite

const (
	TxTypeEVM    = "EVMTx"
	TxTypeCosmos = "CosmosTx"
)

type TestOptions struct {
	Description    string
	TxType         string
	IsDynamicFeeTx bool
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
