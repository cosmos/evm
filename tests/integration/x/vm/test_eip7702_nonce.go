package vm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	testKeyring "github.com/cosmos/evm/testutil/keyring"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	"github.com/cosmos/evm/x/vm/keeper/testdata"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

// TestEIP7702NonceIncrement tests that when a user with EIP-7702 delegation
// deploys a contract whose constructor calls back to the user (via the delegation)
// and deploys more contracts, the user's nonce is correctly incremented.
//
// This test reproduces a bug where the nonce was unconditionally reset to msg.Nonce+1
// after evm.Create(), ignoring any nonce increments that occurred during execution.
//
// Bug scenario:
// 1. User (with EIP-7702 delegation) sends contract creation tx with nonce N
// 2. evm.Create() increments User's nonce to N+1
// 3. Constructor calls back to User via delegation, deploying more contracts
// 4. Each nested CREATE increments User's nonce (N+2, N+3, ...)
// 5. BUG: After evm.Create() returns, nonce is reset to N+1, losing all increments
func (s *KeeperTestSuite) TestEIP7702NonceIncrement() {
	s.SetupTest()

	delegationTarget, err := testdata.LoadDelegationTargetContract()
	s.Require().NoError(err)

	maliciousDeployer, err := testdata.LoadMaliciousDeployerContract()
	s.Require().NoError(err)

	chainIDUint256 := uint256.NewInt(evmtypes.GetChainConfig().GetChainId())

	// Use key 0 as BOTH the delegated EOA AND the deployer
	// This is critical - the bug only manifests when the tx sender
	// is the same address that gets called back via EIP-7702
	user := s.Keyring.GetKey(0)
	// Use key 1 just to deploy the DelegationTarget contract initially
	helper := s.Keyring.GetKey(1)

	// Deploy DelegationTarget contract (using helper account)
	delegationTargetAddr, err := s.Factory.DeployContract(
		helper.Priv,
		evmtypes.EvmTxArgs{GasLimit: 5_000_000},
		testutiltypes.ContractDeploymentData{
			Contract: delegationTarget,
		},
	)
	s.Require().NoError(err)
	s.Require().NoError(s.Network.NextBlock())

	// Set up EIP-7702 delegation for user -> DelegationTarget
	// User delegates their account to DelegationTarget code
	s.setupDelegationSameAccount(user, delegationTargetAddr, chainIDUint256)
	s.Require().NoError(s.Network.NextBlock())

	// Verify delegation was set
	code := s.Network.App.GetEVMKeeper().GetCode(
		s.Network.GetContext(),
		s.Network.App.GetEVMKeeper().GetCodeHash(s.Network.GetContext(), user.Addr),
	)
	delegationAddr, ok := ethtypes.ParseDelegation(code)
	s.Require().True(ok, "delegation should be set")
	s.Require().Equal(delegationTargetAddr, delegationAddr)

	// Get initial nonce of user (should be 2: one for setcode tx auth increment)
	initialNonce := s.Network.App.GetEVMKeeper().GetNonce(s.Network.GetContext(), user.Addr)

	// User deploys MaliciousDeployer
	// The constructor will call back to user.Addr (which has DelegationTarget code)
	// and deploy numContracts contracts through it
	numContracts := uint64(30)

	_, err = s.Factory.DeployContract(
		user.Priv,
		evmtypes.EvmTxArgs{GasLimit: 10_000_000},
		testutiltypes.ContractDeploymentData{
			Contract:        maliciousDeployer,
			ConstructorArgs: []interface{}{user.Addr, big.NewInt(int64(numContracts)), big.NewInt(int64(1))},
		},
	)
	s.Require().NoError(err)
	s.Require().NoError(s.Network.NextBlock())

	// Verify the nonce
	finalNonce := s.Network.App.GetEVMKeeper().GetNonce(s.Network.GetContext(), user.Addr)

	// Expected: initialNonce + 1 (for MaliciousDeployer creation) + numContracts (nested CREATEs)
	// The +1 comes from go-ethereum incrementing nonce at the start of evm.Create()
	// The +numContracts comes from the nested CREATE calls through the delegation
	expectedNonce := initialNonce + 1 + numContracts
	s.Require().Equal(expectedNonce, finalNonce,
		"user nonce should be %d (initial %d + 1 for deployment + %d nested creates), but got %d",
		expectedNonce, initialNonce, numContracts, finalNonce)
}

// setupDelegationSameAccount sets up EIP-7702 delegation where the user
// sends the SetCode tx for themselves. In this case, the authorization nonce
// must be currentNonce + 1 because the tx increments the nonce before
// the authorization is applied.
func (s *KeeperTestSuite) setupDelegationSameAccount(user testKeyring.Key, targetAddr common.Address, chainID *uint256.Int) {
	accResp, err := s.Handler.GetEvmAccount(user.Addr)
	s.Require().NoError(err)

	// When sender == authority, authorization nonce must be currentNonce + 1
	auth := ethtypes.SetCodeAuthorization{
		ChainID: *chainID,
		Address: targetAddr,
		Nonce:   accResp.GetNonce() + 1,
	}

	signedAuth := s.SignSetCodeAuthorization(user, auth)

	zeroAddr := common.Address{}
	txArgs := evmtypes.EvmTxArgs{
		To:                &zeroAddr,
		GasLimit:          100_000,
		AuthorizationList: []ethtypes.SetCodeAuthorization{signedAuth},
	}

	_, err = s.Factory.ExecuteEthTx(user.Priv, txArgs)
	s.Require().NoError(err)
}
