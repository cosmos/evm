package e2e

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/cosmos/evm/evmd/e2e/contracts"
	"github.com/cosmos/evm/evmd/e2e/testharness"
	"github.com/cosmos/evm/evmd/e2e/utils"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

// TestJSONRPCEstimateGasRevert verifies that eth_estimateGas returns an error
// containing revert data when the target call would revert, and that the revert
// reason decodes as expected.
func TestJSONRPCEstimateGasRevert(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	// Deploy Reverter
	creation := common.FromHex(contracts.ReverterBinHex())
	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, nil, big.NewInt(0), creation, 0)
	req.NoError(err)

	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)
	revAddr := rec.ContractAddress

	// Build call data that will revert with a known reason
	parsed, err := abi.JSON(strings.NewReader(contracts.ReverterABIJSON()))
	req.NoError(err)
	reason := "ESTIMATE_GAS_REVERT"
	callData, err := parsed.Pack("revertWithReason", reason)
	req.NoError(err)

	// eth_estimateGas should error and include revert data
	_, err = chain.EthClient.EstimateGas(harness.Ctx, ethereum.CallMsg{
		From: harness.SenderAddr,
		To:   &revAddr,
		Data: callData,
	})
	req.Error(err)

	// Extract revert data hex from the RPC error
	var hexData string
	var de rpc.DataError
	if errors.As(err, &de) {
		switch v := de.ErrorData().(type) {
		case string:
			hexData = v
		case map[string]any:
			// Common shapes:
			// {"data":"0x..."} OR {"data":{"data":"0x...", ...}}
			if d, ok := v["data"].(string); ok {
				hexData = d
			} else if inner, ok := v["data"].(map[string]any); ok {
				if d, ok := inner["data"].(string); ok {
					hexData = d
				}
			}
		}
	}
	req.NotEmpty(hexData, "missing revert data in eth_estimateGas error: %v", err)

	// Decode revert reason from the data
	b := common.FromHex(hexData)
	msg, derr := abi.UnpackRevert(b)
	req.NoError(derr)
	req.Equal(reason, msg)

	// Cross-check via eth_call helper against latest state (no specific block)
	rr, err := utils.GetRevertReasonViaEthCall(harness.Ctx, chain.RPC, revAddr, callData, nil)
	req.NoError(err)
	req.Equal(reason, rr)
}

