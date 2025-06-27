package common

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	testutil "github.com/cosmos/evm/testutil"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

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
	setupBalanceHandlerTest(t)

	_, addrs, err := testutil.GeneratePrivKeyAddressPairs(1)
	require.NoError(t, err)
	accAddr := addrs[0]

	// valid address
	ev := sdk.NewEvent("bank", sdk.NewAttribute(banktypes.AttributeKeySpender, accAddr.String()))
	addr, err := parseHexAddress(ev, banktypes.AttributeKeySpender)
	require.NoError(t, err)
	require.Equal(t, common.Address(accAddr.Bytes()), addr)

	// missing attribute
	ev = sdk.NewEvent("bank")
	_, err = parseHexAddress(ev, banktypes.AttributeKeySpender)
	require.Error(t, err)

	// invalid address
	ev = sdk.NewEvent("bank", sdk.NewAttribute(banktypes.AttributeKeySpender, "invalid"))
	_, err = parseHexAddress(ev, banktypes.AttributeKeySpender)
	require.Error(t, err)
}

func TestParseAmount(t *testing.T) {
	setupBalanceHandlerTest(t)

	coinStr := sdk.NewCoins(sdk.NewInt64Coin(evmtypes.GetEVMCoinDenom(), 5)).String()
	ev := sdk.NewEvent("bank", sdk.NewAttribute(sdk.AttributeKeyAmount, coinStr))
	amt, err := parseAmount(ev)
	require.NoError(t, err)
	require.True(t, amt.Eq(uint256.NewInt(5)))

	// missing amount
	ev = sdk.NewEvent("bank")
	_, err = parseAmount(ev)
	require.Error(t, err)

	// invalid coins
	ev = sdk.NewEvent("bank", sdk.NewAttribute(sdk.AttributeKeyAmount, "invalid"))
	_, err = parseAmount(ev)
	require.Error(t, err)
}

func TestAfterBalanceChange(t *testing.T) {
	setupBalanceHandlerTest(t)

	storeKey := storetypes.NewKVStoreKey("test")
	tKey := storetypes.NewTransientStoreKey("test_t")
	ctx := sdktestutil.DefaultContext(storeKey, tKey)

	stateDB := statedb.New(ctx, testutil.NewMockKeeper(), statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash())))

	_, addrs, err := testutil.GeneratePrivKeyAddressPairs(2)
	require.NoError(t, err)
	spenderAcc := addrs[0]
	receiverAcc := addrs[1]
	spender := common.Address(spenderAcc.Bytes())
	receiver := common.Address(receiverAcc.Bytes())

	// initial balance for spender
	stateDB.AddBalance(spender, uint256.NewInt(5), tracing.BalanceChangeUnspecified)

	bh := NewBalanceHandler()
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
	stateDB := statedb.New(ctx, testutil.NewMockKeeper(), statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash())))

	_, addrs, err := testutil.GeneratePrivKeyAddressPairs(1)
	require.NoError(t, err)
	addr := addrs[0]

	bh := NewBalanceHandler()
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
