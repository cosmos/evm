//go:build system_test

package accountabstraction

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	suite "github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/stretchr/testify/require"
)

type TestSuite struct {
	*suite.BaseTestSuite

	counterAddress   common.Address
	counterABI       abi.ABI
	primaryAccountID string
}

func NewTestSuite(base *suite.BaseTestSuite) *TestSuite {
	return &TestSuite{
		BaseTestSuite: base,
	}
}

// SetPrimaryAccount configures the account that will be used for contract deployment/setup.
func (s *TestSuite) SetPrimaryAccount(acc *suite.TestAccount) {
	if acc == nil {
		return
	}
	s.primaryAccountID = acc.ID
}

// SetupTest setup test suite and deploy test contracts
func (s *TestSuite) SetupTest(t *testing.T) {
	s.BaseTestSuite.SetupTest(t)

	if s.primaryAccountID == "" {
		s.primaryAccountID = s.AccID(0)
	}

	counterPath := filepath.Join("..", "Counter", "out", "Counter.sol", "Counter.json")
	bytecode, err := loadContractCreationBytecode(counterPath)
	Expect(err).To(BeNil(), "failed to load counter creation bytecode")

	addr, err := deployContract(s.EthClient, s.EthAccount(s.primaryAccountID), bytecode)
	require.NoError(t, err, "failed to deploy counter contract")
	s.counterAddress = addr

	counterABI, err := loadContractABI(counterPath)
	Expect(err).To(BeNil(), "failed to load counter contract abi")
	s.counterABI = counterABI
}

// WaitForCommit waits for a commit of given transaction
func (s *TestSuite) WaitForCommit(txHash common.Hash) {
	_, err := s.EthClient.WaitForCommit("node0", txHash.Hex(), time.Second*10)
	Expect(err).To(BeNil())
}

// GetChainID returns chain id of test network
func (s *TestSuite) GetChainID() uint64 {
	return s.EthClient.ChainID.Uint64()
}

// GetNonce returns current nonce of account
func (s *TestSuite) GetNonce(accID string) uint64 {
	nonce, err := s.NonceAt("node0", accID)
	Expect(err).To(BeNil())
	return nonce
}

// GetSequence returns the Cosmos account sequence for the given account ID.
func (s *TestSuite) GetSequence(accID string) uint64 {
	cosmosAcc := s.CosmosAccount(accID)
	ctx := s.CosmosClient.ClientCtx.WithClient(s.CosmosClient.RpcClients["node0"])
	account, err := ctx.AccountRetriever.GetAccount(ctx, cosmosAcc.AccAddress)
	Expect(err).To(BeNil(), "unable to retrieve cosmos account for %s", accID)
	return account.GetSequence()
}

// GetPrivKey returns ecdsa private key of account
func (s *TestSuite) GetPrivKey(accID string) *ecdsa.PrivateKey {
	return s.EthAccount(accID).PrivKey
}

// GetAddr returns ethereum address of account
func (s *TestSuite) GetAddr(accID string) common.Address {
	return s.EthAccount(accID).Address
}

// GetCounterAddr returns the deployed counter contract address.
func (s *TestSuite) GetCounterAddr() common.Address {
	return s.counterAddress
}

// SendSetCodeTx sends SetCodeTx
func (s *TestSuite) SendSetCodeTx(accID string, signedAuths ...ethtypes.SetCodeAuthorization) (common.Hash, error) {
	ctx := context.Background()
	ethCli := s.EthClient.Clients["node0"]
	acc := s.EthAccount(accID)
	if acc == nil {
		return common.Hash{}, fmt.Errorf("account %s not found", accID)
	}
	key := acc.PrivKey

	chainID, err := ethCli.ChainID(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to get evm chain id")
	}

	fromAddr := acc.Address
	nonce, err := ethCli.PendingNonceAt(ctx, fromAddr)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to fetch pending nonce: %w", err)
	}

	txdata := &ethtypes.SetCodeTx{
		ChainID:    uint256.MustFromBig(chainID),
		Nonce:      nonce,
		GasTipCap:  uint256.NewInt(1_000_000),
		GasFeeCap:  uint256.NewInt(1_000_000_000),
		Gas:        100_000,
		To:         common.Address{},
		Value:      uint256.NewInt(0),
		Data:       []byte{},
		AccessList: ethtypes.AccessList{},
		AuthList:   signedAuths,
	}

	signer := ethtypes.LatestSignerForChainID(chainID)
	signedTx := ethtypes.MustSignNewTx(key, signer, txdata)

	if err := ethCli.SendTransaction(ctx, signedTx); err != nil {
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	return signedTx.Hash(), nil
}

