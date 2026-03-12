//go:build system_test

package apphash

import (
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"

	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/sjson"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/tests/systemtests/clients"
	"github.com/cosmos/evm/tests/systemtests/suite"

	systest "github.com/cosmos/cosmos-sdk/testutil/systemtests"
)

const (
	bankReproEnv          = "EVM_RUN_BANK_APPHASH_REPRO"
	bankReproDenom        = "atest"
	bankReproBatches      = 5000
	bankReproTxsPerSender = 3
	bankReproSenderCount  = 50
	bankReproSendAmount   = 100 // same amount as EVM test
	bankReproBlockWait    = 30 * time.Second
)

// cosmosEphemeralSender is a locally-generated cosmos account.
type cosmosEphemeralSender struct {
	id      string
	privKey *ethsecp256k1.PrivKey
	addr    sdk.AccAddress
	accNum  uint64
}

// RunLiveBankSendsAppHash is like RunLiveHotSendsAppHash but uses
// cosmos-native MsgSend (bank module) instead of EVM value transfers.
// This isolates the EVM statedb layer: if this test also diverges under
// BlockSTM, the bug is in the bank module or BlockSTM's conflict tracking
// of bank store keys, not the EVM statedb.
func RunLiveBankSendsAppHash(t *testing.T, base *suite.BaseTestSuite) {
	if os.Getenv(bankReproEnv) != "1" {
		t.Skipf("set %s=1 to run the bank apphash reproducer", bankReproEnv)
	}

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

	// Patch per-node config files before starting the chain.
	for i := 0; i < 4; i++ {
		nodeDir := fmt.Sprintf("testnet/node%d/evmd/config", i)

		appToml := filepath.Join(systest.WorkDir, nodeDir, "app.toml")
		data, err := os.ReadFile(appToml)
		require.NoErrorf(t, err, "reading app.toml for node%d", i)
		s := string(data)
		s = strings.Replace(s, "global-slots = 5120", "global-slots = 50000", 1)
		s = strings.Replace(s, "global-queue = 1024", "global-queue = 10000", 1)
		require.NoError(t, os.WriteFile(appToml, []byte(s), 0o600))

		configToml := filepath.Join(systest.WorkDir, nodeDir, "config.toml")
		data, err = os.ReadFile(configToml)
		require.NoErrorf(t, err, "reading config.toml for node%d", i)
		s = string(data)
		s = strings.Replace(s, `timeout_commit = "2.7s"`, `timeout_commit = "500ms"`, 1)
		require.NoError(t, os.WriteFile(configToml, []byte(s), 0o600))

		t.Logf("patched node%d: global-slots=50000 global-queue=10000 timeout_commit=500ms", i)
	}

	base.StartChain(t, nodeArgs...)
	base.UnlockChain()

	base.AwaitNBlocks(t, 2)

	lastCommonHeight, statusByNode := waitForCommonHeight(t, base, 2, bankReproBlockWait)
	t.Logf("starting bank apphash reproducer at common height=%d statuses=%s", lastCommonHeight, formatStatuses(statusByNode))
	gasPrice := initialGasPrice(t, base)

	// Generate ephemeral senders with cosmos keys and fund them via EVM.
	senders := generateCosmosSenders(t, bankReproSenderCount)
	fundCosmosSendersViaEVM(t, base, senders, gasPrice)

	// Wait for funding txs to confirm.
	t.Logf("waiting for funding txs to confirm...")
	lastSender := senders[len(senders)-1]
	require.Eventually(t, func() bool {
		bal, err := base.CosmosClient.GetBalance(base.Node(0), lastSender.addr, bankReproDenom)
		return err == nil && bal.Sign() > 0
	}, bankReproBlockWait, 500*time.Millisecond, "funding txs did not confirm in time")

	lastCommonHeight, statusByNode = waitForCommonHeight(t, base, lastCommonHeight+1, bankReproBlockWait)
	t.Logf("funded %d cosmos senders, height=%d statuses=%s", len(senders), lastCommonHeight, formatStatuses(statusByNode))

	// Query account numbers from chain (needed for signing cosmos txs).
	queryCosmosSenderAccNumbers(t, base, senders)

	// Recipient is acc3 — same as EVM test.
	recipient := base.CosmosAccount("acc3").AccAddress
	nonces := make(map[string]uint64, len(senders))

	nodes := base.Nodes()
	rpcNodes := nodes[1:]
	sendAmount := sdkmath.NewInt(bankReproSendAmount)

	for batch := 0; batch < bankReproBatches; batch++ {
		var batchSent, batchSkipped int
		for i := 0; i < bankReproTxsPerSender; i++ {
			for si, sender := range senders {
				nonce := nonces[sender.id]
				targetNode := rpcNodes[si%len(rpcNodes)]

				_, err := base.CosmosClient.BankSend(
					targetNode,
					&clients.CosmosAccount{
						AccAddress:    sender.addr,
						AccountNumber: sender.accNum,
						PrivKey:       sender.privKey,
					},
					sender.addr,
					recipient,
					sendAmount,
					nonce,
					gasPrice,
				)
				if err != nil {
					batchSkipped++
					continue
				}
				nonces[sender.id] = nonce + 1
				batchSent++
			}
		}

		targetHeight := lastCommonHeight + 1
		newCommonHeight, statusByNode := waitForCommonHeight(t, base, targetHeight, bankReproBlockWait)
		lastCommonHeight = newCommonHeight

		if mismatch := checkAppHashAtHeight(t, base, newCommonHeight); mismatch != "" {
			exportGenesisOnDivergence(t, base, newCommonHeight)
			diag := dumpDiagnostics(t, base, newCommonHeight)
			t.Fatalf("apphash mismatch at height=%d: %s\n%s", newCommonHeight, mismatch, diag)
		}

		for _, nodeID := range base.Nodes() {
			logPath := filepath.Join(systest.WorkDir, "testnet", nodeID+".out")
			data, err := os.ReadFile(logPath)
			if err == nil && strings.Contains(string(data), "CONSENSUS FAILURE") {
				exportGenesisOnDivergence(t, base, newCommonHeight)
				diag := dumpDiagnostics(t, base, newCommonHeight)
				t.Fatalf("CONSENSUS FAILURE detected on %s at batch=%d height=%d\nlog: %s\n%s",
					nodeID, batch, newCommonHeight, string(data), diag)
			}
		}

		if batch%10 == 0 {
			t.Logf("batch=%d height=%d sent=%d skipped=%d apphash=%s", batch, newCommonHeight, batchSent, batchSkipped, statusByNode[base.Node(0)].AppHash)
		}
	}

	finalStatuses := getStatuses(t, base)
	t.Logf("completed bank apphash reproducer without mismatch: %s", formatStatuses(finalStatuses))
}

