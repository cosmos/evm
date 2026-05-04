package slashing

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
)

// SigningInfo represents the signing info for a validator
type SigningInfo struct {
	ValidatorAddress    common.Address `abi:"validatorAddress"`
	StartHeight         int64          `abi:"startHeight"`
	IndexOffset         int64          `abi:"indexOffset"`
	JailedUntil         int64          `abi:"jailedUntil"`
	Tombstoned          bool           `abi:"tombstoned"`
	MissedBlocksCounter int64          `abi:"missedBlocksCounter"`
}

// SigningInfoOutput represents the output of the signing info query
type SigningInfoOutput struct {
	SigningInfo SigningInfo
}

// SigningInfosOutput represents the output of the signing infos query
type SigningInfosOutput struct {
	SigningInfos []SigningInfo      `abi:"signingInfos"`
	PageResponse query.PageResponse `abi:"pageResponse"`
}

// SigningInfosInput represents the input for the signing infos query
type SigningInfosInput struct {
	Pagination query.PageRequest `abi:"pagination"`
}

// ParseSigningInfoArgs parses the arguments for the signing info query
func ParseSigningInfoArgs(args []interface{}) (*slashingtypes.QuerySigningInfoRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	hexAddr, ok := args[0].(common.Address)
	if !ok || hexAddr == (common.Address{}) {
		return nil, fmt.Errorf("invalid consensus address")
	}

	return &slashingtypes.QuerySigningInfoRequest{
		ConsAddress: types.ConsAddress(hexAddr.Bytes()).String(),
	}, nil
}

// ParseSigningInfosArgs parses the arguments for the signing infos query
func ParseSigningInfosArgs(method *abi.Method, args []interface{}) (*slashingtypes.QuerySigningInfosRequest, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 1, len(args))
	}

	var input SigningInfosInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, fmt.Errorf("error while unpacking args to SigningInfosInput: %s", err)
	}

	return &slashingtypes.QuerySigningInfosRequest{
		Pagination: &input.Pagination,
	}, nil
}

func (sio *SigningInfoOutput) FromResponse(res *slashingtypes.QuerySigningInfoResponse) *SigningInfoOutput {
	sio.SigningInfo = SigningInfo{
		ValidatorAddress:    common.BytesToAddress([]byte(res.ValSigningInfo.Address)),
		StartHeight:         res.ValSigningInfo.StartHeight,
		IndexOffset:         res.ValSigningInfo.IndexOffset,
		JailedUntil:         res.ValSigningInfo.JailedUntil.Unix(),
		Tombstoned:          res.ValSigningInfo.Tombstoned,
		MissedBlocksCounter: res.ValSigningInfo.MissedBlocksCounter,
	}
	return sio
}

func (sio *SigningInfosOutput) FromResponse(res *slashingtypes.QuerySigningInfosResponse) *SigningInfosOutput {
	sio.SigningInfos = make([]SigningInfo, len(res.Info))
	for i, info := range res.Info {
		sio.SigningInfos[i] = SigningInfo{
			ValidatorAddress:    common.BytesToAddress([]byte(info.Address)),
			StartHeight:         info.StartHeight,
			IndexOffset:         info.IndexOffset,
			JailedUntil:         info.JailedUntil.Unix(),
			Tombstoned:          info.Tombstoned,
			MissedBlocksCounter: info.MissedBlocksCounter,
		}
	}
	if res.Pagination != nil {
		sio.PageResponse = query.PageResponse{
			NextKey: res.Pagination.NextKey,
			Total:   res.Pagination.Total,
		}
	}
	return sio
}

// ValidatorUnjailed defines the data structure for the ValidatorUnjailed event.
type ValidatorUnjailed struct {
	Validator common.Address
}

// Params defines the parameters for the slashing module
type Params struct {
	SignedBlocksWindow      int64   `abi:"signedBlocksWindow"`
	MinSignedPerWindow      cmn.Dec `abi:"minSignedPerWindow"`
	DowntimeJailDuration    int64   `abi:"downtimeJailDuration"`
	SlashFractionDoubleSign cmn.Dec `abi:"slashFractionDoubleSign"`
	SlashFractionDowntime   cmn.Dec `abi:"slashFractionDowntime"`
}

// ParamsOutput represents the output of the params query
type ParamsOutput struct {
	Params Params
}

func (po *ParamsOutput) FromResponse(res *slashingtypes.QueryParamsResponse) *ParamsOutput {
	po.Params = Params{
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
	return po
}
