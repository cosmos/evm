package cosmos_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/ante/cosmos"
	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/testutil/constants"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestAuthzLimiterDecorator(t *testing.T) {
	evmConfigurator := evmtypes.NewEVMConfigurator().
		WithEVMCoinInfo(constants.ExampleChainCoinInfo[constants.ExampleChainID])
	err := evmConfigurator.Configure()
	require.NoError(t, err)

	encodingCfg := encoding.MakeConfig(constants.ExampleChainID.EVMChainID)
	txCfg := encodingCfg.TxConfig
	testPrivKeys, testAddresses, err := testutil.GeneratePrivKeyAddressPairs(5)
	require.NoError(t, err)

	evmDenom := evmtypes.GetEVMCoinDenom()
	distantFuture := time.Date(9000, 1, 1, 0, 0, 0, 0, time.UTC)

	validator := sdk.ValAddress(testAddresses[4])
	stakingAuthDelegate, err := stakingtypes.NewStakeAuthorization([]sdk.ValAddress{validator}, nil, stakingtypes.AuthorizationType_AUTHORIZATION_TYPE_DELEGATE, nil)
	require.NoError(t, err)

	stakingAuthUndelegate, err := stakingtypes.NewStakeAuthorization([]sdk.ValAddress{validator}, nil, stakingtypes.AuthorizationType_AUTHORIZATION_TYPE_UNDELEGATE, nil)
	require.NoError(t, err)

	decorator := cosmos.NewAuthzLimiterDecorator(
		sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}),
		sdk.MsgTypeURL(&stakingtypes.MsgUndelegate{}),
	)

	testCases := []struct {
		name        string
		msgs        []sdk.Msg
		checkTx     bool
		expectedErr error
	}{
		{
			"enabled msg - non blocked msg",
			[]sdk.Msg{
				banktypes.NewMsgSend(
					testAddresses[0],
					testAddresses[1],
					sdk.NewCoins(sdk.NewInt64Coin(evmDenom, 100e6)),
				),
			},
			false,
			nil,
		},
		{
			"enabled msg MsgEthereumTx - blocked msg not wrapped in MsgExec",
			[]sdk.Msg{
				&evmtypes.MsgEthereumTx{},
			},
			false,
			nil,
		},
		{
			"enabled msg - blocked msg not wrapped in MsgExec",
			[]sdk.Msg{
				&stakingtypes.MsgCancelUnbondingDelegation{},
			},
			false,
			nil,
		},
		{
			"enabled msg - MsgGrant contains a non blocked msg",
			[]sdk.Msg{
				testutil.NewMsgGrant(
					testAddresses[0],
					testAddresses[1],
					authz.NewGenericAuthorization(sdk.MsgTypeURL(&banktypes.MsgSend{})),
					&distantFuture,
				),
			},
			false,
			nil,
		},
		{
			"enabled msg - MsgGrant contains a non blocked msg",
			[]sdk.Msg{
				testutil.NewMsgGrant(
					testAddresses[0],
					testAddresses[1],
					stakingAuthDelegate,
					&distantFuture,
				),
			},
			false,
			nil,
		},
		{
			"disabled msg - MsgGrant contains a blocked msg",
			[]sdk.Msg{
				testutil.NewMsgGrant(
					testAddresses[0],
					testAddresses[1],
					authz.NewGenericAuthorization(sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{})),
					&distantFuture,
				),
			},
			false,
			sdkerrors.ErrUnauthorized,
		},
		{
			"disabled msg - MsgGrant contains a blocked msg",
			[]sdk.Msg{
				testutil.NewMsgGrant(
					testAddresses[0],
					testAddresses[1],
					stakingAuthUndelegate,
					&distantFuture,
				),
			},
			false,
			sdkerrors.ErrUnauthorized,
		},
		{
			"allowed msg - when a MsgExec contains a non blocked msg",
			[]sdk.Msg{
				testutil.NewMsgExec(
					testAddresses[1],
					[]sdk.Msg{banktypes.NewMsgSend(
						testAddresses[0],
						testAddresses[3],
						sdk.NewCoins(sdk.NewInt64Coin(evmDenom, 100e6)),
					)}),
			},
			false,
			nil,
		},
		{
			"disabled msg - MsgExec contains a blocked msg",
			[]sdk.Msg{
				testutil.NewMsgExec(
					testAddresses[1],
					[]sdk.Msg{
						&evmtypes.MsgEthereumTx{},
					},
				),
			},
			false,
			sdkerrors.ErrUnauthorized,
		},
		{
			"disabled msg - surrounded by valid msgs",
			[]sdk.Msg{
				testutil.NewMsgGrant(
					testAddresses[0],
					testAddresses[1],
					stakingAuthDelegate,
					&distantFuture,
				),
				testutil.NewMsgExec(
					testAddresses[1],
					[]sdk.Msg{
						banktypes.NewMsgSend(
							testAddresses[0],
							testAddresses[3],
							sdk.NewCoins(sdk.NewInt64Coin(evmDenom, 100e6)),
						),
						&evmtypes.MsgEthereumTx{},
					},
				),
			},
			false,
			sdkerrors.ErrUnauthorized,
		},
		{
			"disabled msg - nested MsgExec containing a blocked msg",
			[]sdk.Msg{
				testutil.CreateNestedMsgExec(
					testAddresses[1],
					2,
					[]sdk.Msg{
						&evmtypes.MsgEthereumTx{},
					},
				),
			},
			false,
			sdkerrors.ErrUnauthorized,
		},
		{
			"disabled msg - nested MsgGrant containing a blocked msg",
			[]sdk.Msg{
				testutil.NewMsgExec(
					testAddresses[1],
					[]sdk.Msg{
						testutil.NewMsgGrant(
							testAddresses[0],
							testAddresses[1],
							authz.NewGenericAuthorization(sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{})),
							&distantFuture,
						),
					},
				),
			},
			false,
			sdkerrors.ErrUnauthorized,
		},
		{
			"disabled msg - nested MsgExec NOT containing a blocked msg but has more nesting levels than the allowed",
			[]sdk.Msg{
				testutil.CreateNestedMsgExec(
					testAddresses[1],
					6,
					[]sdk.Msg{
						banktypes.NewMsgSend(
							testAddresses[0],
							testAddresses[3],
							sdk.NewCoins(sdk.NewInt64Coin(evmDenom, 100e6)),
						),
					},
				),
			},
			false,
			sdkerrors.ErrUnauthorized,
		},
		{
			"disabled msg - multiple two nested MsgExec messages NOT containing a blocked msg over the limit",
			[]sdk.Msg{
				testutil.CreateNestedMsgExec(
					testAddresses[1],
					5,
					[]sdk.Msg{
						banktypes.NewMsgSend(
							testAddresses[0],
							testAddresses[3],
							sdk.NewCoins(sdk.NewInt64Coin(evmDenom, 100e6)),
						),
					},
				),
				testutil.CreateNestedMsgExec(
					testAddresses[1],
					5,
					[]sdk.Msg{
						banktypes.NewMsgSend(
							testAddresses[0],
							testAddresses[3],
							sdk.NewCoins(sdk.NewInt64Coin(evmDenom, 100e6)),
						),
					},
				),
			},
			false,
			sdkerrors.ErrUnauthorized,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Case %s", tc.name), func(t *testing.T) {
			ctx := sdk.Context{}.WithIsCheckTx(tc.checkTx)
			tx, err := testutil.CreateTx(ctx, txCfg, testPrivKeys[0], tc.msgs...)
			require.NoError(t, err)

			_, err = decorator.AnteHandle(ctx, tx, false, testutil.NoOpNextFn)
			if tc.expectedErr != nil {
				require.Error(t, err)
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
