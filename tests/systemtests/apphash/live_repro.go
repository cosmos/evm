//go:build system_test

package apphash

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/tests/systemtests/suite"
)

const (
	liveReproEnv          = "EVM_RUN_LIVE_APPHASH_REPRO"
	liveReproDenom        = "atest"
	liveReproBatches      = 300
	liveReproTxsPerSender = 12
	liveReproBlockWait    = 45 * time.Second
)

func RunLiveHotSendsAppHash(t *testing.T, base *suite.BaseTestSuite) {
	if os.Getenv(liveReproEnv) != "1" {
		t.Skipf("set %s=1 to run the live apphash reproducer", liveReproEnv)
	}

	base.SetupTest(t, suite.DefaultNodeArgs()...)

	recipient := base.EthAccount("acc3").Address
	senders := []*suite.TestAccount{base.Acc(0), base.Acc(1), base.Acc(2)}
	nonces := make(map[string]uint64, len(senders))
	for _, sender := range senders {
		nonce, err := base.NonceAt(base.Node(0), sender.ID)
		require.NoError(t, err)
		nonces[sender.ID] = nonce
	}

	lastCommonHeight, statusByNode := waitForCommonHeight(t, base, 2, liveReproBlockWait)
	t.Logf("starting live apphash reproducer at common height=%d statuses=%s", lastCommonHeight, formatStatuses(statusByNode))
	gasPrice := initialGasPrice(t, base)

	for batch := 0; batch < liveReproBatches; batch++ {
		for i := 0; i < liveReproTxsPerSender; i++ {
			for _, sender := range senders {
				nonce := nonces[sender.ID]
				tx := ethtypes.NewTransaction(nonce, recipient, big.NewInt(1), 21_000, gasPrice, nil)
				_, err := base.EthClient.SendRawTransaction(base.Node(0), sender.Eth, tx)
				require.NoErrorf(t, err, "failed sending sender=%s nonce=%d batch=%d", sender.ID, nonce, batch)
				nonces[sender.ID] = nonce + 1
			}
		}

		targetHeight := lastCommonHeight + 1
		newCommonHeight, statusByNode := waitForCommonHeight(t, base, targetHeight, liveReproBlockWait)
		lastCommonHeight = newCommonHeight

		if mismatch := firstAppHashMismatch(statusByNode); mismatch != "" {
			t.Fatalf("apphash mismatch at height=%d: %s\nbalances:\n%s", newCommonHeight, mismatch, dumpBalances(t, base))
		}

		heights := collectHeights(statusByNode)
		if !allEqual(heights) {
			t.Fatalf("height divergence after batch=%d statuses=%s\nbalances:\n%s", batch, formatStatuses(statusByNode), dumpBalances(t, base))
		}

		if batch%10 == 0 {
			t.Logf("batch=%d height=%d apphash=%s", batch, newCommonHeight, statusByNode[base.Node(0)].AppHash)
		}
	}

	finalStatuses := getStatuses(t, base)
	t.Logf("completed live apphash reproducer without mismatch: %s", formatStatuses(finalStatuses))
}

type nodeStatus struct {
	NodeID  string
	Height  int64
	AppHash string
}

func initialGasPrice(t *testing.T, base *suite.BaseTestSuite) *big.Int {
	t.Helper()

	baseFee, err := base.GetLatestBaseFee(base.Node(0))
	require.NoError(t, err)
	return new(big.Int).Mul(baseFee, big.NewInt(100))
}

func waitForCommonHeight(t *testing.T, base *suite.BaseTestSuite, minHeight int64, timeout time.Duration) (int64, map[string]nodeStatus) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastStatuses map[string]nodeStatus
	for {
		lastStatuses = getStatuses(t, base)
		commonHeight := minCommonHeight(lastStatuses)
		if commonHeight >= minHeight {
			return commonHeight, lastStatuses
		}

		select {
		case <-ctx.Done():
			t.Fatalf(
				"timed out waiting for common height %d; statuses=%s\nbalances:\n%s",
				minHeight,
				formatStatuses(lastStatuses),
				dumpBalances(t, base),
			)
		case <-ticker.C:
		}
	}
}

func getStatuses(t *testing.T, base *suite.BaseTestSuite) map[string]nodeStatus {
	t.Helper()

	statuses := make(map[string]nodeStatus, len(base.Nodes()))
	for _, nodeID := range base.Nodes() {
		res, err := base.CosmosClient.RpcClients[nodeID].Status(context.Background())
		require.NoError(t, err)
		statuses[nodeID] = nodeStatus{
			NodeID:  nodeID,
			Height:  res.SyncInfo.LatestBlockHeight,
			AppHash: strings.ToUpper(hex.EncodeToString(res.SyncInfo.LatestAppHash)),
		}
	}
	return statuses
}

func minCommonHeight(statuses map[string]nodeStatus) int64 {
	minHeight := int64(^uint64(0) >> 1)
	for _, status := range statuses {
		if status.Height < minHeight {
			minHeight = status.Height
		}
	}
	if minHeight == int64(^uint64(0)>>1) {
		return 0
	}
	return minHeight
}

func collectHeights(statuses map[string]nodeStatus) []int64 {
	heights := make([]int64, 0, len(statuses))
	for _, nodeID := range sortedNodeIDs(statuses) {
		heights = append(heights, statuses[nodeID].Height)
	}
	return heights
}

func allEqual[T comparable](values []T) bool {
	if len(values) < 2 {
		return true
	}
	first := values[0]
	for _, value := range values[1:] {
		if value != first {
			return false
		}
	}
	return true
}

func firstAppHashMismatch(statuses map[string]nodeStatus) string {
	if len(statuses) < 2 {
		return ""
	}

	var baseline *nodeStatus
	for _, nodeID := range sortedNodeIDs(statuses) {
		status := statuses[nodeID]
		if baseline == nil {
			baseline = &status
			continue
		}
		if baseline.AppHash != status.AppHash {
			return formatStatuses(statuses)
		}
	}
	return ""
}

func dumpBalances(t *testing.T, base *suite.BaseTestSuite) string {
	t.Helper()

	lines := make([]string, 0, len(base.Nodes())*4)
	for _, nodeID := range base.Nodes() {
		for _, accID := range []string{"acc0", "acc1", "acc2", "acc3"} {
			balance, err := base.CosmosClient.GetBalance(nodeID, base.CosmosAccount(accID).AccAddress, liveReproDenom)
			require.NoError(t, err)
			lines = append(lines, fmt.Sprintf("%s %s balance=%s", nodeID, accID, balance.String()))
		}
	}
	return strings.Join(lines, "\n")
}

func formatStatuses(statuses map[string]nodeStatus) string {
	parts := make([]string, 0, len(statuses))
	for _, nodeID := range sortedNodeIDs(statuses) {
		status := statuses[nodeID]
		parts = append(parts, fmt.Sprintf("%s[h=%d app=%s]", status.NodeID, status.Height, status.AppHash))
	}
	return strings.Join(parts, " ")
}

func sortedNodeIDs(statuses map[string]nodeStatus) []string {
	nodeIDs := make([]string, 0, len(statuses))
	for nodeID := range statuses {
		nodeIDs = append(nodeIDs, nodeID)
	}
	slices.Sort(nodeIDs)
	return nodeIDs
}
