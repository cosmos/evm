package systemtests

import (
	"context"
	"encoding/hex"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/systemtests"
)

func StartChain(t *testing.T, sut *systemtests.SystemUnderTest) {
	sut.StartChain(t, "--json-rpc.api=eth,txpool,personal,net,debug,web3", "--chain-id", "local-4221", "--api.enable=true")
}

func TestNonceGappedTxsPass(t *testing.T) {
	//t.Skip("nonce gaps are not yet supported")
	sut := systemtests.Sut
	StartChain(t, sut)
	sut.AwaitNBlocks(t, 10)

	// this PK is derived from the accounts created in testnet.go
	pk := "0x88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305"

	// get the directory of the counter project to run commands from
	_, filename, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(filename)
	counterDir := filepath.Join(testDir, "Counter")

	// deploy the contract
	cmd := exec.Command(
		"forge",
		"create", "src/Counter.sol:Counter",
		"--rpc-url", "http://127.0.0.1:8545",
		"--broadcast",
		"--private-key", pk,
	)
	cmd.Dir = counterDir
	res, err := cmd.CombinedOutput()
	require.NoError(t, err)
	require.NotEmpty(t, string(res))

	// get contract address
	contractAddr := parseContractAddress(string(res))
	require.NotEmpty(t, contractAddr)

	wg := sync.WaitGroup{}

	var gappedRes []byte
	wg.Add(1)
	go func() {
		defer wg.Done()
		var gappedErr error
		gappedRes, gappedErr = exec.Command(
			"cast", "send",
			contractAddr,
			"increment()",
			"--rpc-url", "http://127.0.0.1:8545",
			"--private-key", pk,
			"--nonce", "2",
		).CombinedOutput()
		require.NoError(t, gappedErr)
	}()

	// wait a bit to make sure the tx is submitted and waiting in the txpool.
	time.Sleep(2 * time.Second)

	res, err = exec.Command(
		"cast", "send",
		contractAddr,
		"increment()",
		"--rpc-url", "http://127.0.0.1:8545",
		"--private-key", pk,
		"--nonce", "1",
	).CombinedOutput()
	require.NoError(t, err)

	wg.Wait()

	gappedReceipt, err := parseReceipt(string(gappedRes))
	require.NoError(t, err)

	receipt, err := parseReceipt(string(res))
	require.NoError(t, err)

	// Verify that nonce 1 transaction comes before nonce 2 transaction (even though submitted out of order)
	require.LessOrEqual(t, receipt.BlockNumber.Uint64(), gappedReceipt.BlockNumber.Uint64(), "Nonce 1 should be in same or earlier block than nonce 2")

	// If they're in the same block, verify proper ordering and consecutive indices
	if receipt.BlockNumber.Cmp(gappedReceipt.BlockNumber) == 0 {
		require.Less(t, receipt.TransactionIndex, gappedReceipt.TransactionIndex, "Nonce 1 transaction should come before nonce 2 transaction in same block")
		require.Equal(t, gappedReceipt.TransactionIndex, receipt.TransactionIndex+1, "Transaction indices should be consecutive in same block")
	} else {
		// If they're in different blocks, that's also valid - nonce 2 was queued until nonce 1 was processed
		require.Less(t, receipt.BlockNumber.Uint64(), gappedReceipt.BlockNumber.Uint64(), "Nonce 1 should be in earlier block than nonce 2")
		// Both should be the first transaction in their respective blocks (index 0)
		require.Equal(t, uint(0), receipt.TransactionIndex, "Nonce 1 should be first transaction in its block")
		require.Equal(t, uint(0), gappedReceipt.TransactionIndex, "Nonce 2 should be first transaction in its block")
	}
}

