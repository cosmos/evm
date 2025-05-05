package keeper_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/precisebank/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func TestSendCoinsFromAccountToModule_BlockedReserve(t *testing.T) {
	// Other modules shouldn't be able to send x/precisebank coins as the module
	// account balance is for internal reserve use only.

	td := newMockedTestData(t)
	td.ak.EXPECT().
		GetModuleAccount(td.ctx, types.ModuleName).
		Return(authtypes.NewModuleAccount(
			authtypes.NewBaseAccountWithAddress(sdk.AccAddress{100}),
			types.ModuleName,
		)).
		Once()

	fromAddr := sdk.AccAddress([]byte{1})
	err := td.keeper.SendCoinsFromAccountToModule(td.ctx, fromAddr, types.ModuleName, cs(c("busd", 1000)))

	require.Error(t, err)
	require.EqualError(t, err, "module account precisebank is not allowed to receive funds: unauthorized")
}

func TestSendCoinsFromModuleToAccount_BlockedReserve(t *testing.T) {
	// Other modules shouldn't be able to send x/precisebank module account
	// funds.

	td := newMockedTestData(t)
	td.ak.EXPECT().
		GetModuleAddress(types.ModuleName).
		Return(sdk.AccAddress{100}).
		Once()

	toAddr := sdk.AccAddress([]byte{1})
	err := td.keeper.SendCoinsFromModuleToAccount(td.ctx, types.ModuleName, toAddr, cs(c("busd", 1000)))

	require.Error(t, err)
	require.EqualError(t, err, "module account precisebank is not allowed to send funds: unauthorized")
}
