package accountabstraction

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"testing"

	//nolint:revive // dot imports are fine for Ginkgo
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	. "github.com/onsi/gomega"

	basesuite "github.com/cosmos/evm/tests/systemtests/suite"
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
}

func (s *TestSuite) GetNonce(accID string) uint64 {
	nonce, err := s.NonceAt("node0", accID)
	Expect(err).To(BeNil())
	return nonce
}

func (s *TestSuite) GetPrivKey(accID string) *ecdsa.PrivateKey {
	return s.SystemTestSuite.EthClient.Accs[accID].PrivKey
}

func (s *TestSuite) GetSmartWalletAddress() common.Address {
	return s.smartWalletAddress
}

func (s *TestSuite) SendSetCodeTx(accID string, signedAuth ethtypes.SetCodeAuthorization) (common.Hash, error) {
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
		GasTipCap:  uint256.NewInt(1000000),
		GasFeeCap:  uint256.NewInt(1000000000),
		Gas:        100000,
		To:         signedAuth.Address,
		Value:      uint256.NewInt(0),
		Data:       []byte{},
		AccessList: ethtypes.AccessList{},
		AuthList:   []ethtypes.SetCodeAuthorization{signedAuth},
	}

	signer := ethtypes.LatestSignerForChainID(chainID)
	signedTx := ethtypes.MustSignNewTx(key, signer, txdata)

	err = ethCli.SendTransaction(ctx, signedTx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to send transaction: %w", err)
	}

	return signedTx.Hash(), nil
}
