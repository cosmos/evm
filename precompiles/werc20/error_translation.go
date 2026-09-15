package werc20

import (
	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (p Precompile) translateWERC20Error(ctx sdk.Context, method string, err error) error {
	translation := cmn.TranslateCosmosError(p.ABI, cosmosErrorRegistry, err)
	if translation.IsUnmapped {
		ctx.Logger().With("evm extension", "werc20").Warn(
			"unmapped registered Cosmos error",
			"precompile", "werc20",
			"method", method,
			"codespace", translation.Key.Codespace,
			"code", translation.Key.Code,
		)
	}
	return translation.Revert
}

func (p Precompile) werc20MsgError(ctx sdk.Context, method string, err error) error {
	if _, ok := cmn.ExtractCosmosErrorKey(err); ok {
		return p.translateWERC20Error(ctx, method, err)
	}
	return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, method, err.Error())
}
