package ics20

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	cmn "github.com/cosmos/evm/precompiles/common"
	precompiletest "github.com/cosmos/evm/precompiles/testutil"
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

func TestICS20MissingDenomSuccessMatchesUpstream(t *testing.T) {
	ctx := sdk.Context{}.WithLogger(log.NewNopLogger())
	for _, name := range []string{DenomMethod, DenomHashMethod} {
		t.Run(name, func(t *testing.T) {
			method := ABI.Methods[name]
			var output interface{} = transfertypes.Denom{}
			if name == DenomHashMethod {
				output = ""
			}
			expected, err := method.Outputs.Pack(output)
			require.NoError(t, err)
			cases := []struct {
				name    string
				err     error
				success bool
			}{
				{"SDK missing denom", status.Error(codes.NotFound, ErrDenomNotFound), true},
				{"wrapped SDK error", fmt.Errorf("outer: %w", status.Error(codes.NotFound, ErrDenomNotFound)), true},
				{"plain matching message", errors.New(ErrDenomNotFound), true},
				{"message substring", fmt.Errorf("prefix: %w: suffix", errors.New(ErrDenomNotFound)), true},
				{"registered missing denom", transfertypes.ErrDenomNotFound, true},
				{"matching message with other status", status.Error(codes.InvalidArgument, ErrDenomNotFound), true},
				{"unrelated NotFound", status.Error(codes.NotFound, "backend record unavailable"), false},
				{"partial message", status.Error(codes.NotFound, "denomination unavailable"), false},
			}
			for _, scenario := range cases {
				t.Run(scenario.name, func(t *testing.T) {
					p := Precompile{ABI: ABI, transferKeeper: ics20QueryKeeperStub{denomErr: scenario.err, denomHashErr: scenario.err}}
					var bz []byte
					var err error
					if name == DenomMethod {
						bz, err = p.Denom(ctx, nil, &method, []interface{}{"00"})
					} else {
						bz, err = p.DenomHash(ctx, nil, &method, []interface{}{"transfer/channel-0/uatom"})
					}
					if scenario.success {
						require.NoError(t, err)
						require.Equal(t, expected, bz)
						return
					}
					require.Nil(t, bz)
					require.Error(t, err)
					revert := cmn.NewRevertWithSolidityError(ABI, cmn.SolidityErrQueryFailed, name, scenario.err.Error())
					require.Equal(t, revert.(cmn.RevertDataCarrier).RevertData(), err.(cmn.RevertDataCarrier).RevertData())
				})
			}
		})
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

func TestICS20CustomQueryServerPreservesTerminalBeforeNotFound(t *testing.T) {
	for _, terminal := range []error{precompiletest.StatusRevert{}, precompiletest.StatusOutOfGas{}} {
		input := fmt.Errorf("%s: %w", ErrDenomNotFound, terminal)
		p := Precompile{ABI: ABI, transferKeeper: ics20QueryKeeperStub{denomErr: input, denomHashErr: input}}
		method := ABI.Methods[DenomMethod]
		_, err := p.Denom(sdk.Context{}, nil, &method, []interface{}{"00"})
		require.Equal(t, input, err)
		method = ABI.Methods[DenomHashMethod]
		_, err = p.DenomHash(sdk.Context{}, nil, &method, []interface{}{"transfer/channel-0/uatom"})
		require.Equal(t, input, err)
	}
}
