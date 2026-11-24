package mempool_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/mempool"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestRecheckCosmosRecoversPanic(t *testing.T) {
	panickingAnte := func(ctx sdk.Context, tx sdk.Tx, simulate bool) (sdk.Context, error) {
		panic("Int overflow")
	}

	rechecker := mempool.NewTxRechecker(panickingAnte, nil)

	ctx, err := rechecker.RecheckCosmos(sdk.Context{}, nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Int overflow")
	require.Equal(t, sdk.Context{}, ctx)
}
