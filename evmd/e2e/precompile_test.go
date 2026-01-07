package e2e

import (
	"context"
	"math/big"
	"testing"
	"time"

	evmcontracts "github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/evmd/e2e/fixtures"
	"github.com/cosmos/evm/evmd/e2e/testharness"
	"github.com/cosmos/evm/evmd/e2e/utils"
	bankpc "github.com/cosmos/evm/precompiles/bank"
	bech32pc "github.com/cosmos/evm/precompiles/bech32"
	distributionpc "github.com/cosmos/evm/precompiles/distribution"
	ics02pc "github.com/cosmos/evm/precompiles/ics02"
	stakingpc "github.com/cosmos/evm/precompiles/staking"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/types/bech32"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"
)

// TestBankPrecompile verifies EVM -> bech32 flow by:
// - deriving a bech32 recipient from a fresh EVM address
// - querying bank precompile balances before and after
// - transferring 1e18 astake via the native ERC-20 mapping (WERC20)
// - asserting the bank balance delta equals 1e18 and receipt.status=1
func TestBankPrecompile(t *testing.T) {
	t.Skip("Skipping bank precompile test (since it is not currently integrated)")
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	const (
		amountWeiStr = "1000000000000000000" // 1e18
	)

	bankPC := common.HexToAddress(utils.BankPrecompileAddr)
	bech32PC := common.HexToAddress(utils.Bech32PrecompileAddr)
	werc20 := common.HexToAddress(utils.WERC20Addr)
	amount, ok := new(big.Int).SetString(amountWeiStr, 10)
	req.Truef(ok, "invalid bank amount constant %s", amountWeiStr)

	// Load ABIs from the evm module without changing that repo
	bankP := bankpc.NewPrecompile(nil, nil)
	bankABI := bankP.ABI

	bech32P, err := bech32pc.NewPrecompile(1) // any non-zero base gas
	req.NoError(err)
	bech32ABI := bech32P.ABI

	// Use the compiled ERC20 contract ABI from evm/contracts
	erc20ABI := evmcontracts.ERC20MinterBurnerDecimalsContract.ABI

	// Create a fresh EVM recipient and derive its bech32 form
	recvKey, err := crypto.GenerateKey()
	req.NoError(err)
	recipient := crypto.PubkeyToAddress(recvKey.PublicKey)
	recipientBech32, err := bech32.ConvertAndEncode(utils.TestBech32Prefix, recipient.Bytes())
	req.NoError(err)
	req.NotEmpty(recipientBech32)

	// Sanity: bech32 -> hex via precompile
	inB2H, err := bech32ABI.Pack("bech32ToHex", recipientBech32)
	req.NoError(err)
	resB2H, err := chain.EthClient.CallContract(harness.Ctx, ethereum.CallMsg{From: harness.SenderAddr, To: &bech32PC, Data: inB2H}, nil)
	req.NoError(err)
	outsB2H, err := bech32ABI.Unpack("bech32ToHex", resB2H)
	req.NoError(err)
	req.Len(outsB2H, 1)
	gotAddr, ok := outsB2H[0].(common.Address)
	req.True(ok)
	req.Equal(recipient, gotAddr)

	// Helper types and finder
	type balance struct {
		ContractAddress common.Address
		Amount          *big.Int
	}
	findAmt := func(list []balance, token common.Address) *big.Int {
		for _, b := range list {
			if b.ContractAddress == token {
				return new(big.Int).Set(b.Amount)
			}
		}
		return big.NewInt(0)
	}

	// Pre-transfer balances(recipient)
	callBalances, err := bankABI.Pack("balances", recipient)
	req.NoError(err)
	outPre, err := chain.EthClient.CallContract(harness.Ctx, ethereum.CallMsg{From: harness.SenderAddr, To: &bankPC, Data: callBalances}, nil)
	req.NoError(err)
	var pre []balance
	err = bankABI.UnpackIntoInterface(&pre, "balances", outPre)
	req.NoError(err)
	preAmt := findAmt(pre, werc20)

	// Transfer 1e18 astake via WERC20 to recipient
	callTransfer, err := erc20ABI.Pack("transfer", recipient, amount)
	req.NoError(err)
	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &werc20, big.NewInt(0), callTransfer, 0)
	req.NoError(err)
	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)
	t.Logf("bank_precompile transfer tx=%s gasUsed=%d", txHash.Hex(), rec.GasUsed)

	// Post-transfer balances(recipient)
	outPost, err := chain.EthClient.CallContract(harness.Ctx, ethereum.CallMsg{From: harness.SenderAddr, To: &bankPC, Data: callBalances}, nil)
	req.NoError(err)
	var post []balance
	err = bankABI.UnpackIntoInterface(&post, "balances", outPost)
	req.NoError(err)
	postAmt := findAmt(post, werc20)
	delta := new(big.Int).Sub(postAmt, preAmt)
	req.Equalf(0, delta.Cmp(amount), "recipient bank delta %s != %s", delta.String(), amount.String())
}

