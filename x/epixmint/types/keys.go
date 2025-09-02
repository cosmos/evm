package types

const (
	// ModuleName name that will be used throughout the module
	ModuleName = "epixmint"

	StoreKey = ModuleName

	// RouterKey Top level router key
	RouterKey = ModuleName
)

// key prefixes for store
var (
	ParamsKey = []byte{0x01} // module parameters
)