// TestJSONRPCFilters covers eth_getLogs filtering semantics:
// - blockHash-only filter (with topic0) restricts results to that block and includes our log
// - range filter with address + topic OR across indexed caller includes both expected logs
func TestJSONRPCFilters(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	// Deploy Counter
	parsedABI, err := abi.JSON(strings.NewReader(contracts.CounterABIJSON()))
	req.NoError(err)
	creation := common.FromHex(contracts.CounterBinHex())

	txHashCreate, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, nil, big.NewInt(0), creation, 0)
	req.NoError(err)
	recCreate, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHashCreate, utils.ReceiptTimeout)
	req.NoError(err)
	req.Equal(uint64(1), recCreate.Status)
	counter := recCreate.ContractAddress

	// Second caller (B): fund with small amount for gas
	bKey, err := crypto.GenerateKey()
	req.NoError(err)
	bAddr := crypto.PubkeyToAddress(bKey.PublicKey)
	fundAmt := big.NewInt(1_000_000_000_000_000) // 1e15 wei
	txFund, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &bAddr, fundAmt, nil, utils.BasicTransferGas)
	req.NoError(err)
	recFund, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txFund, utils.ReceiptTimeout)
	req.NoError(err)
	req.Equal(uint64(1), recFund.Status)

	// Emit two events from two callers
	callSet1, err := parsedABI.Pack("set", big.NewInt(1))
	req.NoError(err)
	callSet2, err := parsedABI.Pack("set", big.NewInt(2))
	req.NoError(err)

	txA, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &counter, big.NewInt(0), callSet1, 0)
	req.NoError(err)
	recA, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txA, utils.ReceiptTimeout)
	req.NoError(err)
	req.Equal(uint64(1), recA.Status)

	txB, err := utils.SendTx(harness.Ctx, chain.EthClient, bKey, &counter, big.NewInt(0), callSet2, 0)
	req.NoError(err)
	recB, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txB, utils.ReceiptTimeout)
	req.NoError(err)
	req.Equal(uint64(1), recB.Status)

	// Topics
	topic0 := crypto.Keccak256Hash([]byte("ValueChanged(uint256,address)"))
	topicA := common.BytesToHash(common.LeftPadBytes(harness.SenderAddr.Bytes(), 32))
	topicB := common.BytesToHash(common.LeftPadBytes(bAddr.Bytes(), 32))

	// blockHash-only filter (plus topic0) for the block that included txA
	logs1, err := chain.EthClient.FilterLogs(harness.Ctx, ethereum.FilterQuery{BlockHash: &recA.BlockHash, Topics: [][]common.Hash{{topic0}}})
	req.NoError(err)
	req.NotEmpty(logs1)
	for _, lg := range logs1 {
		req.Equal(recA.BlockHash, lg.BlockHash)
	}
	// Ensure our txA event is present
	foundA := false
	for i := range logs1 {
		lg := logs1[i]
		if lg.Address == counter && lg.TxHash == recA.TxHash && len(lg.Topics) > 0 && lg.Topics[0] == topic0 {
			foundA = true
			break
		}
	}
	req.True(foundA, "expected txA event in blockHash-only filter results")

	// range + address + OR on indexed caller
	from := recA.BlockNumber
	to := recB.BlockNumber
	if to.Cmp(from) < 0 {
		from, to = to, from
	}
	logs2, err := chain.EthClient.FilterLogs(harness.Ctx, ethereum.FilterQuery{
		FromBlock: from,
		ToBlock:   to,
		Addresses: []common.Address{counter},
		Topics:    [][]common.Hash{{topic0}, {topicA, topicB}},
	})
	req.NoError(err)

	var seenA, seenB bool
	for i := range logs2 {
		lg := logs2[i]
		if lg.Address != counter || len(lg.Topics) < 2 || lg.Topics[0] != topic0 {
			continue
		}
		if lg.TxHash == recA.TxHash || lg.Topics[1] == topicA {
			seenA = true
		}
		if lg.TxHash == recB.TxHash || lg.Topics[1] == topicB {
			seenB = true
		}
	}
	req.True(seenA, "expected A's event present in range+OR filter")
	req.True(seenB, "expected B's event present in range+OR filter")
}

