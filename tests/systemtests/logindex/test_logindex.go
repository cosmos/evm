//go:build system_test

package logindex

import (
	"context"
	"math/big"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/tests/systemtests/clients"
	"github.com/cosmos/evm/tests/systemtests/suite"
)

// RunLogIndexBlockGlobal verifies that log indices in transaction receipts
// are block-global (cumulative across transactions) per the Ethereum JSON-RPC spec.
// each increment() call emits 2 events. If two txs land in the same block,
// the second tx's logs should have indices 2 and 3, not 0 and 1.
func RunLogIndexBlockGlobal(t *testing.T, base *suite.BaseTestSuite) {
	base.SetupTest(t)
	base.AwaitNBlocks(t, 5)

	_, filename, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(filename)
	counterDir := filepath.Join(testDir, "..", "Counter")

	// deploy EventEmitter contract using acc0
	cmd := exec.Command(
		"forge", "create", "src/EventEmitter.sol:EventEmitter",
		"--rpc-url", clients.JsonRPCUrl0,
		"--broadcast",
		"--private-key", "0x"+clients.Acc0PrivKey,
	)
	cmd.Dir = counterDir
	res, err := cmd.CombinedOutput()
	require.NoError(t, err, "forge create failed: %s", string(res))

	contractAddr := parseContractAddress(string(res))
	require.NotEmpty(t, contractAddr, "failed to parse contract address from: %s", string(res))

	t.Logf("EventEmitter deployed at %s", contractAddr)

	// wait a block for deployment to finalize
	base.AwaitNBlocks(t, 1)

	// send 2 increment() calls from different accounts simultaneously
	// so they land in the same block
	incrementData := common.Hex2Bytes("d09de08a") // increment() selector

	ethCli := base.EthClient
	ctx := context.Background()
	cli := ethCli.Clients["node0"]
	chainID := ethCli.ChainID
	contract := common.HexToAddress(contractAddr)

	acc0 := base.EthAccount("acc0")
	acc1 := base.EthAccount("acc1")

	// get nonces for both accounts
	nonce0, err := cli.NonceAt(ctx, acc0.Address, nil)
	require.NoError(t, err)
	nonce1, err := cli.NonceAt(ctx, acc1.Address, nil)
	require.NoError(t, err)

	// get suggested gas price
	gasPrice, err := cli.SuggestGasPrice(ctx)
	require.NoError(t, err)
	// use a higher gas price to ensure inclusion
	gasPrice = new(big.Int).Mul(gasPrice, big.NewInt(2))

	signer := ethtypes.NewLondonSigner(chainID)

	// build and sign tx for acc0
	tx0 := ethtypes.NewTransaction(nonce0, contract, big.NewInt(0), 100000, gasPrice, incrementData)
	signedTx0, err := ethtypes.SignTx(tx0, signer, acc0.PrivKey)
	require.NoError(t, err)

	// build and sign tx for acc1
	tx1 := ethtypes.NewTransaction(nonce1, contract, big.NewInt(0), 100000, gasPrice, incrementData)
	signedTx1, err := ethtypes.SignTx(tx1, signer, acc1.PrivKey)
	require.NoError(t, err)

	// send both transactions concurrently
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := cli.SendTransaction(ctx, signedTx0)
		require.NoError(t, err)
	}()
	go func() {
		defer wg.Done()
		err := cli.SendTransaction(ctx, signedTx1)
		require.NoError(t, err)
	}()
	wg.Wait()

	t.Logf("tx0 hash: %s", signedTx0.Hash().Hex())
	t.Logf("tx1 hash: %s", signedTx1.Hash().Hex())

	// wait for both receipts
	receipt0, err := ethCli.WaitForCommit("node0", signedTx0.Hash().Hex(), 60*time.Second)
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt0.Status, "tx0 failed")

	receipt1, err := ethCli.WaitForCommit("node0", signedTx1.Hash().Hex(), 60*time.Second)
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt1.Status, "tx1 failed")

	t.Logf("receipt0: block=%d txIndex=%d logs=%d", receipt0.BlockNumber, receipt0.TransactionIndex, len(receipt0.Logs))
	t.Logf("receipt1: block=%d txIndex=%d logs=%d", receipt1.BlockNumber, receipt1.TransactionIndex, len(receipt1.Logs))

	// each increment() emits 2 events (Incremented + ValueSet)
	require.Len(t, receipt0.Logs, 2, "expected 2 logs in tx0")
	require.Len(t, receipt1.Logs, 2, "expected 2 logs in tx1")

	if receipt0.BlockNumber.Cmp(receipt1.BlockNumber) == 0 {
		t.Log("both txs landed in the same block - verifying block-global log indices")

		// determine which tx came first by TransactionIndex
		var first, second *ethtypes.Receipt
		if receipt0.TransactionIndex < receipt1.TransactionIndex {
			first, second = receipt0, receipt1
		} else {
			first, second = receipt1, receipt0
		}

		// first tx's logs should start at some offset (there may be other txs in the block)
		firstLogStart := first.Logs[0].Index
		require.Equal(t, firstLogStart+1, first.Logs[1].Index,
			"first tx's log indices should be consecutive")

		// second tx's logs must continue from first tx's last log index
		require.Equal(t, firstLogStart+2, second.Logs[0].Index,
			"second tx's first log should continue after first tx's logs (got %d, want %d)",
			second.Logs[0].Index, firstLogStart+2)
		require.Equal(t, firstLogStart+3, second.Logs[1].Index,
			"second tx's second log should be consecutive")

		t.Logf("log indices: tx0=[%d,%d] tx1=[%d,%d]",
			first.Logs[0].Index, first.Logs[1].Index,
			second.Logs[0].Index, second.Logs[1].Index)
	} else {
		t.Logf("txs landed in different blocks (%d vs %d) - cannot verify cross-tx log indexing in this run",
			receipt0.BlockNumber, receipt1.BlockNumber)
		t.Log("each tx's logs should still be internally consistent")

		// still verify per-tx log indices are sequential
		require.Equal(t, receipt0.Logs[0].Index+1, receipt0.Logs[1].Index)
		require.Equal(t, receipt1.Logs[0].Index+1, receipt1.Logs[1].Index)
	}
}

func parseContractAddress(output string) string {
	re := regexp.MustCompile(`Deployed to: (0x[a-fA-F0-9]{40})`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}
