package vm

import (
	"bytes"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/contracts"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testKeyring "github.com/cosmos/evm/testutil/keyring"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	"github.com/cosmos/evm/x/vm/types"
)

func TestIterateContracts(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	keyring := testKeyring.New(1)
	opts := []network.ConfigOption{
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	}
	opts = append(opts, options...)
	network := network.NewUnitTestNetwork(create, opts...)
	handler := grpc.NewIntegrationHandler(network)
	factory := factory.New(network, handler)

	contractAddr, err := factory.DeployContract(
		keyring.GetPrivKey(0),
		types.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        contracts.ERC20MinterBurnerDecimalsContract,
			ConstructorArgs: []interface{}{"TestToken", "TTK", uint8(18)},
		},
	)
	require.NoError(t, err, "failed to deploy contract")
	require.NoError(t, network.NextBlock(), "failed to advance block")

	contractAddr2, err := factory.DeployContract(
		keyring.GetPrivKey(0),
		types.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        contracts.ERC20MinterBurnerDecimalsContract,
			ConstructorArgs: []interface{}{"AnotherToken", "ATK", uint8(18)},
		},
	)
	require.NoError(t, err, "failed to deploy contract")
	require.NoError(t, network.NextBlock(), "failed to advance block")

	var (
		foundAddrs  []common.Address
		foundHashes []common.Hash
	)

	network.App.GetEVMKeeper().IterateContracts(network.GetContext(), func(addr common.Address, codeHash common.Hash) bool {
		// NOTE: we only care about the 2 contracts deployed above, not the ERC20 native precompile for the aatom denomination
		if bytes.Equal(addr.Bytes(), common.HexToAddress(testconstants.WEVMOSContractMainnet).Bytes()) {
			return false
		}

		foundAddrs = append(foundAddrs, addr)
		foundHashes = append(foundHashes, codeHash)
		return false
	})

	require.Len(t, foundAddrs, 2, "expected 2 contracts to be found when iterating")
	require.Contains(t, foundAddrs, contractAddr, "expected contract 1 to be found when iterating")
	require.Contains(t, foundAddrs, contractAddr2, "expected contract 2 to be found when iterating")
	require.Equal(t, foundHashes[0], foundHashes[1], "expected both contracts to have the same code hash")
	require.NotEqual(t, types.EmptyCodeHash, foundHashes[0], "expected store code hash not to be the keccak256 of empty code")
}