// TestStakingPrecompile delegates and undelegates via the staking precompile, verifying delegation shares
// increase on delegate and return to baseline after the unbonding period completes.
func TestStakingPrecompile(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain
	const (
		stakingPrecompileAddr = "0x0000000000000000000000000000000000000800"
		amountWeiStr          = "1000000000000000000" // 1e18
	)

	stakingAddr := common.HexToAddress(stakingPrecompileAddr)
	amount, ok := new(big.Int).SetString(amountWeiStr, 10)
	req.Truef(ok, "invalid staking amount constant %s", amountWeiStr)

	// Load staking ABI from the evm module
	parsed := stakingpc.ABI

	// Discover bonded validator operator address via evmd CLI inside the validator container
	queryClient := stakingtypes.NewQueryClient(chain.GrpcClient)
	vr, err := queryClient.Validators(harness.Ctx, &stakingtypes.QueryValidatorsRequest{})
	req.NoError(err)
	req.NotEmpty(vr.Validators, "no validators found")
	valoper := vr.Validators[0].OperatorAddress
	req.NotEmpty(valoper)

	// Helper: query current delegation shares for (delegator, validator)
	getShares := func() *big.Int {
		in, err := parsed.Pack("delegation", harness.SenderAddr, valoper)
		req.NoError(err)
		out, err := chain.EthClient.CallContract(harness.Ctx, ethereum.CallMsg{From: harness.SenderAddr, To: &stakingAddr, Data: in}, nil)
		req.NoError(err)
		vals, err := parsed.Unpack("delegation", out)
		req.NoError(err)
		req.Len(vals, 2)
		// first return value is shares (uint256)
		return new(big.Int).Set(vals[0].(*big.Int))
	}

	shares0 := getShares()

	// Delegate amount to validator
	callDelegate, err := parsed.Pack("delegate", harness.SenderAddr, valoper, amount)
	req.NoError(err)
	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &stakingAddr, big.NewInt(0), callDelegate, 0)
	req.NoError(err)
	rec1, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.StakingReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec1)
	req.Equal(uint64(1), rec1.Status)
	t.Logf("delegate tx=%s gasUsed=%d", txHash.Hex(), rec1.GasUsed)

	// Verify shares increased by exactly amount, scaled to staking share precision (1e18)
	shares1 := getShares()
	delta := new(big.Int).Sub(shares1, shares0)
	dec1e18 := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	expectedSharesDelta := new(big.Int).Mul(amount, dec1e18)
	req.Equalf(0, delta.Cmp(expectedSharesDelta), "shares delta %s != %s", delta.String(), expectedSharesDelta.String())

	// Undelegate the same amount
	callUndelegate, err := parsed.Pack("undelegate", harness.SenderAddr, valoper, amount)
	req.NoError(err)
	txHash2, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &stakingAddr, big.NewInt(0), callUndelegate, 0)
	req.NoError(err)
	rec2, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash2, utils.StakingReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec2)
	req.Equal(uint64(1), rec2.Status)
	t.Logf("undelegate tx=%s gasUsed=%d", txHash2.Hex(), rec2.GasUsed)

	// Wait for unbonding completion (genesis set to 10s); give buffer and a couple of blocks
	time.Sleep(utils.UnbondingWaitTime)
	req.NoError(utils.WaitForBlocks(harness.Ctx, chain.EthClient, 2))

	// Verify shares returned to baseline
	shares2 := getShares()
	req.Equalf(0, shares2.Cmp(shares0), "shares after unbonding %s != baseline %s", shares2.String(), shares0.String())
}

