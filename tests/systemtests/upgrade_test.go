//go:build system_test

package systemtests

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	systest "cosmossdk.io/systemtests"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/address"
)

const (
	upgradeHeight int64 = 22
	upgradeName         = "v0.4.0-to-v0.5.0" // must match UpgradeName in simapp/upgrades.go
)

func TestChainUpgrade(t *testing.T) {
	// Scenario:
	// start a legacy chain with some state
	// when a chain upgrade proposal is executed
	// then the chain upgrades successfully
	systest.Sut.StopChain()

	currentBranchBinary := systest.Sut.ExecBinary()
	currentInitializer := systest.Sut.TestnetInitializer()

	legacyBinary := systest.WorkDir + "/binaries/v0.4/evmd"
	systest.Sut.SetExecBinary(legacyBinary)
	systest.Sut.SetTestnetInitializer(systest.InitializerWithBinary(legacyBinary, systest.Sut))
	systest.Sut.SetupChain()

	votingPeriod := 5 * time.Second // enough time to vote
	systest.Sut.ModifyGenesisJSON(t, systest.SetGovVotingPeriod(t, votingPeriod))

	systest.Sut.StartChain(t, fmt.Sprintf("--halt-height=%d", upgradeHeight+1), "--chain-id=local-4221", "--minimum-gas-prices=0.00atest")

	cli := systest.NewCLIWrapper(t, systest.Sut, systest.Verbose)
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

	for i := range 4 {
		go func(i int) { // do parallel
			systest.Sut.Logf("Voting: validator %d\n", i)
			rsp := cli.Run("tx", "gov", "vote", proposalID, "yes", "--fees=10000000000000000000atest", "--from", cli.GetKeyAddr(fmt.Sprintf("node%d", i)))
			systest.RequireTxSuccess(t, rsp)
		}(i)
	}

	systest.Sut.AwaitBlockHeight(t, upgradeHeight-1, 60*time.Second)
	t.Logf("current_height: %d\n", systest.Sut.CurrentHeight())
	raw = cli.CustomQuery("q", "gov", "proposal", proposalID)
	proposalStatus := gjson.Get(raw, "proposal.status").String()
	require.Equal(t, "PROPOSAL_STATUS_PASSED", proposalStatus, raw)

	t.Log("waiting for upgrade info")
	systest.Sut.AwaitUpgradeInfo(t)
	systest.Sut.StopChain()

	t.Log("Upgrade height was reached. Upgrading chain")
	systest.Sut.SetExecBinary(currentBranchBinary)
	systest.Sut.SetTestnetInitializer(currentInitializer)
	systest.Sut.StartChain(t, "--chain-id=local-4221")

	require.Equal(t, upgradeHeight+1, systest.Sut.CurrentHeight())

	// smoke test to make sure the chain still functions.
	cli = systest.NewCLIWrapper(t, systest.Sut, systest.Verbose)
	to := cli.GetKeyAddr("node1")
	from := cli.GetKeyAddr("node0")
	got := cli.Run("tx", "bank", "send", from, to, "1atest", "--from=node0", "--fees=10000000000000000000atest", "--chain-id=local-4221")
	systest.RequireTxSuccess(t, got)
}
