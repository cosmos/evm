package types

const (
	// DefaultGasPrice is the default gas price used for setting gas-related calculations
	DefaultGasPrice = 20
	// DefaultChainID is the default chain ID
	DefaultChainID = "cosmos"
	// DefaultEvmChainID defines the default EVM chain ID
	DefaultEvmChainID = 262144
	// DefaultDisplayDenom defines the display denom for the default EVM coin info
	DefaultDisplayDenom = "atom"
	// DefaultExtendedDenom defines the extended denom for the default EVM coin info
	DefaultExtendedDenom = "aatom"
	// DefaultBaseDenom defines the base denom for the default EVM coin info
	DefaultBaseDenom = "aatom"
	// DefaultDecimals defines the decimals for the default EVM coin info
	DefaultDecimals = EighteenDecimals
	// DefaultWevmosContractMainnet is the default WEVMOS contract address for mainnet
	DefaultWevmosContractMainnet = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"
)

var DefaultEvmCoinInfo = EvmCoinInfo{
	DisplayDenom:  DefaultDisplayDenom,
	Decimals:      DefaultDecimals,
	BaseDenom:     DefaultBaseDenom,
	ExtendedDenom: DefaultExtendedDenom,
}
