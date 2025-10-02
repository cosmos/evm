package accountabstraction

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

type AccountAbstractionTestSuite interface {
	// Test Utils
	SetupTest(t *testing.T)
	AwaitNBlocks(t *testing.T, n int64, duration ...time.Duration)

	// Query
	GetChainID() uint64
	GetNonce(accID string) uint64
	GetPrivKey(accID string) *ecdsa.PrivateKey
	GetAddr(accID string) common.Address
	GetSmartWalletAddr() common.Address

	// Transaction
	SendSetCodeTx(accID string, signedAuth ...ethtypes.SetCodeAuthorization) (common.Hash, error)

	// Verification
	CheckSetCode(authorityAccID string, delegate common.Address, expectDelegation bool)
}
