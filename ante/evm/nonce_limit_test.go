package evm_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	evmante "github.com/cosmos/evm/ante/evm"

	addresscodec "cosmossdk.io/core/address"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// --- minimal codec to satisfy addresscodec.Codec (not used by these tests) ---
type dummyCodec struct{}

func (dummyCodec) StringToBytes(s string) ([]byte, error) { return nil, nil }
func (dummyCodec) BytesToString(b []byte) (string, error) { return "", nil }

// --- mock implementing anteinterfaces.AccountKeeper exactly ---
type mockAccountKeeper struct{ last sdk.AccountI }

func (m *mockAccountKeeper) NewAccountWithAddress(ctx context.Context, addr sdk.AccAddress) sdk.AccountI {
	return nil
}
func (m *mockAccountKeeper) GetModuleAddress(moduleName string) sdk.AccAddress { return nil }
func (m *mockAccountKeeper) GetAccount(ctx context.Context, addr sdk.AccAddress) sdk.AccountI {
	return m.last
}
func (m *mockAccountKeeper) SetAccount(ctx context.Context, account sdk.AccountI)    { m.last = account }
func (m *mockAccountKeeper) RemoveAccount(ctx context.Context, account sdk.AccountI) {}
func (m *mockAccountKeeper) GetParams(ctx context.Context) (params authtypes.Params) { return params }
func (m *mockAccountKeeper) GetSequence(ctx context.Context, addr sdk.AccAddress) (uint64, error) {
	if m.last == nil {
		return 0, nil
	}
	return m.last.GetSequence(), nil
}
func (m *mockAccountKeeper) AddressCodec() addresscodec.Codec                   { return dummyCodec{} }
func (m *mockAccountKeeper) UnorderedTransactionsEnabled() bool                 { return false }
func (m *mockAccountKeeper) RemoveExpiredUnorderedNonces(ctx sdk.Context) error { return nil }
func (m *mockAccountKeeper) TryAddUnorderedNonce(ctx sdk.Context, sender []byte, timestamp time.Time) error {
	return nil
}

func baseAcc(seq uint64) *authtypes.BaseAccount { return &authtypes.BaseAccount{Sequence: seq} }

func TestIncrementNonce_HappyPath(t *testing.T) {
	var ctx sdk.Context
	ak := &mockAccountKeeper{}
	acc := baseAcc(7)

	err := evmante.IncrementNonce(ctx, ak, acc, 7)
	require.NoError(t, err)
	require.Equal(t, uint64(8), acc.GetSequence())
	require.Equal(t, acc, ak.last) // SetAccount called
}

func TestIncrementNonce_NonceMismatch(t *testing.T) {
	var ctx sdk.Context
	ak := &mockAccountKeeper{}
	acc := baseAcc(10)

	err := evmante.IncrementNonce(ctx, ak, acc, 9)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid nonce")
	require.Equal(t, uint64(10), acc.GetSequence()) // unchanged
}

func TestIncrementNonce_OverflowGuard(t *testing.T) {
	var ctx sdk.Context
	ak := &mockAccountKeeper{}
	acc := baseAcc(math.MaxUint64)

	err := evmante.IncrementNonce(ctx, ak, acc, math.MaxUint64)
	require.Error(t, err)
	require.Contains(t, err.Error(), "overflow")
	require.Equal(t, uint64(math.MaxUint64), acc.GetSequence()) // unchanged
}