// TestJSONRPCNodeInfoAndFees validates node info & fee tip RPCs:
// - web3_clientVersion is non-empty
// - net_listening is true
// - net_peerCount is 0 or 1 (single-node harness)
// - eth_maxPriorityFeePerGas and SuggestGasTipCap are non-negative and roughly consistent
func TestJSONRPCNodeInfoAndFees(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	rpcClient, err := rpc.DialContext(harness.Ctx, chain.RPC)
	req.NoError(err)
	defer rpcClient.Close()

	// web3_clientVersion
	{
		cctx, cancel := context.WithTimeout(harness.Ctx, utils.RevertReasonTimeout)
		defer cancel()
		var clientVersion string
		req.NoError(rpcClient.CallContext(cctx, &clientVersion, "web3_clientVersion"))
		req.NotEmpty(clientVersion)
		t.Logf("web3_clientVersion: %s", clientVersion)
	}

	// net_listening
	{
		cctx, cancel := context.WithTimeout(harness.Ctx, utils.RevertReasonTimeout)
		defer cancel()
		var listening bool
		req.NoError(rpcClient.CallContext(cctx, &listening, "net_listening"))
		req.True(listening)
		t.Logf("net_listening: %v", listening)
	}

	// net_peerCount (accept hex string or number)
	peerCountInt := new(big.Int)
	{
		cctx, cancel := context.WithTimeout(harness.Ctx, utils.RevertReasonTimeout)
		defer cancel()
		var peerCountRaw any
		req.NoError(rpcClient.CallContext(cctx, &peerCountRaw, "net_peerCount"))
		switch v := peerCountRaw.(type) {
		case string:
			clean := strings.TrimPrefix(v, "0x")
			if clean == "" {
				clean = "0"
			}
			_, ok := peerCountInt.SetString(clean, 16)
			req.True(ok, "bad hex quantity: %q", v)
		case float64:
			if v < 0 {
				peerCountInt.SetInt64(0)
			} else {
				peerCountInt.SetInt64(int64(v))
			}
		default:
			req.Failf("unexpected type", "net_peerCount result has unsupported type %T", v)
		}
		// Expect 0 or 1 in a single-node harness
		zero := big.NewInt(0)
		one := big.NewInt(1)
		req.GreaterOrEqual(peerCountInt.Cmp(zero), 0)
		req.LessOrEqual(peerCountInt.Cmp(one), 0)
		t.Logf("net_peerCount: %v (%s)", peerCountRaw, peerCountInt.String())
	}

	// eth_maxPriorityFeePerGas (accept hex string or number)
	rpcTip := new(big.Int)
	{
		cctx, cancel := context.WithTimeout(harness.Ctx, utils.RevertReasonTimeout)
		defer cancel()
		var tipRaw any
		req.NoError(rpcClient.CallContext(cctx, &tipRaw, "eth_maxPriorityFeePerGas"))
		switch v := tipRaw.(type) {
		case string:
			clean := strings.TrimPrefix(v, "0x")
			if clean == "" {
				clean = "0"
			}
			_, ok := rpcTip.SetString(clean, 16)
			req.True(ok, "bad hex quantity: %q", v)
		case float64:
			if v < 0 {
				rpcTip.SetInt64(0)
			} else {
				rpcTip.SetInt64(int64(v))
			}
		default:
			req.Failf("unexpected type", "eth_maxPriorityFeePerGas result has unsupported type %T", v)
		}
		req.GreaterOrEqual(rpcTip.Sign(), 0)
		t.Logf("eth_maxPriorityFeePerGas: %v (%s wei)", tipRaw, rpcTip.String())
	}

	// ethclient SuggestGasTipCap
	var tip *big.Int
	{
		cctx, cancel := context.WithTimeout(harness.Ctx, utils.RevertReasonTimeout)
		defer cancel()
		var e error
		tip, e = chain.EthClient.SuggestGasTipCap(cctx)
		req.NoError(e)
		req.NotNil(tip)
		req.GreaterOrEqual(tip.Sign(), 0)
		t.Logf("ethclient.SuggestGasTipCap: %s wei", tip.String())
	}

	// Rough consistency check (within 5x) when both are positive
	if rpcTip.Sign() > 0 && tip.Sign() > 0 {
		minTip := new(big.Int).Set(rpcTip)
		maxTip := new(big.Int).Set(tip)
		if minTip.Cmp(maxTip) > 0 {
			minTip, maxTip = maxTip, minTip
		}
		fiveMin := new(big.Int).Mul(minTip, big.NewInt(5))
		req.LessOrEqual(maxTip.Cmp(fiveMin), 0, "maxPriorityFeePerGas (%s) and SuggestGasTipCap (%s) differ by >5x", rpcTip.String(), tip.String())
	}
}

