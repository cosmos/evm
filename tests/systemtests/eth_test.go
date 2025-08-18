package systemtests

import (
	"context"
	"encoding/hex"
	"fmt"
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

const (
	// this PK is derived from the accounts created in testnet.go
	pk = "0x88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305"
)

func StartChain(t *testing.T, sut *systemtests.SystemUnderTest) {
	sut.StartChain(t, "--json-rpc.api=eth,txpool,personal,net,debug,web3", "--chain-id", "local-4221", "--api.enable=true")
}

// cosmos and eth tx with same nonce.
// tx pool with nonce gapped txs - submit cosmos tx for the gapped txs and first in queue. overlapped txs inbetween pools.

// todo: 2 very fast txs make sure its replaced across everything..
// what happens if 2nd tx is not relayed to the proposer fast enough?
// set 2 vals, one with lots of stake, other v little

func TestPriorityReplacement(t *testing.T) {
	sut := systemtests.Sut
	sut.ResetChain(t)
	StartChain(t, sut)

	sut.AwaitNBlocks(t, 10)

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

	wg.Add(1)
	var lowPrioRes []byte
	go func() {
		defer wg.Done()
		var prioErr error
		lowPrioRes, prioErr = exec.Command(
			"cast", "send",
			contractAddr,
			"increment()",
			"--rpc-url", "http://127.0.0.1:8545",
			"--private-key", pk,
			"--gas-price", "100000000000",
			"--nonce", "2",
		).CombinedOutput()
		require.Error(t, prioErr)
	}()

	var highPrioRes []byte
	wg.Add(1)
	go func() {
		defer wg.Done()
		var prioErr error
		highPrioRes, prioErr = exec.Command(
			"cast", "send",
			contractAddr,
			"increment()",
			"--rpc-url", "http://127.0.0.1:8545",
			"--private-key", pk,
			"--gas-price", "100000000000000",
			"--priority-gas-price", "100",
			"--nonce", "2",
		).CombinedOutput()
		require.NoError(t, prioErr)
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

	lowPrioReceipt, err := parseReceipt(string(lowPrioRes))
	require.NoError(t, err)

	highPrioReceipt, err := parseReceipt(string(highPrioRes))
	require.NoError(t, err)

	// 1 = success, 0 = failure.
	require.Equal(t, highPrioReceipt.Status, uint64(1))
	require.Equal(t, lowPrioReceipt.Status, uint64(0))
}

// todo: check that the other nodes dont have this tx. check ethtxpool.
func TestNonceGappedTxsPass(t *testing.T) {
	sut := systemtests.Sut
	sut.ResetChain(t)
	StartChain(t, sut)

	sut.AwaitNBlocks(t, 10)

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

	outOfOrderNonces := []uint64{4, 2, 5, 3}
	responses := make([][]byte, len(outOfOrderNonces))

	for i, nonce := range outOfOrderNonces {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res1, err1 := exec.Command(
				"cast", "send",
				contractAddr,
				"increment()",
				"--rpc-url", "http://127.0.0.1:8545",
				"--private-key", pk,
				"--nonce", fmt.Sprintf("%d", nonce),
			).CombinedOutput()
			require.NoError(t, err1, "response: %s", string(res1))
			responses[i] = res1
		}()
	}

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
	require.NoError(t, err, "response: %s", string(res))

	wg.Wait()

	receipt, err := parseReceipt(string(res))
	require.NoError(t, err)
	require.Equal(t, receipt.Status, uint64(1))

	for _, bz := range responses {
		receipt, err := parseReceipt(string(bz))
		require.NoError(t, err)
		require.Equal(t, receipt.Status, uint64(1))
	}
}

func TestSimpleSendsScript(t *testing.T) {
	sut := systemtests.Sut
	sut.ResetChain(t)
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
	require.Contains(t, output, "Script ran successfully.")

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