func TestSimpleSendsScript(t *testing.T) {
	t.Skip()
	sut := systemtests.Sut
	StartChain(t, sut)
	sut.AwaitNBlocks(t, 10)
	// this PK is derived from the accounts created in testnet.go
	pk := "0x88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305"

	// get the directory of the counter project to run commands from
	_, filename, _, _ := runtime.Caller(0)
	testDir := filepath.Dir(filename)
	counterDir := filepath.Join(testDir, "Counter")

	// Wait for the RPC endpoint to be fully ready
	time.Sleep(3 * time.Second)

	// First, let's test if forge is available and the script compiles
	compileCmd := exec.Command(
		"forge",
		"build",
	)
	compileCmd.Dir = counterDir
	compileRes, err := compileCmd.CombinedOutput()
	require.NoError(t, err, "Forge build failed: %s", string(compileRes))

	// Set the private key as an environment variable for the script
	cmd := exec.Command(
		"forge",
		"script",
		"script/SimpleSends.s.sol:SimpleSendsScript",
		"--rpc-url", "http://127.0.0.1:8545",
		"--broadcast",
		"--private-key", pk,
		"--gas-limit", "5000000", // Reduced gas limit
		"--timeout", "60", // Add timeout
	)
	cmd.Dir = counterDir
	cmd.Env = append(cmd.Env, "PRIVATE_KEY="+pk)

	// Set a timeout for the command execution
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	cmd.Dir = counterDir
	cmd.Env = append(os.Environ(), "PRIVATE_KEY="+pk)

	res, err := cmd.CombinedOutput()
	require.NoError(t, err, "Script execution failed: %s", string(res))
	require.NotEmpty(t, string(res))

	// Verify the script output contains expected logs
	output := string(res)
	require.Contains(t, output, "Starting simple ETH transfers...")
	require.Contains(t, output, "Sender:")
	require.Contains(t, output, "Sender balance:")
	require.Contains(t, output, "=== Transfer Summary ===")
	require.Contains(t, output, "Total transfers: 10")
	require.Contains(t, output, "Amount per transfer:")
	require.Contains(t, output, "Total sent:")
	require.Contains(t, output, "Remaining balance:")

	// Wait for a few blocks to ensure transactions are processed
	sut.AwaitNBlocks(t, 5)

	// Verify that the script executed without errors
	require.NotContains(t, output, "Error:")
	require.NotContains(t, output, "Failed:")
}

func parseContractAddress(output string) string {
	re := regexp.MustCompile(`Deployed to: (0x[a-fA-F0-9]{40})`)
	matches := re.FindStringSubmatch(output)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func parseReceipt(output string) (*types.Receipt, error) {
	receipt := &types.Receipt{}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "blockHash":
			receipt.BlockHash = common.HexToHash(value)
		case "blockNumber":
			if blockNum, err := strconv.ParseUint(value, 10, 64); err == nil {
				receipt.BlockNumber = big.NewInt(int64(blockNum))
			}
		case "transactionHash":
			receipt.TxHash = common.HexToHash(value)
		case "transactionIndex":
			if txIndex, err := strconv.ParseUint(value, 10, 64); err == nil {
				receipt.TransactionIndex = uint(txIndex)
			}
		case "contractAddress":
			if value != "" {
				receipt.ContractAddress = common.HexToAddress(value)
			}
		case "gasUsed":
			if gasUsed, err := strconv.ParseUint(value, 10, 64); err == nil {
				receipt.GasUsed = gasUsed
			}
		case "cumulativeGasUsed":
			if cumulativeGas, err := strconv.ParseUint(value, 10, 64); err == nil {
				receipt.CumulativeGasUsed = cumulativeGas
			}
		case "status":
			if strings.Contains(value, "1") || strings.Contains(value, "success") {
				receipt.Status = types.ReceiptStatusSuccessful
			} else {
				receipt.Status = types.ReceiptStatusFailed
			}
		case "type":
			if txType, err := strconv.ParseUint(value, 10, 8); err == nil {
				receipt.Type = uint8(txType)
			}
		case "logsBloom":
			if bloomHex := strings.TrimPrefix(value, "0x"); bloomHex != "" {
				if bloomBytes, err := hex.DecodeString(bloomHex); err == nil && len(bloomBytes) == 256 {
					copy(receipt.Bloom[:], bloomBytes)
				}
			}
		}
	}

	return receipt, nil
}