// TestJSONRPCPendingState validates pending vs latest nonces around a just-submitted tx.
// Pre-inclusion, pending should equal latest+1; after inclusion, latest catches up.
func TestJSONRPCPendingState(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain
	// Baseline: latest and pending nonces should be consistent
	latest0, err := chain.EthClient.NonceAt(harness.Ctx, harness.SenderAddr, nil)
	req.NoError(err)

	pending0, err := chain.EthClient.PendingNonceAt(harness.Ctx, harness.SenderAddr)
	req.NoError(err)
	req.GreaterOrEqual(pending0, latest0, "pending nonce should be >= latest nonce at baseline")

	// Send a minimal transfer from the funded sender to a fresh recipient
	recvKey, err := crypto.GenerateKey()
	req.NoError(err)
	recipient := crypto.PubkeyToAddress(recvKey.PublicKey)

	value := big.NewInt(1) // 1 wei
	gas := uint64(21000)   // basic transfer

	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &recipient, value, nil, gas)
	req.NoError(err)

	// Observe pending vs latest before inclusion. It's possible the tx is mined
	// so quickly that we don't catch the pre-inclusion window; that's not a failure.
	sawPendingBump := false
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		latest, err := chain.EthClient.NonceAt(harness.Ctx, harness.SenderAddr, nil)
		req.NoError(err)
		pending, err := chain.EthClient.PendingNonceAt(harness.Ctx, harness.SenderAddr)
		req.NoError(err)

		if latest == latest0 && pending == latest0+1 {
			sawPendingBump = true
			t.Log("Observed pending == latest+1 prior to inclusion")
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Wait for inclusion and assert success
	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)

	if !sawPendingBump {
		t.Log("Tx mined quickly; did not observe pre-inclusion pending bump; continuing")
	}

	// After inclusion: latest nonce should have caught up by +1; pending >= latest
	latest1, err := chain.EthClient.NonceAt(harness.Ctx, harness.SenderAddr, nil)
	req.NoError(err)
	req.Equal(latest0+1, latest1)

	pending1, err := chain.EthClient.PendingNonceAt(harness.Ctx, harness.SenderAddr)
	req.NoError(err)
	req.GreaterOrEqual(pending1, latest1)
}

// TestJSONRPCSanity ensures core JSON-RPC calls behave as expected:
// - chainId and net_version match expected (1946 / 0x79a)
// - blockNumber advances over time
// - gasPrice >= baseFee and feeHistory returns sane values
// - estimateGas for a simple transfer is usable and the tx succeeds
func TestJSONRPCSanity(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	// chainId and net_version
	cid, err := chain.EthClient.ChainID(harness.Ctx)
	req.NoError(err)
	req.Equal(utils.TestEVMChainID, cid.Uint64())

	nid, err := chain.EthClient.NetworkID(harness.Ctx)
	req.NoError(err)
	req.Equal(utils.TestEVMChainID, nid.Uint64())

	// Ensure at least one block has been produced
	req.NoError(utils.WaitForBlocks(harness.Ctx, chain.EthClient, 1))

	// gasPrice and fee history sanity
	gp, err := chain.EthClient.SuggestGasPrice(harness.Ctx)
	req.NoError(err)
	req.NotNil(gp)
	req.Positive(gp.Sign(), "gasPrice should be > 0")

	hdr, err := chain.EthClient.HeaderByNumber(harness.Ctx, nil)
	req.NoError(err)
	req.NotNil(hdr)
	if hdr.BaseFee != nil {
		req.Positive(hdr.BaseFee.Sign(), "baseFee should be > 0")
		req.GreaterOrEqual(gp.Cmp(hdr.BaseFee), 0, "gasPrice %s < baseFee %s", gp.String(), hdr.BaseFee.String())
	}

	fh, err := chain.EthClient.FeeHistory(harness.Ctx, 4, nil, []float64{25, 50, 75})
	req.NoError(err)
	req.NotNil(fh)
	req.NotEmpty(fh.BaseFee)
	for _, bf := range fh.BaseFee {
		req.NotNil(bf)
		req.GreaterOrEqual(bf.Sign(), 0)
	}

	// estimateGas usable for a simple transfer, then send using that estimate
	recvKey, err := crypto.GenerateKey()
	req.NoError(err)
	recipient := crypto.PubkeyToAddress(recvKey.PublicKey)
	value := big.NewInt(1) // 1 wei

	est, err := chain.EthClient.EstimateGas(harness.Ctx, ethereum.CallMsg{From: harness.SenderAddr, To: &recipient, Value: value})
	req.NoError(err)
	req.GreaterOrEqual(int(est), 21000)
	req.Less(int(est), 100000)

	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &recipient, value, nil, est)
	req.NoError(err)

	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)
}

