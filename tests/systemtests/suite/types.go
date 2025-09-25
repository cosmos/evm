package suite

const (
	TxTypeEVM    = "EVMTx"
	TxTypeCosmos = "CosmosTx"

	NodeArgsChainID                    = "--chain-id=local-4221"
	NodeArgsApiEnable                  = "--api.enable=true"
	NodeArgsJsonrpcApi                 = "--json-rpc.api=eth,txpool,personal,net,debug,web3"
	NodeArgsJsonrpcAllowUnprotectedTxs = "--json-rpc.allow-unprotected-txs=true"
)

// TestOptions defines the options for a test case.
type TestOptions struct {
	Description    string
	TxType         string
	IsDynamicFeeTx bool
}

// TxInfo holds information about a transaction.
type TxInfo struct {
	DstNodeID string
	TxType    string
	TxHash    string
}

// NewTxInfo creates a new TxInfo instance.
func NewTxInfo(nodeID, txHash, txType string) *TxInfo {
	return &TxInfo{
		DstNodeID: nodeID,
		TxHash:    txHash,
		TxType:    txType,
	}
}

// DefaultNodeArgs returns the default node arguments for starting the chain.
func DefaultNodeArgs() []string {
	return []string{
		NodeArgsJsonrpcApi,
		NodeArgsChainID,
		NodeArgsApiEnable,
		NodeArgsJsonrpcAllowUnprotectedTxs,
	}
}

// MinimumGasPriceZeroArgs returns the node arguments with minimum gas price set to zero.
func MinimumGasPriceZeroArgs() []string {
	return append(DefaultNodeArgs(), "--minimum-gas-prices=0stake")
}
