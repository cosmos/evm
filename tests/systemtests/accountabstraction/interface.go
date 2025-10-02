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
	// Lifecycle
	SetupTest(t *testing.T)
	AwaitNBlocks(t *testing.T, n int64, duration ...time.Duration)

	// Query helpers
	GetChainID() uint64
	GetNonce(accID string) uint64
	GetPrivKey(accID string) *ecdsa.PrivateKey
	GetAddr(accID string) common.Address
	GetCounterAddr() common.Address

	// Transactions
	SendSetCodeTx(accID string, signedAuth ...ethtypes.SetCodeAuthorization) (common.Hash, error)
	InvokeCounter(accID string, method string, args ...interface{}) (common.Hash, error)

	// Verification
	CheckSetCode(authorityAccID string, delegate common.Address, expectDelegation bool)
	QueryCounterNumber(accID string) (*big.Int, error)
}
