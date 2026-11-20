package common_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
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
	tests := map[string]abi.ABI{
		"common":       cmn.SharedErrorABI,
		"bank":         bank.ABI,
		"bech32":       bech32.ABI,
		"distribution": distribution.ABI,
		"erc20":        erc20.ABI,
		"gov":          gov.ABI,
		"ics02":        ics02.ABI,
		"ics20":        ics20.ABI,
		"slashing":     slashing.ABI,
		"staking":      staking.ABI,
		"werc20":       werc20.ABI,
	}
	for name, contractABI := range tests {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, cmn.ValidateSharedErrorABI(contractABI))
		})
	}

	sharedPageRequest := cmn.SharedErrorABI.Errors[cmn.SolidityErrInvalidPageRequest]
	require.Equal(t, "InvalidPageRequest(string,uint256,string)", sharedPageRequest.Sig)

	definition, ok := slashing.ABI.Errors[slashing.SolidityErrSlashingInputInvalid]
	require.True(t, ok)
	require.Equal(t, "SlashingInputInvalid(string,string)", definition.Sig)

	require.NoError(t, cmn.ValidateCosmosErrorRegistry(erc20.ABI, nil, cmn.SharedSDKErrorMappings(), cmn.ApprovedOverrideDeclarations().ForABI("ERC20I")))
	require.NoError(t, cmn.ValidateCosmosErrorRegistry(werc20.ABI, nil, cmn.SharedSDKErrorMappings(), cmn.ApprovedOverrideDeclarations().ForABI("IWERC20")))
}
