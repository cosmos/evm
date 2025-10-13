package eip712

import (
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TestSuite interface {
	// Test Lifecycle
	BeforeEachCase(t *testing.T)
	AfterEachCase(t *testing.T)

	// Tx - EIP-712 signing
	SendBankSendWithEIP712(t *testing.T, nodeID string, accID string, to sdk.AccAddress, amount *big.Int, nonce uint64, gasPrice *big.Int) (string, error)

	// Query
	GetBalance(t *testing.T, nodeID string, address sdk.AccAddress, denom string) (*big.Int, error)
	WaitForCommit(nodeID string, txHash string, timeout ...int) error

	// Config
	Node(idx int) string
	Acc(idx int) string

	// Test Utils
	AwaitNBlocks(t *testing.T, n int64)
}
