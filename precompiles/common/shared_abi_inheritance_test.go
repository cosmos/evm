package common_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/precompiles/bank"
	"github.com/cosmos/evm/precompiles/bech32"
	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/distribution"
	"github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/precompiles/gov"
	"github.com/cosmos/evm/precompiles/ics02"
	"github.com/cosmos/evm/precompiles/ics20"
	"github.com/cosmos/evm/precompiles/slashing"
	"github.com/cosmos/evm/precompiles/staking"
	"github.com/cosmos/evm/precompiles/werc20"
)

func TestEffectivePrecompileABIsInheritSharedErrors(t *testing.T) {
	tests := map[string]func() error{
		"bank":         func() error { return cmn.ValidateSharedErrorABI(bank.ABI) },
		"bech32":       func() error { return cmn.ValidateSharedErrorABI(bech32.ABI) },
		"distribution": func() error { return cmn.ValidateSharedErrorABI(distribution.ABI) },
		"erc20":        func() error { return cmn.ValidateSharedErrorABI(erc20.ABI) },
		"gov":          func() error { return cmn.ValidateSharedErrorABI(gov.ABI) },
		"ics02":        func() error { return cmn.ValidateSharedErrorABI(ics02.ABI) },
		"ics20":        func() error { return cmn.ValidateSharedErrorABI(ics20.ABI) },
		"slashing":     func() error { return cmn.ValidateSharedErrorABI(slashing.ABI) },
		"staking":      func() error { return cmn.ValidateSharedErrorABI(staking.ABI) },
		"werc20":       func() error { return cmn.ValidateSharedErrorABI(werc20.ABI) },
	}
	for name, validate := range tests {
		t.Run(name, func(t *testing.T) { require.NoError(t, validate()) })
	}

	definition, ok := slashing.ABI.Errors[slashing.SolidityErrSlashingInputInvalid]
	require.True(t, ok)
	require.Equal(t, "SlashingInputInvalid(string,string)", definition.Sig)

	require.NoError(t, cmn.ValidateCosmosErrorRegistry(erc20.ABI, nil, cmn.SharedSDKErrorMappings(), cmn.ApprovedOverrideDeclarations().ForABI("ERC20I")))
	require.NoError(t, cmn.ValidateCosmosErrorRegistry(werc20.ABI, nil, cmn.SharedSDKErrorMappings(), cmn.ApprovedOverrideDeclarations().ForABI("IWERC20")))
}