// TestJSONRPCSubscriptions_NewHeads subscribes to newHeads and asserts headers arrive.
func TestJSONRPCSubscriptions_NewHeads(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	ws := wsClientOrFail(harness.Ctx, t, chain.WS)
	defer ws.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	headersCh := make(chan *types.Header, 16)
	sub, err := ws.SubscribeNewHead(ctx, headersCh)
	req.NoError(err)
	defer sub.Unsubscribe()

	// Trigger a block by sending a trivial tx
	recvKey, err := crypto.GenerateKey()
	req.NoError(err)
	recipient := crypto.PubkeyToAddress(recvKey.PublicKey)
	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &recipient, big.NewInt(1), nil, utils.BasicTransferGas)
	req.NoError(err)
	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)

	select {
	case h := <-headersCh:
		req.NotNil(h)
		// basic sanity: non-nil number
		req.NotNil(h.Number)
	case err := <-sub.Err():
		req.NoError(err)
	case <-ctx.Done():
		req.Fail("timeout waiting for new head via websocket subscription")
	}
}

// TestJSONRPCSubscriptions_Logs subscribes to logs, deploys Counter, emits an event, and asserts a log is received.
func TestJSONRPCSubscriptions_Logs(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	ws := wsClientOrFail(harness.Ctx, t, chain.WS)
	defer ws.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	parsedABI, err := abi.JSON(strings.NewReader(contracts.CounterABIJSON()))
	req.NoError(err)
	creation := common.FromHex(contracts.CounterBinHex())

	// Deploy contract
	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, nil, big.NewInt(0), creation, 0)
	req.NoError(err)
	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)
	contractAddr := rec.ContractAddress

	logsCh := make(chan types.Log, 16)
	sub, err := ws.SubscribeFilterLogs(ctx, ethereum.FilterQuery{Addresses: []common.Address{contractAddr}}, logsCh)
	req.NoError(err)
	defer sub.Unsubscribe()

	// Call set(7) which emits ValueChanged
	callData, err := parsedABI.Pack("set", big.NewInt(7))
	req.NoError(err)
	txHash2, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &contractAddr, big.NewInt(0), callData, 0)
	req.NoError(err)
	rec2, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash2, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec2)
	req.Equal(uint64(1), rec2.Status)

	// Wait for a matching log
	for {
		select {
		case lg := <-logsCh:
			if lg.Address == contractAddr && lg.TxHash == rec2.TxHash {
				req.NotEmpty(lg.Topics)
				return
			}
			// keep waiting until timeout or matching log
		case err := <-sub.Err():
			req.NoError(err)
		case <-ctx.Done():
			req.Fail("timeout waiting for contract event log via websocket subscription")
		}
	}
}

