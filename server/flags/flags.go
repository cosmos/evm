package flags

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
)

// CometBFT/cosmos-sdk full-node start flags
const (
	WithCometBFT = "with-cometbft"
	Address      = "address"
	Transport    = "transport"
	TraceStore   = "trace-store"
	CPUProfile   = "cpu-profile"

	// AppDBBackend is the type of database for application and snapshots databases
	AppDBBackend = "app-db-backend"
)

// GRPC-related flags.
const (
	GRPCOnly       = "grpc-only"
	GRPCEnable     = "grpc.enable"
	GRPCAddress    = "grpc.address"
	GRPCWebEnable  = "grpc-web.enable"
	GRPCWebAddress = "grpc-web.address"
)

// Cosmos API flags
const (
	RPCEnable         = "api.enable"
	EnabledUnsafeCors = "api.enabled-unsafe-cors"
)

// JSON-RPC flags
const (
	JSONRPCEnable               = "json-rpc.enable"
	JSONRPCAPI                  = "json-rpc.api"
	JSONRPCAddress              = "json-rpc.address"
	JSONWsAddress               = "json-rpc.ws-address"
	JSONRPCWSOrigins            = "json-rpc.ws-origins"
	JSONRPCGasCap               = "json-rpc.gas-cap"
	JSONRPCAllowInsecureUnlock  = "json-rpc.allow-insecure-unlock"
	JSONRPCEVMTimeout           = "json-rpc.evm-timeout"
	JSONRPCTxFeeCap             = "json-rpc.txfee-cap"
	JSONRPCFilterCap            = "json-rpc.filter-cap"
	JSONRPCLogsCap              = "json-rpc.logs-cap"
	JSONRPCBlockRangeCap        = "json-rpc.block-range-cap"
	JSONRPCHTTPTimeout          = "json-rpc.http-timeout"
	JSONRPCHTTPIdleTimeout      = "json-rpc.http-idle-timeout"
	JSONRPCAllowUnprotectedTxs  = "json-rpc.allow-unprotected-txs"
	JSONRPCMaxOpenConnections   = "json-rpc.max-open-connections"
	JSONRPCEnableIndexer        = "json-rpc.enable-indexer"
	JSONRPCBatchRequestLimit    = "json-rpc.batch-request-limit"
	JSONRPCBatchResponseMaxSize = "json-rpc.batch-response-max-size"
	JSONRPCEnableProfiling      = "json-rpc.enable-profiling"
	// JSONRPCEnableMetrics enables EVM RPC metrics server.
	// Set to `metrics` which is hardcoded flag from go-ethereum.
	// https://github.com/ethereum/go-ethereum/blob/master/metrics/metrics.go#L35-L55
	JSONRPCEnableMetrics = "metrics"
)

// EVM flags
const (
	EVMTracer                  = "evm.tracer"
	EVMMaxTxGasWanted          = "evm.max-tx-gas-wanted"
	EVMEnablePreimageRecording = "evm.cache-preimage"
	EVMChainID                 = "evm.evm-chain-id"
	EVMMinTip                  = "evm.min-tip"
	EvmGethMetricsAddress      = "evm.geth-metrics-address"

	EVMMempoolPriceLimit   = "evm.mempool.price-limit"
	EVMMempoolPriceBump    = "evm.mempool.price-bump"
	EVMMempoolAccountSlots = "evm.mempool.account-slots"
	EVMMempoolGlobalSlots  = "evm.mempool.global-slots"
	EVMMempoolAccountQueue = "evm.mempool.account-queue"
	EVMMempoolGlobalQueue  = "evm.mempool.global-queue"
	EVMMempoolLifetime     = "evm.mempool.lifetime"
)

// TLS flags
const (
	TLSCertPath = "tls.certificate-path"
	TLSKeyPath  = "tls.key-path"
)

// AddTxFlags adds common flags for commands to post tx
func AddTxFlags(cmd *cobra.Command) (*cobra.Command, error) {
	cmd.PersistentFlags().String(flags.FlagChainID, "", "Specify Chain ID for sending Tx")
	cmd.PersistentFlags().String(flags.FlagFrom, "", "Name or address of private key with which to sign")
	cmd.PersistentFlags().String(flags.FlagFees, "", "Fees to pay along with transaction; eg: 10aatom")
	cmd.PersistentFlags().String(flags.FlagGasPrices, "", "Gas prices to determine the transaction fee (e.g. 10aatom)")
	cmd.PersistentFlags().String(flags.FlagNode, "tcp://localhost:26657", "<host>:<port> to cometbft rpc interface for this chain")                                                                                                     //nolint:lll
	cmd.PersistentFlags().Float64(flags.FlagGasAdjustment, flags.DefaultGasAdjustment, "adjustment factor to be multiplied against the estimate returned by the tx simulation; if the gas limit is set manually this flag is ignored ") //nolint:lll
	cmd.PersistentFlags().StringP(flags.FlagBroadcastMode, "b", flags.BroadcastSync, "Transaction broadcasting mode (sync|async)")
	cmd.PersistentFlags().String(flags.FlagKeyringBackend, keyring.BackendOS, "Select keyring's backend")

	// --gas can accept integers and "simulate"
	// cmd.PersistentFlags().Var(&flags.GasFlagVar, "gas", fmt.Sprintf(
	//	"gas limit to set per-transaction; set to %q to calculate required gas automatically (default %d)",
	//	flags.GasFlagAuto, flags.DefaultGasLimit,
	// ))

	// viper.BindPFlag(flags.FlagTrustNode, cmd.Flags().Lookup(flags.FlagTrustNode))
	if err := viper.BindPFlag(flags.FlagNode, cmd.PersistentFlags().Lookup(flags.FlagNode)); err != nil {
		return nil, err
	}
	if err := viper.BindPFlag(flags.FlagKeyringBackend, cmd.PersistentFlags().Lookup(flags.FlagKeyringBackend)); err != nil {
		return nil, err
	}
	return cmd, nil
}
