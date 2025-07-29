package systemtests

import (
	"encoding/hex"
	"math/big"
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
	t.Skip("nonce gaps are not yet supported")
	pk := "0x88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305"

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
		require.NoError(t, gappedErr, "response: %s", string(gappedRes))
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
	require.NoError(t, err, "response: %s", string(res))

	wg.Wait()

	gappedReceipt, err := parseReceipt(string(gappedRes))
	require.NoError(t, err)

	receipt, err := parseReceipt(string(res))
	require.NoError(t, err)

	// the gapped tx should be the regular receipt + 1 if they were in the same block.
	if gappedReceipt.BlockNumber == receipt.BlockNumber {
		require.Equal(t, gappedReceipt.TransactionIndex, receipt.TransactionIndex+1, "gapped tx: %d ---- other tx: %d ----", gappedReceipt.TransactionIndex, receipt.TransactionIndex)
	}
	require.Equal(t, gappedReceipt.Status, uint64(1))
	require.Equal(t, receipt.Status, uint64(1))
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
