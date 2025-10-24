//go:build system_test

package chainupgrade

import (
	"fmt"
	"testing"
	"time"

	systest "cosmossdk.io/systemtests"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
	"github.com/cosmos/evm/tests/systemtests/suite"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const (
	upgradeHeight int64 = 12
	upgradeName         = "v0.4.0-to-v0.5.0" // must match UpgradeName in evmd/upgrades.go
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

	legacyBinary := systest.WorkDir + "/binaries/v0.4/evmd"
	sut.SetExecBinary(legacyBinary)
	sut.SetTestnetInitializer(systest.InitializerWithBinary(legacyBinary, sut))
	sut.SetupChain()

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
	sut.StartChain(t, "--chain-id=local-4221", "--mempool.max-txs=0")

	require.Equal(t, upgradeHeight+1, sut.CurrentHeight())

	// smoke test to make sure the chain still functions.
	cli = systest.NewCLIWrapper(t, sut, systest.Verbose)
	to := cli.GetKeyAddr("node1")
	from := cli.GetKeyAddr("node0")
	got := cli.Run("tx", "bank", "send", from, to, "1atest", "--from=node0", "--fees=10000000000000000000atest", "--chain-id=local-4221")
	systest.RequireTxSuccess(t, got)
}