// TestDebugTraceOnRevertedTx implements backlog item #11:
// - Send a tx that reverts
// - Call debug_traceTransaction on the tx hash
// - Assert structLogs contain the REVERT opcode
// - Skip if the debug API is unavailable
func TestDebugTraceOnRevertedTx(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	// Deploy Reverter contract
	creation := common.FromHex(contracts.ReverterBinHex())
	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, nil, nil, creation, 0)
	req.NoError(err)

	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)
	reverter := rec.ContractAddress
	req.NotEqual(common.Address{}, reverter)

	// Prepare calldata that will revert with a reason
	parsed, err := abi.JSON(strings.NewReader(contracts.ReverterABIJSON()))
	req.NoError(err)
	reason := "NOPE"
	callData, err := parsed.Pack("revertWithReason", reason)
	req.NoError(err)

	// Send a tx that reverts
	txHash2, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &reverter, nil, callData, 100000)
	req.NoError(err)
	rec2, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash2, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec2)
	req.Equal(uint64(0), rec2.Status)

	// Dial RPC and attempt debug_traceTransaction
	rpcClient, err := rpc.DialContext(harness.Ctx, chain.RPC)
	req.NoError(err, "RPC not reachable for debug endpoint")
	defer rpcClient.Close()

	// Short timeout for the trace call
	tctx, cancel := context.WithTimeout(harness.Ctx, 10*time.Second)
	defer cancel()

	// Minimal trace result shape we care about
	type structLog struct {
		Op string `json:"op"`
	}
	type traceResult struct {
		Failed     bool        `json:"failed"`
		StructLogs []structLog `json:"structLogs"`
	}

	var out traceResult
	err = rpcClient.CallContext(tctx, &out, "debug_traceTransaction", rec2.TxHash.Hex(), map[string]any{})
	req.NoError(err, "debug_traceTransaction must be available and succeed")

	req.NotEmpty(out.StructLogs, "structLogs should not be empty for a reverted tx")

	// Find a REVERT opcode in the trace
	foundRevert := false
	for _, sl := range out.StructLogs {
		if strings.EqualFold(sl.Op, "REVERT") {
			foundRevert = true
			break
		}
	}
	req.True(foundRevert, "expected REVERT opcode in structLogs")
}

// TestJSONRPCTxReceipts covers JSON-RPC tx and receipt lookups:
// - Send a type-2 transfer and wait for inclusion
// - Verify receipt fields (status, gasUsed, effectiveGasPrice >= baseFee)
// - Lookup via getTransactionByHash, byBlockHash+Index, byBlockNumber+Index
// - Verify from/to/nonce/type across all variants
func TestJSONRPCTxReceipts(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	recvKey, err := crypto.GenerateKey()
	req.NoError(err)
	recipient := crypto.PubkeyToAddress(recvKey.PublicKey)

	expectedNonce, err := chain.EthClient.PendingNonceAt(harness.Ctx, harness.SenderAddr)
	req.NoError(err)

	value := big.NewInt(1_000_000_000_000_000_000) // 1e18 wei
	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &recipient, value, nil, utils.BasicTransferGas)
	req.NoError(err)

	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)
	req.GreaterOrEqual(int(rec.GasUsed), utils.BasicTransferGas)

	hdr, err := chain.EthClient.HeaderByNumber(harness.Ctx, rec.BlockNumber)
	req.NoError(err)
	req.NotNil(hdr)
	if hdr.BaseFee != nil {
		req.GreaterOrEqual(rec.EffectiveGasPrice.Cmp(hdr.BaseFee), 0)
	}

	tx, isPending, err := chain.EthClient.TransactionByHash(harness.Ctx, txHash)
	req.NoError(err)
	req.False(isPending)

	chainID, err := chain.EthClient.ChainID(harness.Ctx)
	req.NoError(err)
	from, err := types.Sender(types.LatestSignerForChainID(chainID), tx)
	req.NoError(err)
	req.Equal(harness.SenderAddr, from)

	to := tx.To()
	req.NotNil(to)
	req.Equal(recipient, *to)
	req.Equal(expectedNonce, tx.Nonce())
	req.Equal(uint8(types.DynamicFeeTxType), tx.Type())

	rc, err := rpc.DialContext(harness.Ctx, chain.RPC)
	req.NoError(err)
	defer rc.Close()

	type rpcTx struct {
		Hash             common.Hash     `json:"hash"`
		From             common.Address  `json:"from"`
		To               *common.Address `json:"to"`
		Nonce            hexutil.Uint64  `json:"nonce"`
		Type             hexutil.Uint64  `json:"type"`
		TransactionIndex hexutil.Uint64  `json:"transactionIndex"`
	}

	idx := hexutil.Uint64(rec.TransactionIndex)
	var byHashIdx rpcTx
	err = rc.CallContext(harness.Ctx, &byHashIdx, "eth_getTransactionByBlockHashAndIndex", rec.BlockHash, idx)
	req.NoError(err)
	req.Equal(txHash, byHashIdx.Hash)
	req.Equal(harness.SenderAddr, byHashIdx.From)
	req.NotNil(byHashIdx.To)
	req.Equal(recipient, *byHashIdx.To)
	req.Equal(hexutil.Uint64(expectedNonce), byHashIdx.Nonce)
	req.Equal(hexutil.Uint64(types.DynamicFeeTxType), byHashIdx.Type)
	req.Equal(idx, byHashIdx.TransactionIndex)

	var byNumIdx rpcTx
	bn := hexutil.Uint64(rec.BlockNumber.Uint64())
	err = rc.CallContext(harness.Ctx, &byNumIdx, "eth_getTransactionByBlockNumberAndIndex", bn, idx)
	req.NoError(err)
	req.Equal(txHash, byNumIdx.Hash)
	req.Equal(harness.SenderAddr, byNumIdx.From)
	req.NotNil(byNumIdx.To)
	req.Equal(recipient, *byNumIdx.To)
	req.Equal(hexutil.Uint64(expectedNonce), byNumIdx.Nonce)
	req.Equal(hexutil.Uint64(types.DynamicFeeTxType), byNumIdx.Type)
	req.Equal(idx, byNumIdx.TransactionIndex)

	rcvd, err := chain.EthClient.TransactionReceipt(harness.Ctx, txHash)
	req.NoError(err)
	req.NotNil(rcvd)
	req.Equal(uint64(1), rcvd.Status)
	req.Equal(txHash, rec.TxHash)
	req.Equal(txHash, rcvd.TxHash)
}

