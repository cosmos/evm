package bech32

import (
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"
)

func TestBech32InheritsCanonicalSharedErrorABI(t *testing.T) {
	require.NoError(t, cmn.ValidateSharedErrorABI(ABI))
	require.NoError(t, cmn.ValidateCosmosErrorRegistry(ABI, nil, cmn.SharedSDKErrorMappings(), nil))
}
