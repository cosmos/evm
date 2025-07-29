package common_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
	testutil "github.com/cosmos/evm/testutil"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/evm/x/vm/types/mocks"

	storetypes "cosmossdk.io/store/types"

	sdktestutil "github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func setupBalanceHandlerTest(t *testing.T) {
	t.Helper()

	sdk.GetConfig().SetBech32PrefixForAccount(testconstants.ExampleBech32Prefix, "")
	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	require.NoError(t, configurator.WithEVMCoinInfo(testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]).Configure())
}

func TestParseHexAddress(t *testing.T) {
	var accAddr sdk.AccAddress

	testCases := []struct {
		name     string
		maleate  func() sdk.Event
		key      string
		expAddr  common.Address
		expError bool
	}{
		{
			name: "valid address",
			maleate: func() sdk.Event {
				return sdk.NewEvent("bank", sdk.NewAttribute(banktypes.AttributeKeySpender, accAddr.String()))
			},
			key:      banktypes.AttributeKeySpender,
			expError: false,
		},
		{
			name: "missing attribute",
			maleate: func() sdk.Event {
				return sdk.NewEvent("bank")
			},
			key:      banktypes.AttributeKeySpender,
			expError: true,
		},
		{
			name: "invalid address",
			maleate: func() sdk.Event {
				return sdk.NewEvent("bank", sdk.NewAttribute(banktypes.AttributeKeySpender, "invalid"))
			},
			key:      banktypes.AttributeKeySpender,
			expError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupBalanceHandlerTest(t)

			_, addrs, err := testutil.GeneratePrivKeyAddressPairs(1)
			require.NoError(t, err)
			accAddr = addrs[0]

			event := tc.maleate()

			addr, err := cmn.ParseHexAddress(event, tc.key)
			if tc.expError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, common.BytesToAddress(accAddr), addr)
		})
	}
}

func TestParseAmount(t *testing.T) {
	testCases := []struct {
		name     string
		maleate  func() sdk.Event
		expAmt   *uint256.Int
		expError bool
	}{
		{
			name: "valid amount",
			maleate: func() sdk.Event {
				coinStr := sdk.NewCoins(sdk.NewInt64Coin(evmtypes.GetEVMCoinDenom(), 5)).String()
				return sdk.NewEvent("bank", sdk.NewAttribute(sdk.AttributeKeyAmount, coinStr))
			},
			expAmt: uint256.NewInt(5),
		},
		{
			name: "missing amount",
			maleate: func() sdk.Event {
				return sdk.NewEvent("bank")
			},
			expError: true,
		},
		{
			name: "invalid coins",
			maleate: func() sdk.Event {
				return sdk.NewEvent("bank", sdk.NewAttribute(sdk.AttributeKeyAmount, "invalid"))
			},
			expError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			setupBalanceHandlerTest(t)

			amt, err := cmn.ParseAmount(tc.maleate())
			if tc.expError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.True(t, amt.Eq(tc.expAmt))
		})
	}
}

func TestAfterBalanceChange(t *testing.T) {
	setupBalanceHandlerTest(t)

	storeKey := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_t")
	ctx := sdktestutil.DefaultContext(storeKey, tKey)

	stateDB := statedb.New(ctx, mocks.NewEVMKeeper(), statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash())))

	_, addrs, err := testutil.GeneratePrivKeyAddressPairs(2)
	require.NoError(t, err)
	spenderAcc := addrs[0]
	receiverAcc := addrs[1]
	spender := common.BytesToAddress(spenderAcc)
	receiver := common.BytesToAddress(receiverAcc)

	// initial balance for spender
	stateDB.AddBalance(spender, uint256.NewInt(5), tracing.BalanceChangeUnspecified)

	bh := cmn.NewBalanceHandler()
	bh.BeforeBalanceChange(ctx)

	coins := sdk.NewCoins(sdk.NewInt64Coin(evmtypes.GetEVMCoinDenom(), 3))
	ctx.EventManager().EmitEvents(sdk.Events{
		banktypes.NewCoinSpentEvent(spenderAcc, coins),
		banktypes.NewCoinReceivedEvent(receiverAcc, coins),
	})

	err = bh.AfterBalanceChange(ctx, stateDB)
	require.NoError(t, err)

	require.Equal(t, "2", stateDB.GetBalance(spender).String())
	require.Equal(t, "3", stateDB.GetBalance(receiver).String())
}

func TestAfterBalanceChangeErrors(t *testing.T) {
	setupBalanceHandlerTest(t)

	storeKey := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_t")
	ctx := sdktestutil.DefaultContext(storeKey, tKey)
	stateDB := statedb.New(ctx, mocks.NewEVMKeeper(), statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash())))

	_, addrs, err := testutil.GeneratePrivKeyAddressPairs(1)
	require.NoError(t, err)
	addr := addrs[0]

	bh := cmn.NewBalanceHandler()
	bh.BeforeBalanceChange(ctx)

	// invalid address in event
	coins := sdk.NewCoins(sdk.NewInt64Coin(evmtypes.GetEVMCoinDenom(), 1))
	ctx.EventManager().EmitEvent(banktypes.NewCoinSpentEvent(addr, coins))
	ctx.EventManager().Events()[len(ctx.EventManager().Events())-1].Attributes[0].Value = "invalid"
	err = bh.AfterBalanceChange(ctx, stateDB)
	require.Error(t, err)

	// reset events
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	bh.BeforeBalanceChange(ctx)

	// invalid amount
	ev := sdk.NewEvent(banktypes.EventTypeCoinSpent,
		sdk.NewAttribute(banktypes.AttributeKeySpender, addr.String()),
		sdk.NewAttribute(sdk.AttributeKeyAmount, "invalid"))
	ctx.EventManager().EmitEvent(ev)
	err = bh.AfterBalanceChange(ctx, stateDB)
	require.Error(t, err)
}