// TestJSONRPCTxpool submits a tx and verifies it appears in txpool.pending.
// This test fails if the txpool namespace is unavailable or if the tx is mined before inspection.
func TestJSONRPCTxpool(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	// Raw RPC client for txpool_* methods
	rc, err := rpc.DialContext(harness.Ctx, chain.RPC)
	req.NoError(err)
	defer rc.Close()

	// Require txpool_status to be available
	var status map[string]any
	err = rc.CallContext(harness.Ctx, &status, "txpool_status")
	req.NoError(err)

	// Submit a minimal transfer (1 wei) from funded sender to a fresh recipient
	recvKey, err := crypto.GenerateKey()
	req.NoError(err)
	recipient := crypto.PubkeyToAddress(recvKey.PublicKey)

	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &recipient, big.NewInt(1), nil, utils.BasicTransferGas)
	req.NoError(err)
	t.Logf("submitted tx %s", txHash.Hex())

	type poolContent struct {
		Pending map[string]map[string]any `json:"pending"`
		Queued  map[string]map[string]any `json:"queued"`
	}

	foundPending := false
	foundQueued := false
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		var pc poolContent
		err := rc.CallContext(harness.Ctx, &pc, "txpool_content")
		req.NoError(err)

		for addr := range pc.Pending {
			if strings.EqualFold(addr, harness.SenderAddr.Hex()) {
				foundPending = true
				break
			}
		}
		if !foundPending {
			for addr := range pc.Queued {
				if strings.EqualFold(addr, harness.SenderAddr.Hex()) {
					foundQueued = true
					break
				}
			}
		}
		if foundPending {
			break
		}
		time.Sleep(150 * time.Millisecond)
	}

	// Require presence in txpool.pending
	req.Truef(foundPending, "expected sender %s in txpool.pending; queuedSeen=%v tx=%s", harness.SenderAddr.Hex(), foundQueued, txHash.Hex())
}

// wsClientOrFail dials the websocket endpoint with a short timeout based on the base context.
func wsClientOrFail(ctx context.Context, t *testing.T, wsURL string) *ethclient.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	c, err := ethclient.DialContext(ctx, wsURL)
	require.NoErrorf(t, err, "failed to dial websocket endpoint at %s", wsURL)
	return c
}