func generateCosmosSenders(t *testing.T, count int) []*cosmosEphemeralSender {
	t.Helper()
	senders := make([]*cosmosEphemeralSender, count)
	for i := 0; i < count; i++ {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		privKey := &ethsecp256k1.PrivKey{Key: crypto.FromECDSA(key)}
		addr := sdk.AccAddress(privKey.PubKey().Address().Bytes())
		senders[i] = &cosmosEphemeralSender{
			id:      fmt.Sprintf("ceph%d", i),
			privKey: privKey,
			addr:    addr,
		}
	}
	return senders
}

// fundCosmosSendersViaEVM funds the cosmos senders using EVM transfers (same mechanism as EVM test).
func fundCosmosSendersViaEVM(t *testing.T, base *suite.BaseTestSuite, senders []*cosmosEphemeralSender, gasPrice *big.Int) {
	t.Helper()

	funders := []*suite.TestAccount{base.Acc(0), base.Acc(1), base.Acc(2)}
	funderNonces := make([]uint64, len(funders))
	for i, f := range funders {
		nonce, err := base.NonceAt(base.Node(0), f.ID)
		require.NoError(t, err)
		funderNonces[i] = nonce
	}

	fundAmt := new(big.Int).SetUint64(liveReproFundAmount) // 10 ETH per sender
	gasLimit := uint64(21_000)

	for i, sender := range senders {
		fIdx := i % len(funders)
		funder := funders[fIdx]
		nonce := funderNonces[fIdx]
		funderNonces[fIdx]++

		// Convert cosmos address to eth address for EVM transfer.
		ecdsaKey, err := sender.privKey.ToECDSA()
		require.NoError(t, err)
		ethAddr := crypto.PubkeyToAddress(*ecdsaKey.Public().(*ecdsa.PublicKey))

		ethtx := ethtypes.NewTransaction(nonce, ethAddr, fundAmt, gasLimit, gasPrice, nil)
		_, err = base.EthClient.SendRawTransaction(base.Node(0), funder.Eth, ethtx)
		require.NoErrorf(t, err, "fund %s from %s nonce=%d", sender.id, funder.ID, nonce)
	}
}

// queryCosmosSenderAccNumbers queries the on-chain account number for each sender.
func queryCosmosSenderAccNumbers(t *testing.T, base *suite.BaseTestSuite, senders []*cosmosEphemeralSender) {
	t.Helper()
	clientCtx := base.CosmosClient.ClientCtx.WithClient(base.CosmosClient.RpcClients[base.Node(0)])
	for _, sender := range senders {
		accInfo, err := clientCtx.AccountRetriever.GetAccount(clientCtx, sender.addr)
		require.NoErrorf(t, err, "query account %s (%s)", sender.id, sender.addr)
		sender.accNum = accInfo.GetAccountNumber()
	}
	t.Logf("queried account numbers for %d cosmos senders", len(senders))
}
