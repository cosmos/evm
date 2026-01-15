package bank

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

const testDenom = "atoken"

func TestERC20InitCode(t *testing.T) {
	bankPrecompile := common.HexToAddress(evmtypes.BankPrecompileAddress)
	denom := testDenom

	initcode, err := ERC20InitCode(denom, bankPrecompile)
	require.NoError(t, err)
	require.NotEmpty(t, initcode)

	require.True(t, len(initcode) > len(ERC20Bin))
	require.Equal(t, ERC20Bin, initcode[:len(ERC20Bin)])
}

func TestCreate2DeployData(t *testing.T) {
	bankPrecompile := common.HexToAddress(evmtypes.BankPrecompileAddress)
	denom := testDenom

	deployData, err := Create2DeployData(denom, bankPrecompile)
	require.NoError(t, err)
	require.NotEmpty(t, deployData)

	require.True(t, len(deployData) > 32)
	require.Equal(t, ERC20Salt, deployData[:32])

	initcode, err := ERC20InitCode(denom, bankPrecompile)
	require.NoError(t, err)
	require.Equal(t, initcode, deployData[32:])
}

func TestCreate2DeployDataMatchesExpectedAddress(t *testing.T) {
	bankPrecompile := common.HexToAddress(evmtypes.BankPrecompileAddress)
	denom := testDenom

	expectedAddr, err := ERC20ContractAddress(bankPrecompile, denom)
	require.NoError(t, err)

	deployData, err := Create2DeployData(denom, bankPrecompile)
	require.NoError(t, err)

	require.NotEqual(t, common.Address{}, expectedAddr)
	require.NotEmpty(t, deployData)

	t.Logf("Denom: %s", denom)
	t.Logf("Expected ERC20 address: %s", expectedAddr.Hex())
	t.Logf("Deploy data length: %d bytes", len(deployData))
}

func TestMultipleDenoms(t *testing.T) {
	bankPrecompile := common.HexToAddress(evmtypes.BankPrecompileAddress)

	denoms := []string{
		testDenom,
		"uatom",
		"ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2",
	}

	addresses := make(map[common.Address]string)

	for _, denom := range denoms {
		addr, err := ERC20ContractAddress(bankPrecompile, denom)
		require.NoError(t, err)
		require.NotEqual(t, common.Address{}, addr)

		if existingDenom, exists := addresses[addr]; exists {
			t.Fatalf("Address collision: %s and %s both map to %s", denom, existingDenom, addr.Hex())
		}
		addresses[addr] = denom

		deployData, err := Create2DeployData(denom, bankPrecompile)
		require.NoError(t, err)
		require.NotEmpty(t, deployData)

		t.Logf("Denom: %s -> %s", denom, addr.Hex())
	}
}
