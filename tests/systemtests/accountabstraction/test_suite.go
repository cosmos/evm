package accountabstraction

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	basesuite "github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/stretchr/testify/require"
)

type TestSuite struct {
	*basesuite.SystemTestSuite

	smartWalletAddress common.Address
}

func NewTestSuite(t *testing.T) *TestSuite {
	return &TestSuite{
		SystemTestSuite: basesuite.NewSystemTestSuite(t),
	}
}

func (s *TestSuite) SetupTest(t *testing.T) {
	s.SystemTestSuite.SetupTest(t)

	smartWalletPath := filepath.Join("..", "..", "contracts", "account_abstraction", "smartwallet", "SimpleSmartWallet.json")
	bytecode, err := loadSmartWalletCreationBytecode(smartWalletPath)
	Expect(err).To(BeNil(), "failed to load smart wallet creation bytecode")

	addr, err := deployContract(s.EthClient, bytecode)
	require.NoError(t, err, "failed to deploy smart wallet contract")
	s.smartWalletAddress = addr
}

func (s *TestSuite) GetChainID() uint64 {
	return s.EthClient.ChainID.Uint64()
}

func (s *TestSuite) GetNonce(accID string) uint64 {
	nonce, err := s.NonceAt("node0", accID)
	Expect(err).To(BeNil())
	return nonce
}

func (s *TestSuite) GetPrivKey(accID string) *ecdsa.PrivateKey {
	return s.EthClient.Accs[accID].PrivKey
}

func (s *TestSuite) GetAddr(accID string) common.Address {
	return s.EthClient.Accs[accID].Address
}

func (s *TestSuite) GetSmartWalletAddr() common.Address {
	return s.smartWalletAddress
}

func (s *TestSuite) SendSetCodeTx(accID string, signedAuths ...ethtypes.SetCodeAuthorization) (common.Hash, error) {
	ctx := context.Background()
	ethCli := s.EthClient.Clients["node0"]
	acc := s.EthClient.Accs[accID]
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

func (s *TestSuite) CheckSetCode(authorityAccID string, delegate common.Address, expectDelegation bool) {
	code, err := s.EthClient.CodeAt("node0", authorityAccID)
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
