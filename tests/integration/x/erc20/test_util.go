package erc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/contracts"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	"github.com/cosmos/evm/x/erc20/keeper/testdata"
	"github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// MintFeeCollector mints some coins to the fee collector address.
// Use this only for unit tests. For integration tests, you can use the
// mintFeeCollector flag to setup some balance on genesis
func (s *KeeperTestSuite) MintFeeCollector(coins sdk.Coins) {
	err := s.network.App.GetBankKeeper().MintCoins(s.network.GetContext(), types.ModuleName, coins)
	s.Require().NoError(err)
	err = s.network.App.GetBankKeeper().SendCoinsFromModuleToModule(s.network.GetContext(), types.ModuleName, authtypes.FeeCollectorName, coins)
	s.Require().NoError(err)
}

func (s *KeeperTestSuite) DeployContract(name, symbol string, decimals uint8) (common.Address, error) {
	addr, err := s.factory.DeployContract(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        contracts.ERC20MinterBurnerDecimalsContract,
			ConstructorArgs: []interface{}{name, symbol, decimals},
		},
	)
	if err != nil {
		return common.Address{}, err
	}

	return addr, s.network.NextBlock()
}

func (s *KeeperTestSuite) DeployContractMaliciousDelayed() (common.Address, error) {
	maliciousDelayedContract, err := testdata.LoadMaliciousDelayedContract()
	s.Require().NoError(err, "failed to load malicious delayed contract")

	addr, err := s.factory.DeployContract(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        maliciousDelayedContract,
			ConstructorArgs: []interface{}{big.NewInt(1000000000000000000)},
		},
	)
	if err != nil {
		return common.Address{}, err
	}

	return addr, s.network.NextBlock()
}

func (s *KeeperTestSuite) DeployContractDirectBalanceManipulation() (common.Address, error) {
	balanceManipulationContract, err := testdata.LoadBalanceManipulationContract()
	s.Require().NoError(err, "failed to load balance manipulation contract")

	addr, err := s.factory.DeployContract(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        balanceManipulationContract,
			ConstructorArgs: []interface{}{big.NewInt(1000000000000000000)},
		},
	)
	if err != nil {
		return common.Address{}, err
	}

	return addr, s.network.NextBlock()
}

func (s *KeeperTestSuite) DeployBytes32MetadataTokenContract(name, symbol string) (common.Address, error) {
	bytes32MetadataTokenContract, err := testdata.LoadBytes32MetadataTokenContract()
	if err != nil {
		return common.Address{}, err
	}

	// Convert strings to bytes32 format for the Solidity constructor
	nameBytes32 := [32]byte{}
	symbolBytes32 := [32]byte{}
	copy(nameBytes32[:], []byte(name))
	copy(symbolBytes32[:], []byte(symbol))

	addr, err := s.factory.DeployContract(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        bytes32MetadataTokenContract,
			ConstructorArgs: []interface{}{nameBytes32, symbolBytes32},
		},
	)
	if err != nil {
		return common.Address{}, err
	}

	return addr, s.network.NextBlock()
}
