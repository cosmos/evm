package slashing

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"

	"cosmossdk.io/core/address"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/types"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

// ParseSigningInfoArgs parses the arguments for the signing info query
func ParseSigningInfoArgs(args GetSigningInfoCall, consCodec address.Codec) (*slashingtypes.QuerySigningInfoRequest, error) {
	hexAddr := args.ConsAddress
	if hexAddr == (common.Address{}) {
		return nil, fmt.Errorf("invalid consensus address")
	}

	consAddr, err := consCodec.BytesToString(hexAddr.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to convert consensus address: %w", err)
	}

	return &slashingtypes.QuerySigningInfoRequest{
		ConsAddress: consAddr,
	}, nil
}

// ParseSigningInfosArgs parses the arguments for the signing infos query
func ParseSigningInfosArgs(args GetSigningInfosCall) (*slashingtypes.QuerySigningInfosRequest, error) {
	return &slashingtypes.QuerySigningInfosRequest{
		Pagination: args.Pagination.ToPageRequest(),
	}, nil
}

// FromResponse populates GetSigningInfoReturn from a QuerySigningInfoResponse
func (ret *GetSigningInfoReturn) FromResponse(res *slashingtypes.QuerySigningInfoResponse) error {
	consAddr, err := types.ConsAddressFromBech32(res.ValSigningInfo.Address)
	if err != nil {
		return fmt.Errorf("error parsing consensus address: %w", err)
	}

	ret.SigningInfo = SigningInfo{
		ValidatorAddress:    common.BytesToAddress(consAddr.Bytes()),
		StartHeight:         res.ValSigningInfo.StartHeight,
		IndexOffset:         res.ValSigningInfo.IndexOffset,
		JailedUntil:         res.ValSigningInfo.JailedUntil.Unix(),
		Tombstoned:          res.ValSigningInfo.Tombstoned,
		MissedBlocksCounter: res.ValSigningInfo.MissedBlocksCounter,
	}
	return nil
}

// FromResponse populates GetSigningInfosReturn from a QuerySigningInfosResponse
func (ret *GetSigningInfosReturn) FromResponse(res *slashingtypes.QuerySigningInfosResponse) error {
	ret.SigningInfos = make([]SigningInfo, len(res.Info))
	for i, info := range res.Info {
		consAddr, err := types.ConsAddressFromBech32(info.Address)
		if err != nil {
			return fmt.Errorf("error parsing consensus address: %w", err)
		}
		ret.SigningInfos[i] = SigningInfo{
			ValidatorAddress:    common.BytesToAddress(consAddr.Bytes()),
			StartHeight:         info.StartHeight,
			IndexOffset:         info.IndexOffset,
			JailedUntil:         info.JailedUntil.Unix(),
			Tombstoned:          info.Tombstoned,
			MissedBlocksCounter: info.MissedBlocksCounter,
		}
	}
	if res.Pagination != nil {
		ret.PageResponse = cmn.PageResponse{
			NextKey: res.Pagination.NextKey,
			Total:   res.Pagination.Total,
		}
	}
	return nil
}

// FromResponse populates GetParamsReturn from a QueryParamsResponse
func (ret *GetParamsReturn) FromResponse(res *slashingtypes.QueryParamsResponse) error {
	ret.Params = Params{
		SignedBlocksWindow: res.Params.SignedBlocksWindow,
		MinSignedPerWindow: cmn.Dec{
			Value:     res.Params.MinSignedPerWindow.BigInt(),
			Precision: math.LegacyPrecision,
		},
		DowntimeJailDuration: int64(res.Params.DowntimeJailDuration.Seconds()),
		SlashFractionDoubleSign: cmn.Dec{
			Value:     res.Params.SlashFractionDoubleSign.BigInt(),
			Precision: math.LegacyPrecision,
		},
		SlashFractionDowntime: cmn.Dec{
			Value:     res.Params.SlashFractionDowntime.BigInt(),
			Precision: math.LegacyPrecision,
		},
	}
	return nil
}
