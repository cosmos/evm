package staking

import (
	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (p Precompile) translateStakingError(ctx sdk.Context, method string, err error) error {
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

func (p Precompile) translateStakingGRPCError(boundary cmn.ErrorBoundary, method string, err error) error {
	translation := cmn.TranslateGRPCError(p.ABI, cmn.ReviewedGRPCErrorRegistry(), boundary, method, err)
	return translation.Revert
}

func (p Precompile) stakingMsgError(ctx sdk.Context, method string, err error) error {
	grpcTranslation := cmn.TranslateGRPCError(
		p.ABI,
		cmn.ReviewedGRPCErrorRegistry(),
		cmn.ErrorBoundaryMsgServer,
		method,
		err,
	)
	if grpcTranslation.Matched {
		return grpcTranslation.Revert
	}

	if _, ok := cmn.ExtractCosmosErrorKey(err); ok {
		return p.translateStakingError(ctx, method, err)
	}
	return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, method, err.Error())
}

func (p Precompile) stakingQueryError(ctx sdk.Context, method string, err error) error {
	if _, ok := cmn.ExtractCosmosErrorKey(err); ok {
		return p.translateStakingError(ctx, method, err)
	}
	return cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrQueryFailed, method, err.Error())
}

func stakingQueryPreservesSuccess(method string, err error) bool {
	disposition, ok := cmn.ReviewedGRPCErrorRegistry().Resolve(cmn.ErrorBoundaryQueryServer, method, err)
	return ok && disposition.Kind == cmn.GRPCDispositionPreserveSuccess
}
