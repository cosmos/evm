package ics20

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cmn "github.com/cosmos/evm/precompiles/common"
	transfertypes "github.com/cosmos/ibc-go/v11/modules/apps/transfer/types"

	"cosmossdk.io/log/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ics20QueryKeeperStub struct {
	denomErr     error
	denomsErr    error
	denomHashErr error
}

func (s ics20QueryKeeperStub) Denom(context.Context, *transfertypes.QueryDenomRequest) (*transfertypes.QueryDenomResponse, error) {
	return nil, s.denomErr
}

func (s ics20QueryKeeperStub) Denoms(context.Context, *transfertypes.QueryDenomsRequest) (*transfertypes.QueryDenomsResponse, error) {
	return nil, s.denomsErr
}

func (s ics20QueryKeeperStub) DenomHash(context.Context, *transfertypes.QueryDenomHashRequest) (*transfertypes.QueryDenomHashResponse, error) {
	return nil, s.denomHashErr
}

func (ics20QueryKeeperStub) Transfer(context.Context, *transfertypes.MsgTransfer) (*transfertypes.MsgTransferResponse, error) {
	return nil, nil
}

func TestICS20QueryNotFoundOutcomesIgnoreMessageText(t *testing.T) {
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
	for _, message := range []string{"denomination not found", "upstream wording completely changed"} {
		denomMethod := ABI.Methods[DenomMethod]
		denomPrecompile := Precompile{
			ABI:            ABI,
			transferKeeper: ics20QueryKeeperStub{denomErr: status.Error(codes.NotFound, message)},
		}
		bz, err := denomPrecompile.Denom(ctx, nil, &denomMethod, []interface{}{"00"})
		require.NoError(t, err)
		unpacked, err := denomMethod.Outputs.Unpack(bz)
		require.NoError(t, err)
		require.Len(t, unpacked, 1)

		denomHashMethod := ABI.Methods[DenomHashMethod]
		denomHashPrecompile := Precompile{
			ABI:            ABI,
			transferKeeper: ics20QueryKeeperStub{denomHashErr: status.Error(codes.NotFound, message)},
		}
		bz, err = denomHashPrecompile.DenomHash(ctx, nil, &denomHashMethod, []interface{}{"transfer/channel-0/uatom"})
		require.NoError(t, err)
		unpacked, err = denomHashMethod.Outputs.Unpack(bz)
		require.NoError(t, err)
		require.Equal(t, []interface{}{string("")}, unpacked)
	}
}

func TestICS20AmbiguousQueryStatusRemainsQueryFailed(t *testing.T) {
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
	method := ABI.Methods[DenomMethod]
	p := Precompile{
		ABI:            ABI,
		transferKeeper: ics20QueryKeeperStub{denomErr: status.Error(codes.InvalidArgument, "looks not found")},
	}
	_, err := p.Denom(ctx, nil, &method, []interface{}{"00"})
	require.Error(t, err)
	require.Equal(t, ics20ErrorSelector(cmn.SolidityErrQueryFailed), err.(cmn.RevertDataCarrier).RevertData()[:4])
}

func TestICS20DirectRegisteredQueryErrorDoesNotUseQueryFallback(t *testing.T) {
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
	method := ABI.Methods[DenomMethod]
	p := Precompile{ABI: ABI, transferKeeper: ics20QueryKeeperStub{denomErr: transfertypes.ErrDenomNotFound}}
	_, err := p.Denom(ctx, nil, &method, []interface{}{"00"})
	require.Error(t, err)
	require.Equal(t, ics20ErrorSelector(SolidityErrIBCTransferDenomNotFound), err.(cmn.RevertDataCarrier).RevertData())
	assertICS20NotFallback(t, err)
}
