package gov

import (
	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (p Precompile) translateGovError(ctx sdk.Context, method string, err error) error {
	translation := cmn.TranslateCosmosError(p.ABI, cosmosErrorRegistry, err)
	if translation.IsUnmapped {
		p.Logger(ctx).Warn(
			"unmapped registered Cosmos error",
			"precompile", p.Name(),
			"method", method,
			"codespace", translation.Key.Codespace,
			"code", translation.Key.Code,
		)
	}
	return translation.Revert
}

func (p Precompile) govMsgError(ctx sdk.Context, method string, err error) error {
	if _, ok := cmn.ExtractCosmosErrorKey(err); ok {
		return p.translateGovError(ctx, method, err)
	}
	return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, method, err.Error())
}

func (p Precompile) govQueryError(ctx sdk.Context, method string, err error) error {
	if _, ok := cmn.ExtractCosmosErrorKey(err); ok {
		return p.translateGovError(ctx, method, err)
	}
	return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrQueryFailed, method, err.Error())
}
