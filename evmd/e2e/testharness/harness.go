package testharness

import (
	"context"
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/cosmos/evm/evmd/e2e/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/types/bech32"
)

// Harness is a per-test environment that mirrors the prior EVMSuite fields.
// Each test should call NewEnv(t) to provision its own isolated chain instance.
// No shared state across tests remains.
type Harness struct {
	Chain *Chain

	SenderKey    *ecdsa.PrivateKey
	SenderAddr   common.Address
	SenderBech32 string

	Ctx    context.Context
	Cancel context.CancelFunc
}

// CreateHarness provisions a fresh harness and JSON-RPC client for a single test.
func CreateHarness(t *testing.T) *Harness {
	t.Helper()
	req := require.New(t)

	ctx, cancel := context.WithTimeout(context.Background(), utils.TestSetupTimeout)

	// Generate a funded sender (funded via harness genesis mutation)
	k, err := crypto.GenerateKey()
	req.NoError(err)
	senderAddr := crypto.PubkeyToAddress(k.PublicKey)
	senderBech32, err := bech32.ConvertAndEncode(utils.TestBech32Prefix, senderAddr.Bytes())
	req.NoError(err)

	c := NewChain()
	err = c.Start(ctx, Options{
		RepoRoot:        "..", // tests run from e2e/, Dockerfile lives one level up
		ChainID:         utils.TestChainID,
		EVMChainID:      utils.TestEVMChainID,
		Bech32Prefix:    utils.TestBech32Prefix,
		Denom:           utils.TestDenom,
		DisplayDenom:    utils.DisplayDenom,
		SenderHex:       senderAddr.Hex(),
		SenderBech32:    senderBech32,
		FundSender:      true,
		ValidatorAmount: "5000000000000000000" + utils.TestDenom, // 5e18
		SenderAmount:    "2000000000000000000" + utils.TestDenom, // 2e18
		SelfDelegation:  "1000000000000000000" + utils.TestDenom, // 1e18
		EnableRPC:       true,
		RPCAPIs:         []string{"eth", "txpool", "personal", "net", "debug", "web3"},
		UnbondingTime:   10 * time.Second,
		ConsensusTimeout: map[string]string{
			"timeout_propose":             "2s",
			"timeout_propose_delta":       "200ms",
			"timeout_prevote":             "500ms",
			"timeout_prevote_delta":       "200ms",
			"timeout_precommit":           "500ms",
			"timeout_precommit_delta":     "200ms",
			"timeout_commit":              "1s",
			"timeout_broadcast_tx_commit": "5s",
		},
	})
	req.NoError(err, "failed to start harness")

	h := &Harness{
		Ctx:          ctx,
		Cancel:       cancel,
		Chain:        c,
		SenderKey:    k,
		SenderAddr:   senderAddr,
		SenderBech32: senderBech32,
	}

	// Ensure teardown happens even if the test fails
	t.Cleanup(func() {
		// Attempt to capture useful artifacts before stopping the container
		if h.Chain != nil {
			h.Chain.cleanup(h.Ctx)
		}

		if h.Cancel != nil {
			h.Cancel()
		}
	})

	return h
}