func TestICS02Precompile(t *testing.T) {
	ctx := context.Background()

	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	const (
		ics02PrecompileAddr = "0x0000000000000000000000000000000000000807"
	)

	// Parse fixture
	fixture, err := fixtures.LoadUpdateClientFixture("fixtures/update_client.json")
	req.NoError(err)

	clientStateAny, err := fixture.ClientStateAny()
	req.NoError(err)
	consensusStateAny, err := fixture.ConsensusStateAny()
	req.NoError(err)

	_, err = chain.BroadcastSdkMgs(ctx, harness.SenderKey, 10_000_000, &clienttypes.MsgCreateClient{
		ClientState:    clientStateAny,
		ConsensusState: consensusStateAny,
		Signer:         harness.SenderBech32,
	})
	req.NoError(err)

	// 11) Wait for a few blocks
	time.Sleep(3 * time.Second)

	// GetClientState Call
	clientID := ibctesting.FirstClientID
	ics02Addr := common.HexToAddress(ics02PrecompileAddr)
	calldata, err := ics02pc.ABI.Pack(ics02pc.GetClientStateMethod, clientID)
	req.NoError(err)

	// Call ICS02 precompile to get client state
	out, err := chain.EthClient.CallContract(ctx, ethereum.CallMsg{From: harness.SenderAddr, To: &ics02Addr, Data: calldata}, nil)
	req.NoError(err)
	req.NotEmpty(out)

	// Prepare ICS02 precompile call data
	updateHeaderAny, err := fixture.UpdateClientMessageAny()
	req.NoError(err)
	updateBz, err := updateHeaderAny.Marshal()
	req.NoError(err)

	calldata, err = ics02pc.ABI.Pack(ics02pc.UpdateClientMethod, clientID, updateBz)
	req.NoError(err)

	// Send transaction to ICS02 precompile
	txHash, err := utils.SendTx(ctx, chain.EthClient, harness.SenderKey, &ics02Addr, big.NewInt(0), calldata, 0)
	req.NoError(err)
	rec, err := utils.WaitReceipt(ctx, chain.EthClient, txHash, utils.ICS02ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)
	t.Logf("ics02 update_client tx=%s gasUsed=%d", txHash.Hex(), rec.GasUsed)
}

