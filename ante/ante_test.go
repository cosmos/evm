package ante_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/ante"

	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// After a node restart no Cosmos tx has run through NewDeductFeeDecorator yet,
// so the SDK global is still empty. Building the ante handler must set it so
// that EVM fee deduction doesn't send fees to an unnamed module account.
func TestNewAnteHandlerSetsFeeRecipientModule(t *testing.T) {
	original := sdkante.FeeRecipientModule
	t.Cleanup(func() { sdkante.FeeRecipientModule = original })

	sdkante.FeeRecipientModule = ""
	ante.NewAnteHandler(ante.HandlerOptions{})
	require.Equal(t, authtypes.FeeCollectorName, sdkante.FeeRecipientModule)
}

func TestNewAnteHandlerKeepsCustomFeeRecipientModule(t *testing.T) {
	original := sdkante.FeeRecipientModule
	t.Cleanup(func() { sdkante.FeeRecipientModule = original })

	sdkante.FeeRecipientModule = "custom_fee_module"
	ante.NewAnteHandler(ante.HandlerOptions{})
	require.Equal(t, "custom_fee_module", sdkante.FeeRecipientModule)
}
