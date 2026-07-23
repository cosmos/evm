package ics02

import (
	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (p Precompile) translateICS02Error(ctx sdk.Context, method string, err error) cmn.ErrorTranslation {
	translation := cmn.TranslateCosmosError(p.ABI, cosmosErrorRegistry, err)
	p.logUnmappedICS02Error(ctx, method, translation)
	return translation
}

func (p Precompile) logUnmappedICS02Error(ctx sdk.Context, method string, translation cmn.ErrorTranslation) {
	if !translation.IsUnmapped {
		return
	}
	p.Logger(ctx).Warn(
		"unmapped registered Cosmos error",
		"precompile", p.Name(),
		"method", method,
		"codespace", translation.Key.Codespace,
		"code", translation.Key.Code,
	)
}

func (p Precompile) ics02KeeperError(ctx sdk.Context, method string, err error) error {
	translation := p.translateICS02Error(ctx, method, err)
	if translation.Kind != cmn.MappingKindInternal {
		return translation.Revert
	}
	return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, method, err.Error())
}

func (p Precompile) ics02ValidatedInputError(ctx sdk.Context, err error) error {
	translation := p.translateICS02Error(ctx, UpdateClientMethod, err)
	if translation.Kind != cmn.MappingKindInternal {
		return translation.Revert
	}
	return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, UpdateClientMethod, err.Error())
}

func (p Precompile) ics02QueryError(ctx sdk.Context, method string, err error) error {
	translation := p.translateICS02Error(ctx, method, err)
	if translation.Kind != cmn.MappingKindInternal {
		return translation.Revert
	}
	return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrQueryFailed, method, err.Error())
}
