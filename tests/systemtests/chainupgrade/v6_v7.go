//go:build system_test

package chainupgrade

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/cosmos/evm/tests/systemtests/suite"

	systest "github.com/cosmos/cosmos-sdk/tools/systemtests"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
)

const (
	upgradeHeight int64 = 22
	upgradeName         = "v0.6.0-to-v0.7.0" // must match UpgradeName in evmd/upgrades.go

	// Contended-account workload — multiple senders, single recipient.
	// SendEthLegacyTx hard-codes the recipient to acc3 and the value to 1000 wei,
	// so any change to that helper means changing these constants too.
	contendedSenders   = 3 // acc0, acc1, acc2
	contendedTxsPerEOA = 4
	contendedRecipient = "acc3"
	contendedTxValue   = int64(1000)
)

// RunChainUpgrade exercises an on-chain software upgrade using the injected shared suite.
func RunChainUpgrade(t *testing.T, base *suite.BaseTestSuite) {
	t.Helper()

	base.SetupTest(t)
	sut := base.SystemUnderTest

	// Scenario:
	// start a legacy chain with some state
	// when a chain upgrade proposal is executed
	// then the chain upgrades successfully
	sut.StopChain()

	currentBranchBinary := sut.ExecBinary()
	currentInitializer := sut.TestnetInitializer()

	legacyBinary := systest.WorkDir + "/binaries/v0.6/evmd"
	systest.Sut.SetExecBinary(legacyBinary)
	systest.Sut.SetTestnetInitializer(systest.InitializerWithBinary(legacyBinary, systest.Sut))
	systest.Sut.SetupChain()

	votingPeriod := 5 * time.Second // enough time to vote
	sut.ModifyGenesisJSON(t, systest.SetGovVotingPeriod(t, votingPeriod))

	sut.StartChain(t, fmt.Sprintf("--halt-height=%d", upgradeHeight+1), "--chain-id=local-4221", "--minimum-gas-prices=0.00atest")

	cli := systest.NewCLIWrapper(t, sut, systest.Verbose)
	govAddr := sdk.AccAddress(address.Module("gov")).String()
	// submit upgrade proposal
	proposal := fmt.Sprintf(`
{
 "messages": [
  {
   "@type": "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
   "authority": %q,
   "plan": {
    "name": %q,
    "height": "%d"
   }
  }
 ],
 "metadata": "ipfs://CID",
 "deposit": "100000000stake",
 "title": "my upgrade",
 "summary": "testing"
}`, govAddr, upgradeName, upgradeHeight)
	rsp := cli.SubmitGovProposal(proposal, "--fees=10000000000000000000atest", "--from=node0")
	systest.RequireTxSuccess(t, rsp)
	raw := cli.CustomQuery("q", "gov", "proposals", "--depositor", cli.GetKeyAddr("node0"))
	proposals := gjson.Get(raw, "proposals.#.id").Array()
	require.NotEmpty(t, proposals, raw)
	proposalID := proposals[len(proposals)-1].String()

	for i := range sut.NodesCount() {
		go func(i int) { // do parallel
			sut.Logf("Voting: validator %d\n", i)
			rsp := cli.Run("tx", "gov", "vote", proposalID, "yes", "--fees=10000000000000000000atest", "--from", cli.GetKeyAddr(fmt.Sprintf("node%d", i)))
			systest.RequireTxSuccess(t, rsp)
		}(i)
	}

	sut.AwaitBlockHeight(t, upgradeHeight-1, 60*time.Second)
	t.Logf("current_height: %d\n", sut.CurrentHeight())
	raw = cli.CustomQuery("q", "gov", "proposal", proposalID)
	proposalStatus := gjson.Get(raw, "proposal.status").String()
	require.Equal(t, "PROPOSAL_STATUS_PASSED", proposalStatus, raw)

	t.Log("waiting for upgrade info")
	sut.AwaitUpgradeInfo(t)
	sut.StopChain()

	t.Log("Upgrade height was reached. Upgrading chain")
	sut.SetExecBinary(currentBranchBinary)
	sut.SetTestnetInitializer(currentInitializer)
	// Keep Comet and app mempool settings in sync for the upgraded binary startup.
	base.ModifyCometMempool(t, "app")
	sut.StartChain(t, "--chain-id=local-4221", "--mempool.max-txs=0")

	require.Equal(t, upgradeHeight+1, sut.CurrentHeight())

	// smoke test to make sure the chain still functions.
	cli = systest.NewCLIWrapper(t, sut, systest.Verbose)
	to := cli.GetKeyAddr("node1")
	from := cli.GetKeyAddr("node0")
	got := cli.Run("tx", "bank", "send", from, to, "1atest", "--from=node0", "--fees=10000000000000000000atest", "--chain-id=local-4221")
	systest.RequireTxSuccess(t, got)

	runContendedAccountWorkload(t, base)
}

