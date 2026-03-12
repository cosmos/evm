//go:build system_test

package apphash

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"

	"github.com/cosmos/evm/tests/systemtests/clients"
	"github.com/cosmos/evm/tests/systemtests/suite"

	systest "github.com/cosmos/cosmos-sdk/testutil/systemtests"
)

const (
	liveReproEnv          = "EVM_RUN_LIVE_APPHASH_REPRO"
	liveReproDenom        = "atest"
	liveReproBatches      = 300
	liveReproTxsPerSender = 12
	liveReproSenderCount  = 100
	liveReproFundAmount   = 1_000_000_000_000_000_000 // 1 ETH per sender
	liveReproBlockWait    = 45 * time.Second
)

// ephemeralSender is a locally-generated account used to maximise mempool contention.
type ephemeralSender struct {
	id  string
	acc *clients.EthAccount
}

func RunLiveHotSendsAppHash(t *testing.T, base *suite.BaseTestSuite) {
	if os.Getenv(liveReproEnv) != "1" {
		t.Skipf("set %s=1 to run the live apphash reproducer", liveReproEnv)
	}

	// Manually set up chain with no_base_fee=true to match the conditions
	// where the apphash divergence was originally observed.
	nodeArgs := append(suite.MinimumGasPriceZeroArgs(), "--log_level=error")
	base.LockChain()
	if base.ChainStarted {
		base.ResetChain(t)
	}
	systest.Sut.ModifyGenesisJSON(t, func(genesis []byte) []byte {
		state, err := sjson.SetBytes(genesis, "app_state.feemarket.params.no_base_fee", true)
		require.NoError(t, err)
		return state
	})

	// Increase EVM mempool limits so 100 senders × 12 txs don't hit the cap.
	for i := 0; i < 4; i++ {
		appToml := filepath.Join(systest.WorkDir, fmt.Sprintf("testnet/node%d/evmd/config/app.toml", i))
		data, err := os.ReadFile(appToml)
		require.NoErrorf(t, err, "reading app.toml for node%d at %s", i, appToml)
		s := string(data)
		s = strings.Replace(s, "global-slots = 5120", "global-slots = 50000", 1)
		s = strings.Replace(s, "global-queue = 1024", "global-queue = 10000", 1)
		require.NoError(t, os.WriteFile(appToml, []byte(s), 0o600))
		t.Logf("patched app.toml for node%d: global-slots=50000 global-queue=10000", i)
	}

	base.StartChain(t, nodeArgs...)
	base.UnlockChain()
	base.AwaitNBlocks(t, 2)

	lastCommonHeight, statusByNode := waitForCommonHeight(t, base, 2, liveReproBlockWait)
	t.Logf("starting live apphash reproducer at common height=%d statuses=%s", lastCommonHeight, formatStatuses(statusByNode))
	gasPrice := initialGasPrice(t, base)

	// Generate ephemeral senders and fund them from genesis accounts.
	senders := generateEphemeralSenders(t, liveReproSenderCount)
	fundEphemeralSenders(t, base, senders, gasPrice)

	// Wait for all funding txs to confirm by polling the last sender's balance.
	t.Logf("waiting for funding txs to confirm...")
	lastSender := senders[len(senders)-1]
	require.Eventually(t, func() bool {
		ctx := context.Background()
		cli := base.EthClient.Clients[base.Node(0)]
		bal, err := cli.BalanceAt(ctx, lastSender.acc.Address, nil)
		return err == nil && bal.Sign() > 0
	}, liveReproBlockWait, 500*time.Millisecond, "funding txs did not confirm in time")

	lastCommonHeight, statusByNode = waitForCommonHeight(t, base, lastCommonHeight+1, liveReproBlockWait)
	t.Logf("funded %d ephemeral senders, height=%d statuses=%s", len(senders), lastCommonHeight, formatStatuses(statusByNode))

	recipient := base.EthAccount("acc3").Address
	nonces := make(map[string]uint64, len(senders)) // all start at 0

	nodes := base.Nodes()
	for batch := 0; batch < liveReproBatches; batch++ {
		var batchSent, batchSkipped int
		for i := 0; i < liveReproTxsPerSender; i++ {
			for si, sender := range senders {
				nonce := nonces[sender.id]
				tx := ethtypes.NewTransaction(nonce, recipient, big.NewInt(100), 21_000, gasPrice, nil)
				// Round-robin across all nodes to stress p2p propagation.
				targetNode := nodes[si%len(nodes)]
				_, err := base.EthClient.SendRawTransaction(targetNode, sender.acc, tx)
				if err != nil {
					// Pool full or underpriced — skip this tx, don't advance nonce.
					batchSkipped++
					continue
				}
				nonces[sender.id] = nonce + 1
				batchSent++
			}
		}

		targetHeight := lastCommonHeight + 1
		newCommonHeight, statusByNode := waitForCommonHeight(t, base, targetHeight, liveReproBlockWait)
		lastCommonHeight = newCommonHeight

		if mismatch := firstAppHashMismatch(statusByNode); mismatch != "" {
			diag := dumpDiagnostics(t, base, newCommonHeight)
			t.Fatalf("apphash mismatch at height=%d: %s\n%s", newCommonHeight, mismatch, diag)
		}

		heights := collectHeights(statusByNode)
		if !allEqual(heights) {
			diag := dumpDiagnostics(t, base, newCommonHeight)
			t.Fatalf("height divergence after batch=%d statuses=%s\n%s", batch, formatStatuses(statusByNode), diag)
		}

		if batch%10 == 0 {
			t.Logf("batch=%d height=%d sent=%d skipped=%d apphash=%s", batch, newCommonHeight, batchSent, batchSkipped, statusByNode[base.Node(0)].AppHash)
		}
	}

	finalStatuses := getStatuses(t, base)
	t.Logf("completed live apphash reproducer without mismatch: %s", formatStatuses(finalStatuses))
}

