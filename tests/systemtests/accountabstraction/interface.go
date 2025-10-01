package accountabstraction

import (
	"crypto/ecdsa"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type AccountAbstractionTestSuite interface {
	// Query
	BaseFee() *big.Int
	WaitForCommit(nodeID string, txHash string, txType string, timeout time.Duration) error
	GetNonce(accID string) uint64

	// Config
	SetupTest(t *testing.T)

	// Test Utils
	AwaitNBlocks(t *testing.T, n int64, duration ...time.Duration)

	// Contracts
	GetSmartWalletAddress() common.Address

	GetPrivKey(accID string) *ecdsa.PrivateKey

	SendSetCodeTx(accID string, signedAuth ethtypes.SetCodeAuthorization) (common.Hash, error)
}