// runContendedAccountWorkload exercises BlockSTM's conflict detection across
// the upgrade boundary. Multiple senders fire EVM txs concurrently to a single
// recipient so every tx in a block writes to the same balance slot — the
// classic contended-account scenario. A missed scheduler conflict would
// manifest as a lost write (final balance below the expected delta).
func runContendedAccountWorkload(t *testing.T, base *suite.BaseTestSuite) {
	t.Helper()

	sut := base.SystemUnderTest
	nodeID := base.Node(0)

	baseFee, err := base.GetLatestBaseFee(nodeID)
	require.NoError(t, err, "fetch base fee")
	base.SetBaseFee(baseFee)
	gasPrice := base.GasPriceMultiplier(10)

	recipient := base.EthAccount(contendedRecipient)
	balCtx, cancelBal := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelBal()
	startBal, err := base.EthClient.Clients[nodeID].BalanceAt(balCtx, recipient.Address, nil)
	require.NoError(t, err, "fetch starting recipient balance")

	type sendResult struct {
		hash string
		err  error
	}
	resultsCh := make(chan sendResult, contendedSenders*contendedTxsPerEOA)

	var wg sync.WaitGroup
	for i := 0; i < contendedSenders; i++ {
		wg.Add(1)
		go func(senderIdx int) {
			defer wg.Done()
			signerID := base.AccID(senderIdx)
			// Fire txsPerEOA txs back-to-back from this EOA. Each call reads the
			// chain nonce and adds nonceIdx; while the txs sit in the mempool
			// the chain nonce hasn't advanced, so this produces a contiguous
			// nonce sequence per sender.
			for n := uint64(0); n < contendedTxsPerEOA; n++ {
				info, err := base.SendEthLegacyTx(t, nodeID, signerID, n, gasPrice)
				if err != nil {
					resultsCh <- sendResult{err: fmt.Errorf("sender %s nonce %d: %w", signerID, n, err)}
					return
				}
				resultsCh <- sendResult{hash: info.TxHash}
			}
		}(i)
	}
	wg.Wait()
	close(resultsCh)

	var hashes []string
	for r := range resultsCh {
		require.NoError(t, r.err)
		hashes = append(hashes, r.hash)
	}
	require.Len(t, hashes, contendedSenders*contendedTxsPerEOA)

	for _, h := range hashes {
		_, err := base.EthClient.WaitForCommit(nodeID, h, 60*time.Second)
		require.NoErrorf(t, err, "tx %s did not commit", h)
	}

	endCtx, cancelEnd := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelEnd()
	endBal, err := base.EthClient.Clients[nodeID].BalanceAt(endCtx, recipient.Address, nil)
	require.NoError(t, err, "fetch ending recipient balance")

	expectedDelta := big.NewInt(int64(contendedSenders*contendedTxsPerEOA) * contendedTxValue)
	expected := new(big.Int).Add(startBal, expectedDelta)
	require.Equalf(t, expected.String(), endBal.String(),
		"contended-account balance mismatch: start=%s end=%s expected=%s — possible BlockSTM lost write",
		startBal, endBal, expected)

	// Chain liveness: if any node forked, the chain would have halted by now.
	sut.AwaitNBlocks(t, 2)
}