// generateEphemeralSenders creates fresh ECDSA keypairs for use as senders.
func generateEphemeralSenders(t *testing.T, count int) []*ephemeralSender {
	t.Helper()
	senders := make([]*ephemeralSender, count)
	for i := 0; i < count; i++ {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		senders[i] = &ephemeralSender{
			id: fmt.Sprintf("eph%d", i),
			acc: &clients.EthAccount{
				Address: crypto.PubkeyToAddress(key.PublicKey),
				PrivKey: key,
			},
		}
	}
	return senders
}

// fundEphemeralSenders sends funding txs from the genesis accounts to each ephemeral sender.
// It round-robins across the 3 genesis accounts (acc0-acc2) sequentially.
func fundEphemeralSenders(t *testing.T, base *suite.BaseTestSuite, senders []*ephemeralSender, gasPrice *big.Int) {
	t.Helper()

	funders := []*suite.TestAccount{base.Acc(0), base.Acc(1), base.Acc(2)}
	funderNonces := make([]uint64, len(funders))
	for i, f := range funders {
		nonce, err := base.NonceAt(base.Node(0), f.ID)
		require.NoError(t, err)
		funderNonces[i] = nonce
	}

	fundAmt := new(big.Int).SetUint64(liveReproFundAmount)
	gasLimit := uint64(21_000)

	for i, sender := range senders {
		fIdx := i % len(funders)
		funder := funders[fIdx]
		nonce := funderNonces[fIdx]
		funderNonces[fIdx]++

		tx := ethtypes.NewTransaction(nonce, sender.acc.Address, fundAmt, gasLimit, gasPrice, nil)
		_, err := base.EthClient.SendRawTransaction(base.Node(0), funder.Eth, tx)
		require.NoErrorf(t, err, "fund %s from %s nonce=%d", sender.id, funder.ID, nonce)
	}
}

// nonceAtAddress queries the nonce directly by address (for ephemeral accounts not in the suite registry).
func nonceAtAddress(base *suite.BaseTestSuite, nodeID string, addr common.Address) (uint64, error) {
	ctx := context.Background()
	cli := base.EthClient.Clients[nodeID]
	blockNumber, err := cli.BlockNumber(ctx)
	if err != nil {
		return 0, err
	}
	return cli.NonceAt(ctx, addr, big.NewInt(int64(blockNumber)))
}

type nodeStatus struct {
	NodeID  string
	Height  int64
	AppHash string
}

