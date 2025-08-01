package types

// Parameter store key
var (
	ParamStoreKeyEnableErc20                = []byte("EnableErc20") // figure out where this is initialized
	ParamStoreKeyPermissionlessRegistration = []byte("PermissionlessRegistration")
)

var (
	CtxKeyDynamicPrecompiles = "DynamicPrecompiles"
	CtxKeyNativePrecompiles  = "NativePrecompiles"
)

func NewParams(
	enableErc20 bool,
	permissionlessRegistration bool,
) Params {
	return Params{
		EnableErc20:                enableErc20,
		PermissionlessRegistration: permissionlessRegistration,
	}
}

func DefaultParams() Params {
	return Params{
		EnableErc20:                true,
		PermissionlessRegistration: true,
	}
}