// TestDistributionPrecompile tests delegating tokens and claiming rewards via the distribution precompile.
// This test verifies that:
// - A user can delegate tokens to a validator via the staking precompile
// - Rewards accumulate over time
// - The user can query their pending rewards
// - The user can successfully claim rewards via the distribution precompile
func TestDistributionPrecompile(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	const (
		stakingPrecompileAddr      = "0x0000000000000000000000000000000000000800"
		distributionPrecompileAddr = "0x0000000000000000000000000000000000000801"
		amountWeiStr               = "1000000000000000000" // 1e18
	)

	stakingAddr := common.HexToAddress(stakingPrecompileAddr)
	distributionAddr := common.HexToAddress(distributionPrecompileAddr)
	amount, ok := new(big.Int).SetString(amountWeiStr, 10)
	req.Truef(ok, "invalid staking amount constant %s", amountWeiStr)

	// Load ABIs
	stakingABI := stakingpc.ABI
	distributionABI := distributionpc.ABI

	// Get a bonded validator
	queryClient := stakingtypes.NewQueryClient(chain.GrpcClient)
	vr, err := queryClient.Validators(harness.Ctx, &stakingtypes.QueryValidatorsRequest{})
	req.NoError(err)
	req.NotEmpty(vr.Validators, "no validators found")
	valoper := vr.Validators[0].OperatorAddress
	req.NotEmpty(valoper)

	// Step 1: Delegate tokens to the validator
	callDelegate, err := stakingABI.Pack("delegate", harness.SenderAddr, valoper, amount)
	req.NoError(err)
	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &stakingAddr, big.NewInt(0), callDelegate, 0)
	req.NoError(err)
	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.StakingReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)
	t.Logf("delegate tx=%s gasUsed=%d", txHash.Hex(), rec.GasUsed)

	// Step 2: Wait for some blocks to accumulate rewards
	// In a real chain, rewards accumulate each block
	time.Sleep(5 * time.Second)
	req.NoError(utils.WaitForBlocks(harness.Ctx, chain.EthClient, 5))

	// Step 3: Query delegation total rewards before claiming
	callTotalRewards, err := distributionABI.Pack("delegationTotalRewards", harness.SenderAddr)
	req.NoError(err)
	rewardsOut, err := chain.EthClient.CallContract(harness.Ctx, ethereum.CallMsg{
		From: harness.SenderAddr,
		To:   &distributionAddr,
		Data: callTotalRewards,
	}, nil)
	req.NoError(err)
	req.NotEmpty(rewardsOut, "expected non-empty rewards response")

	// Unpack and validate rewards are greater than 0
	rewardsVals, err := distributionABI.Unpack("delegationTotalRewards", rewardsOut)
	req.NoError(err)
	req.NotEmpty(rewardsVals, "expected rewards values")
	t.Logf("Rewards response: %+v", rewardsVals)

	// The response returns [rewards_per_validator, total_rewards]
	// where total_rewards is the second element - an array of {Denom, Amount, Precision}
	req.GreaterOrEqual(len(rewardsVals), 2, "expected at least 2 return values from delegationTotalRewards")

	// Get the total rewards (second return value)
	totalRewards, ok := rewardsVals[1].([]struct {
		Denom     string   `json:"denom"`
		Amount    *big.Int `json:"amount"`
		Precision uint8    `json:"precision"`
	})
	req.True(ok, "expected total rewards to be array of coins, got %T", rewardsVals[1])
	req.NotEmpty(totalRewards, "expected at least one total reward")

	// Validate all reward amounts are > 0
	for _, reward := range totalRewards {
		req.Positive(reward.Amount.Cmp(big.NewInt(0)),
			"reward for denom %s should be greater than 0, got %s",
			reward.Denom, reward.Amount.String())
		t.Logf("Total reward for %s: %s (precision: %d)", reward.Denom, reward.Amount.String(), reward.Precision)
	}

	// Step 4: Claim rewards from all validators (maxRetrieve=1 since we only delegated to one)
	callClaimRewards, err := distributionABI.Pack("claimRewards", harness.SenderAddr, uint32(1))
	req.NoError(err)
	claimTxHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &distributionAddr, big.NewInt(0), callClaimRewards, 0)
	req.NoError(err)
	claimRec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, claimTxHash, utils.StakingReceiptTimeout)
	req.NoError(err)
	req.NotNil(claimRec)
	req.Equal(uint64(1), claimRec.Status, "claim rewards transaction should succeed")
	t.Logf("claim_rewards tx=%s gasUsed=%d status=%d", claimTxHash.Hex(), claimRec.GasUsed, claimRec.Status)
	t.Logf("Successfully claimed rewards from validator %s", valoper)
}