func initialGasPrice(t *testing.T, base *suite.BaseTestSuite) *big.Int {
	t.Helper()

	baseFee, err := base.GetLatestBaseFee(base.Node(0))
	if err != nil || baseFee == nil || baseFee.Sign() <= 0 {
		// no_base_fee=true: use a fixed gas price above the minimum-gas-prices floor.
		t.Logf("no base fee available, using fixed gas price")
		return big.NewInt(100_000_000_000) // 100 gwei — high enough to stay above mempool floor
	}
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

// dumpDiagnostics collects comprehensive state from all nodes at the given height
// to help identify which layer (EVM, SDK, state) caused a divergence.
func dumpDiagnostics(t *testing.T, base *suite.BaseTestSuite, height int64) string {
	t.Helper()

	var sb strings.Builder
	sb.WriteString("\n=== DIVERGENCE DIAGNOSTICS ===\n")

	// 1. Tx ordering comparison — did all nodes see the same block?
	sb.WriteString("\n--- BLOCK TX ORDERING ---\n")
	dumpBlockTxOrdering(t, base, height, &sb)

	// 2. Base fee comparison — feemarket divergence?
	sb.WriteString("\n--- BASE FEES ---\n")
	dumpBaseFees(t, base, height, &sb)

	// 3. Balance comparison across all nodes for genesis + recipient accounts
	sb.WriteString("\n--- BALANCES ---\n")
	dumpBalancesInto(t, base, &sb)

	// 4. App hash history — find the first divergent height
	sb.WriteString("\n--- APPHASH HISTORY (last 5 heights) ---\n")
	dumpAppHashHistory(t, base, height, &sb)

	sb.WriteString("=== END DIAGNOSTICS ===\n")
	return sb.String()
}

// dumpBlockTxOrdering fetches the block at the given height from each node and
// compares the transaction hashes and their ordering.
func dumpBlockTxOrdering(t *testing.T, base *suite.BaseTestSuite, height int64, sb *strings.Builder) {
	t.Helper()

	type nodeTxList struct {
		nodeID string
		hashes []string
	}

	var results []nodeTxList
	for _, nodeID := range base.Nodes() {
		rpcCli := base.CosmosClient.RpcClients[nodeID]
		block, err := rpcCli.Block(context.Background(), &height)
		if err != nil {
			sb.WriteString(fmt.Sprintf("%s: error fetching block at height %d: %v\n", nodeID, height, err))
			continue
		}

		hashes := make([]string, len(block.Block.Txs))
		for i, tx := range block.Block.Txs {
			hashes[i] = fmt.Sprintf("%X", tx.Hash())
		}
		results = append(results, nodeTxList{nodeID: nodeID, hashes: hashes})
		sb.WriteString(fmt.Sprintf("%s: %d txs\n", nodeID, len(hashes)))
	}

	// Compare tx lists between nodes
	if len(results) >= 2 {
		ref := results[0]
		for _, other := range results[1:] {
			if len(ref.hashes) != len(other.hashes) {
				sb.WriteString(fmt.Sprintf("TX COUNT MISMATCH: %s has %d, %s has %d\n", ref.nodeID, len(ref.hashes), other.nodeID, len(other.hashes)))
				continue
			}
			orderMatch := true
			setMatch := true
			otherSet := make(map[string]bool, len(other.hashes))
			for _, h := range other.hashes {
				otherSet[h] = true
			}
			for i, h := range ref.hashes {
				if !otherSet[h] {
					setMatch = false
				}
				if i < len(other.hashes) && h != other.hashes[i] {
					orderMatch = false
				}
			}
			if !setMatch {
				sb.WriteString(fmt.Sprintf("TX SET MISMATCH between %s and %s — different txs in block!\n", ref.nodeID, other.nodeID))
			} else if !orderMatch {
				sb.WriteString(fmt.Sprintf("TX ORDER MISMATCH between %s and %s — same txs, different order\n", ref.nodeID, other.nodeID))
			} else {
				sb.WriteString(fmt.Sprintf("TX MATCH between %s and %s — identical txs and order\n", ref.nodeID, other.nodeID))
			}
		}
	}
}

// dumpBaseFees queries the base fee at the given height from each node via the eth RPC.
func dumpBaseFees(t *testing.T, base *suite.BaseTestSuite, height int64, sb *strings.Builder) {
	t.Helper()

	ctx := context.Background()
	blockNum := big.NewInt(height)

	for _, nodeID := range base.Nodes() {
		cli := base.EthClient.Clients[nodeID]
		block, err := cli.BlockByNumber(ctx, blockNum)
		if err != nil {
			sb.WriteString(fmt.Sprintf("%s: error fetching eth block %d: %v\n", nodeID, height, err))
			continue
		}
		sb.WriteString(fmt.Sprintf("%s: baseFee=%s gasUsed=%d gasLimit=%d\n",
			nodeID, block.BaseFee().String(), block.GasUsed(), block.GasLimit()))
	}
}

// dumpBalancesInto writes per-node balances for the genesis and recipient accounts.
func dumpBalancesInto(t *testing.T, base *suite.BaseTestSuite, sb *strings.Builder) {
	t.Helper()

	for _, nodeID := range base.Nodes() {
		for _, accID := range []string{"acc0", "acc1", "acc2", "acc3"} {
			balance, err := base.CosmosClient.GetBalance(nodeID, base.CosmosAccount(accID).AccAddress, liveReproDenom)
			if err != nil {
				sb.WriteString(fmt.Sprintf("%s %s: error: %v\n", nodeID, accID, err))
				continue
			}
			sb.WriteString(fmt.Sprintf("%s %s balance=%s\n", nodeID, accID, balance.String()))
		}
	}

	// Cross-node balance diff for quick identification
	nodes := base.Nodes()
	if len(nodes) >= 2 {
		sb.WriteString("\n--- BALANCE DIFFS (node0 vs others) ---\n")
		for _, accID := range []string{"acc0", "acc1", "acc2", "acc3"} {
			bal0, err := base.CosmosClient.GetBalance(nodes[0], base.CosmosAccount(accID).AccAddress, liveReproDenom)
			if err != nil {
				continue
			}
			for _, otherNode := range nodes[1:] {
				balOther, err := base.CosmosClient.GetBalance(otherNode, base.CosmosAccount(accID).AccAddress, liveReproDenom)
				if err != nil {
					continue
				}
				diff := new(big.Int).Sub(bal0, balOther)
				if diff.Sign() != 0 {
					sb.WriteString(fmt.Sprintf("DIFF %s: %s - %s = %s\n", accID, nodes[0], otherNode, diff.String()))
				}
			}
		}
	}
}

// dumpAppHashHistory logs the app hash at each of the last N heights to find where divergence started.
func dumpAppHashHistory(t *testing.T, base *suite.BaseTestSuite, currentHeight int64, sb *strings.Builder) {
	t.Helper()

	lookback := int64(5)
	startHeight := currentHeight - lookback
	if startHeight < 1 {
		startHeight = 1
	}

	firstDivergent := int64(-1)
	for h := startHeight; h <= currentHeight; h++ {
		hashes := make(map[string]string)
		for _, nodeID := range base.Nodes() {
			// Commit at height H produces the app hash stored in block H+1's header.
			// Query the block at H+1 to get the app hash resulting from executing H.
			queryHeight := h + 1
			rpcCli := base.CosmosClient.RpcClients[nodeID]
			block, err := rpcCli.Block(context.Background(), &queryHeight)
			if err != nil {
				hashes[nodeID] = fmt.Sprintf("error: %v", err)
				continue
			}
			hashes[nodeID] = strings.ToUpper(hex.EncodeToString(block.Block.AppHash))
		}

		// Check for divergence at this height
		vals := make([]string, 0, len(hashes))
		for _, v := range hashes {
			vals = append(vals, v)
		}
		diverged := !allEqual(vals)
		marker := ""
		if diverged && firstDivergent == -1 {
			firstDivergent = h
			marker = " <<< FIRST DIVERGENCE"
		} else if diverged {
			marker = " <<< DIVERGED"
		}

		for _, nodeID := range base.Nodes() {
			sb.WriteString(fmt.Sprintf("height=%d %s appHash=%s%s\n", h, nodeID, hashes[nodeID], marker))
		}
	}

	if firstDivergent > 0 {
		sb.WriteString(fmt.Sprintf("\nFirst divergence at height %d\n", firstDivergent))
	}
}

// dumpBalances is kept for backward compatibility with the timeout fatalf paths.
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
