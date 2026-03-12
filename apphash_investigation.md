# Apphash Indeterminism Investigation

## TL;DR

There are **two independent sources** of apphash indeterminism:

1. **BlockSTM** — confirmed 8/8 runs (6 EVM + 1 bank + 1 overnight). Parallel tx execution causes lost bank balance writes. **Affects both EVM and cosmos-native MsgSend paths.** The bug is in BlockSTM's core conflict detection, NOT the EVM statedb layer. Locally reproducible.
2. **Unknown** — predates BlockSTM (present in v0.4.x/v0.5.x). Only reproducible on devnets so far.

## BlockSTM: Root Cause Confirmed

### The bug

When BlockSTM executes transactions in parallel, its conflict detection misses read-write conflicts on bank balance keys. Two transactions that both credit the same account will:

1. Both read the same starting balance
2. Both compute `balance + amount`
3. Second write silently overwrites the first

Result: one credit is lost. This is a classic lost-update anomaly.

### Critical finding: NOT EVM-specific

**Run 8** used cosmos-native `MsgSend` through the standard `SendCoins` path (no EVM, no `UncheckedSetBalance`, no statedb). It diverged at height 66 (node3) with the **same pattern**: only bank store diverges, all other stores identical.

This eliminates the hypothesis that the EVM statedb's `SpendableCoin` → `UncheckedSetBalance` bypass was the cause. The bug is in **BlockSTM's conflict tracking for bank balance keys** regardless of which API writes them.

### Proof (8/8 runs)

| Run | Type | BlockSTM | Result |
|-----|------|----------|--------|
| 1 | EVM | ON | Diverged at height **29** (node0) |
| 2 | EVM | OFF | **430+** blocks, no divergence |
| 3 | EVM | ON | Diverged at height **288** (node3) |
| 4 | EVM | OFF | **430+** blocks, no divergence |
| 5 | EVM | ON | Diverged at height **46** (node2) |
| 6 | EVM | OFF | **557+** blocks, no divergence |
| 7 | EVM | ON | Diverged at height **341** (node2) |
| 8 | **Bank MsgSend** | ON | Diverged at height **66** (node3) |

Overnight: 3805 blocks with BlockSTM OFF, zero divergence.

### Exact state diff at divergence

**Run 1 (EVM, height 29):**
```
account: cosmos1fx944mzagwdhx0wz7k9tfztc8g3lkfk6pzezqh
node0:     1000000000000000091400 atest
node1/2/3: 1000000000000000091800 atest
difference: 400 atest (= 4 × 100 atest per send)
```

**Run 7 (EVM, height 341):**
```
account: cosmos1fx944mzagwdhx0wz7k9tfztc8g3lkfk6pzezqh
node0/1/3: 1000000000000004729800 atest
node2:     1000000000000004729600 atest
difference: 200 atest (= 2 × 100 atest per send)
```

**Run 8 (Bank MsgSend, height 66):**
- Only bank store hash differs between node3 and nodes 0/1/2
- 0 txs in the divergent block — damage done in a prior block
- Genesis exports failed (DB locked by running nodes)

Common pattern:
- Total supply identical across all nodes — no phantom minting/burning
- All other 16 IAVL stores identical (evm, acc, staking, distribution, feemarket, etc.)
- Any node can be the divergent one (node0, node2, node3 observed)
- Difference is always a multiple of the send amount

### Timing sensitivity confirms race condition

Adding `fmt.Fprintf(os.Stderr, ...)` in the hot path (`ApplyTransaction` and `commitWithCtx`) **masked the bug for 350+ blocks**. The I/O serialization acted as an accidental mutex. Removing it brought divergence back within 29-46 blocks.

### Write paths affected

**EVM path (runs 1-7):**
```
statedb.SetBalance
  → keeper.SetBalance (reads SpendableCoin, writes via UncheckedSetBalance)
    → bankWrapper.SetBalance
      → bank.UncheckedSetBalance (writes to k.Balances collection)
```

**Cosmos path (run 8):**
```
MsgSend
  → keeper.SendCoins
    → SubUnlockedCoins (debit sender)
    → AddCoins (credit recipient)
      → both write to k.Balances collection
```

Both paths diverge under BlockSTM. The common denominator is the bank module's `Balances` collection and BlockSTM's failure to detect read-write conflicts on it.

### Likely root cause mechanisms

1. **CacheKVStore reads hiding from BlockSTM** — the `CacheWrap` stack between the execution context and BlockSTM's multi-version store means balance reads may be served from cache without registering in BlockSTM's conflict detection read set.

2. **Multiple bank write surfaces per tx** — each EVM tx has at least 3 bank writes (fee deduction, value transfer, gas refund). Even cosmos MsgSend has 2 (debit + credit). Any of these can race.

3. **`collections.Map` interaction with BlockSTM** — the bank `Balances` uses `collections.Map`. If this abstraction bypasses or doesn't properly integrate with BlockSTM's store wrappers, conflicts would be invisible.

## Eliminated suspects

| Suspect | Why eliminated |
|---------|---------------|
| EVM statedb / UncheckedSetBalance | **Run 8 diverged with standard MsgSend** |
| Map iteration in statedb | `sortedDirties()` and `SortedKeys()` ensure determinism |
| CacheKVStore.Write() | Collects dirty entries, sorts by key, writes in order |
| ARC cache | Write-through, thread-safe, query contexts bypass it |
| Virtual fee collection | Bug predates it (v0.4.x) |
| ObjStore iteration | BTree-backed, deterministic |
| CheckTx / mempool interference | Separate states, no races detected by `-race` |
| IAVL write ordering | IAVL is order-agnostic (balanced BST, same hash regardless of insert order) |

## Reproduction

Branch: `vlad/indeterminism-test`

**EVM test:**
```bash
cd tests/systemtests
ulimit -n 10240
EVM_RUN_LIVE_APPHASH_REPRO=1 go test -failfast -timeout=30m \
  -mod=readonly -tags=system_test -count=1 -v \
  -run TestLiveHotSendsAppHash ./... \
  --wait-time=20s --binary evmd --chain-id local-4221
```

**Bank MsgSend test:**
```bash
cd tests/systemtests
ulimit -n 10240
EVM_RUN_BANK_APPHASH_REPRO=1 go test -failfast -timeout=30m \
  -mod=readonly -tags=system_test -count=1 -v \
  -run TestLiveBankSendsAppHash ./... \
  --wait-time=20s --binary evmd --chain-id local-4221
```

Config: 4 validators, 50 senders × 3 txs/batch, 100 atest per send, 500ms commit timeout.

To toggle BlockSTM: comment/uncomment `bApp.SetBlockSTMTxRunner(...)` in `evmd/app.go` ~line 259.

## Run logs

- `tests/systemtests/apphash/runs/run7_bstm_on/` — EVM, BlockSTM ON, diverged h341
- `tests/systemtests/apphash/runs/run8_bank_bstm_on/` — Bank MsgSend, BlockSTM ON, diverged h66
- `tests/systemtests/apphash/analyses/` — 10-agent deep analysis
