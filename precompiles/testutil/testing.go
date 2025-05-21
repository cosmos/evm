package testutil

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// NewPrecompileContract creates a new precompile contract and sets the gas meter.
func NewPrecompileContract(t *testing.T, ctx sdk.Context, caller common.Address, precompile common.Address,
	gas uint64,
) (*vm.Contract, sdk.Context) {
	t.Helper()
	contract := vm.NewContract(caller, precompile, uint256.NewInt(0), gas, nil)
	ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	initialGas := ctx.GasMeter().GasConsumed()
	require.Equal(t, uint64(0), initialGas)
	return contract, ctx
}