// CheckSetCode checks the account is EIP-7702 SetCode authorized.
func (s *TestSuite) CheckSetCode(authorityAccID string, delegate common.Address, expectDelegation bool) {
	account := s.EthAccount(authorityAccID)
	Expect(account).ToNot(BeNil(), "account %s not found", authorityAccID)

	ctx := context.Background()
	code, err := s.EthClient.Clients["node0"].CodeAt(ctx, account.Address, nil)
	Expect(err).To(BeNil(), "unable to retrieve updated code for %s", authorityAccID)

	if expectDelegation {
		// 3byte prefix + 20byte authorized contract address
		Expect(len(code)).To(Equal(23), "expected delegation code for %s", authorityAccID)
		resolvedAddr, ok := ethtypes.ParseDelegation(code)
		Expect(ok).To(BeTrue(), "expected delegation prefix in code for %s", authorityAccID)
		Expect(resolvedAddr).To(Equal(delegate), "unexpected delegate for %s", authorityAccID)
		return
	} else {
		Expect(len(code)).To(Equal(0), "expected delegation code for %s", authorityAccID)
		_, ok := ethtypes.ParseDelegation(code)
		Expect(ok).To(BeFalse(), "expected delegation prefix in code for %s", authorityAccID)
	}
}

// InvokeCounter sends a transaction from the delegated account to execute a counter method.
func (s *TestSuite) InvokeCounter(accID string, method string, args ...interface{}) (common.Hash, error) {
	account := s.EthAccount(accID)
	if account == nil {
		return common.Hash{}, fmt.Errorf("account %s not found", accID)
	}

	calldata, err := s.counterABI.Pack(method, args...)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack counter calldata: %w", err)
	}

	ctx := context.Background()
	ethCli := s.EthClient.Clients["node0"]
	chainID, err := ethCli.ChainID(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to fetch chain id: %w", err)
	}

	nonce, err := ethCli.PendingNonceAt(ctx, account.Address)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to fetch pending nonce: %w", err)
	}

	gasTipCap := big.NewInt(1_000_000)
	gasFeeCap := big.NewInt(1_000_000_000)
	gasLimit := uint64(500_000)

	to := account.Address
	txData := &ethtypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       gasLimit,
		To:        &to,
		Value:     big.NewInt(0),
		Data:      calldata,
	}

	signer := ethtypes.LatestSignerForChainID(chainID)
	signedTx, err := ethtypes.SignNewTx(account.PrivKey, signer, txData)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to sign counter tx: %w", err)
	}

	if err := ethCli.SendTransaction(ctx, signedTx); err != nil {
		return common.Hash{}, fmt.Errorf("failed to send counter tx: %w", err)
	}

	receipt, err := s.EthClient.WaitForCommit("node0", signedTx.Hash().Hex(), time.Second*10)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to fetch counter tx receipt: %w", err)
	}
	if receipt.Status != 1 {
		return common.Hash{}, fmt.Errorf("counter tx reverted: %s", signedTx.Hash())
	}

	return signedTx.Hash(), nil
}

// QueryCounterNumber queries the delegated counter contract via the account code.
func (s *TestSuite) QueryCounterNumber(accID string) (*big.Int, error) {
	account := s.EthAccount(accID)
	if account == nil {
		return nil, fmt.Errorf("account %s not found", accID)
	}

	calldata, err := s.counterABI.Pack("number")
	if err != nil {
		return nil, fmt.Errorf("failed to pack counter number calldata: %w", err)
	}

	ctx := context.Background()
	ethCli := s.EthClient.Clients["node0"]
	callMsg := ethereum.CallMsg{
		From: account.Address,
		To:   &account.Address,
		Data: calldata,
	}

	output, err := ethCli.CallContract(ctx, callMsg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to call counter contract: %w", err)
	}
	if len(output) == 0 {
		return big.NewInt(0), nil
	}

	values, err := s.counterABI.Unpack("number", output)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack counter result: %w", err)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("counter query returned no values")
	}

	value, ok := values[0].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("unexpected counter return type %T", values[0])
	}

	return new(big.Int).Set(value), nil
}
