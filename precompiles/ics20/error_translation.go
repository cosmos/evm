package ics20

import (
	cmn "github.com/cosmos/evm/precompiles/common"
	host "github.com/cosmos/ibc-go/v11/modules/core/24-host"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func isHostInvalidID(err error) bool {
	key, ok := cmn.ExtractCosmosErrorKey(err)
	return ok && key == cmn.NewCosmosErrorKey(host.ErrInvalidID)
}

func invalidSourceChannelError() error {
	return cmn.NewRevertWithSolidityError(ABI, SolidityErrInvalidSourceChannel, TransferMethod, ErrInvalidSourceChannel)
}

func (p Precompile) translateICS20Error(ctx sdk.Context, method string, err error) cmn.ErrorTranslation {
	translation := cmn.TranslateCosmosError(p.ABI, cosmosErrorRegistry, err)
	p.logUnmappedICS20Error(ctx, method, translation)
	return translation
}

func (p Precompile) logUnmappedICS20Error(ctx sdk.Context, method string, translation cmn.ErrorTranslation) {
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

func (p Precompile) ics20MsgError(ctx sdk.Context, err error) error {
	translation := p.translateICS20Error(ctx, TransferMethod, err)
	if translation.Kind != cmn.MappingKindInternal {
		return translation.Revert
	}
	return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, TransferMethod, err.Error())
}

func (p Precompile) ics20ValidatedInputError(ctx sdk.Context, err error) error {
	translation := p.translateICS20Error(ctx, TransferMethod, err)
	if translation.Kind != cmn.MappingKindInternal {
		return translation.Revert
	}
	return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, TransferMethod, err.Error())
}

func (p Precompile) ics20QueryError(ctx sdk.Context, method string, err error) error {
	translation := p.translateICS20Error(ctx, method, err)
	if translation.Kind != cmn.MappingKindInternal {
		return translation.Revert
	}
	return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrQueryFailed, method, err.Error())
}

func ics20QueryPreservesSuccess(method string, err error) bool {
	disposition, ok := cmn.ReviewedGRPCErrorRegistry().Resolve(cmn.ErrorBoundaryQueryServer, method, err)
	return ok && disposition.Kind == cmn.GRPCDispositionPreserveSuccess
}
