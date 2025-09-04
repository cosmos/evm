package suite

const (
	NodeArgsChainID                    = "--chain-id=local-4221"
	NodeArgsApiEnable                  = "--api.enable=true"
	NodeArgsJsonrpcApi                 = "--json-rpc.api=eth,txpool,personal,net,debug,web3"
	NodeArgsJsonrpcAllowUnprotectedTxs = "--json-rpc.allow-unprotected-txs=true"
)

func DefaultNodeArgs() []string {
	return []string{
		NodeArgsJsonrpcApi,
		NodeArgsChainID,
		NodeArgsApiEnable,
		NodeArgsJsonrpcAllowUnprotectedTxs,
	}
}
