# BlockSTM Apphash Indeterminism — Multi-Agent Analysis

10 independent research agents analyzed different facets of the apphash divergence bug that correlates with BlockSTM parallel execution. This document synthesizes their findings.

Note: While BlockSTM is strongly correlated with the divergence (8/8 runs diverge with it ON, 0/3+ with it OFF), these analyses are **hypotheses** about the mechanism, not proven root causes. The agents explored the codebase without runtime debugging — the actual conflict path has not been traced at the instruction level.

---

## Key Finding: Bank-Native MsgSend Also Diverges (Run 8)

A new test (`TestLiveBankSendsAppHash`) using standard cosmos `MsgSend` through the `SendCoins` path (no EVM, no `UncheckedSetBalance`) diverged at height 66 (node3) with BlockSTM ON.

- Same pattern: only bank store diverges, all other stores identical
- 0 txs in the divergent block itself — damage done in a prior block
- The lost write was exactly 1 tx fee (not a value transfer)
- This eliminates the `SpendableCoin` → `UncheckedSetBalance` hypothesis from Analysis 2

This suggests the issue is not specific to the EVM statedb layer. Possible explanations include:
1. BlockSTM's multi-version store failing to track bank `Balances` collection reads/writes
2. CacheKVStore reads hiding from BlockSTM's conflict detection (Analysis 4)
3. An issue with how `collections.Map` (used by bank's `Balances`) interacts with BlockSTM
4. Something else entirely — these are hypotheses, not confirmed mechanisms

---

## 1. BlockSTM Internals

BlockSTM (`txnrunner.NewSTMRunner`) in Cosmos SDK v0.54.0-rc.1 uses a multi-version store to track per-transaction reads and writes. Conflict detection relies on comparing each transaction's read set against other transactions' write sets.

**Hypothesis:** The EVM statedb uses `UncheckedSetBalance` — a private API not designed for external module use. BlockSTM's dependency tracker may be optimized for standard bank APIs like `SendCoins`. Direct raw writes through `Unchecked*` methods could fall into tracking blind spots. However, Run 8's divergence through the standard `SendCoins` path weakens this as the sole explanation.

Additional concern: the denom conversion between `GetEVMCoinDenom()` and `GetEVMCoinExtendedDenom()` may cause the conflict detector to see reads and writes as targeting different keys, even though they refer to the same balance.

## 2. Bank Module Write Path Comparison

**Standard path (MsgSend):**
```
SendCoins → SubUnlockedCoins + AddCoins
```
Single-key read-write on `Balances[addr]`. BlockSTM should track this dependency.

**EVM path (SetBalance):**
```
SpendableCoin(addr)       → READ  Balances[addr] + LockedCoins[addr]
LockedCoins(addr)         → READ  LockedCoins[addr]
UncheckedSetBalance(addr) → WRITE Balances[addr]
```
Reads 2 keys, writes 1. The read-modify-write is non-atomic.

**Run 8 result:** `MsgSend` through the standard path also diverges. The lost write was a fee deduction (not value transfer), suggesting the conflict surface spans the entire tx lifecycle including the ante handler.

## 3. StateDB Commit & BlockSTM Retry Behavior

**Hypothesis:** When BlockSTM detects a conflict and retries a transaction:
- A new cache context is created with a fresh statedb
- `getStateObject()` reads from `keeper.GetAccount(ctx)`, which hits the current store state
- If the first attempt already committed via `CacheContext.Write()`, that committed state becomes the baseline for the retry

Stale reads from a previous execution attempt could contaminate retry execution. This would be a transactional atomicity violation. Not confirmed — would require tracing actual BlockSTM retry behavior at runtime.

## 4. CacheKVStore Isolation

**Hypothesis:** The EVM statedb wraps the multi-store via `snapshotmulti.Store`, which manages a stack of `CacheWrap` instances above BlockSTM's multi-version store layer. If balance reads are satisfied from the CacheKVStore cache rather than penetrating to the multi-version store, they would be invisible to BlockSTM's read-set tracking.

This could explain how two parallel transactions read the same balance without BlockSTM detecting a conflict. However, it's unclear whether this caching behavior actually occurs in the bank module's standard `SendCoins` path (which also diverged in Run 8).

## 5. Store Key Encoding

`SpendableCoin` reads from `Balances[denom][addr]` and `LockedCoins[addr]`. `UncheckedSetBalance` writes to `Balances[denom][addr]`. The keys match — this is NOT a prefix mismatch issue.

The concern is the non-atomic read-modify-write pattern, but key encoding itself is not the problem.

## 6. Non-BlockSTM Indeterminism Source (Pre-v0.4.x Bug)

**Finding:** Unsorted map iteration in `StateDB.Finalise()` (`statedb.go:140-150`):
```go
for addr := range s.journal.dirties {  // UNSORTED MAP ITERATION
    obj, exist := s.stateObjects[addr]
    ...
    delete(s.stateObjects, obj.address)
}
```
Go map iteration is randomized. While `commitWithCtx()` properly sorts via `sortedDirties()`, `Finalise()` bypasses this. However, it only deletes from an in-memory map (not the store), and the actual store commit is sorted, so the practical impact is unclear. Needs further investigation to determine if this is in the consensus path.

**Other suspects:** Bloom collection with iterator deletion during iteration (`keeper.go:194-211`), runtime-dependent BlockSTM parallelism (`GOMAXPROCS`/`NumCPU`).

## 7. IAVL Hash Sensitivity

**Confirmed: IAVL is order-agnostic.** IAVL v1.2.6 is a balanced BST that produces the same root hash regardless of write order. The codebase also sorts all writes before hitting the store (`sortedDirties()`, `SortedKeys()`).

The apphash divergence is NOT caused by write ordering — it's from different final values.

## 8. Supply Invariant

Supply is identical across all nodes at divergence, but account balances differ. In Run 8 (bank MsgSend), the divergent account was short by exactly 1 tx fee — suggesting a fee deduction was lost or duplicated on one node.

In precisebank's `sendExtendedCoins`, the debit and credit are NOT atomic — multiple separate operations occur (integer transfer, sender borrow, recipient carry, fractional balance persist). Under BlockSTM re-execution, partial writes could persist from failed attempts. This is a hypothesis — not confirmed.

## 9. Gas Refund as Additional Conflict Surface

There are **at least 3 bank write operations per EVM transaction:**
1. **Fee deduction** (pre-execution): Sender → FeeCollector
2. **Value transfer** (EVM execution): Sender → Recipient via `SetBalance`
3. **Gas refund** (post-execution): FeeCollector → Sender via `RefundGas`

Run 8's divergence was exactly 1 tx fee, which is consistent with a lost fee deduction. This suggests the ante handler's fee path is also vulnerable, not just value transfers.

## 10. Fix Strategies

| Strategy | Feasibility | Effort | Risk |
|----------|------------|--------|------|
| Disable BlockSTM entirely | HIGH | 1 day | Low — kills parallelism |
| Serialize txs touching same account | HIGH | 2-3 days | Low — partial parallelism preserved |
| Convert to delta-based SendCoins | MEDIUM-HIGH | 2-3 days | Medium — locked coins edge cases |
| Atomic AddBalance keeper method | MEDIUM | 3-4 days | Medium — new API surface |
| BlockSTM conflict hints | LOW | N/A | Requires upstream SDK changes |

**Recommended:** Disable BlockSTM as immediate fix, then investigate the actual conflict detection mechanism to determine the correct long-term solution.

---

## Summary

BlockSTM parallel execution is strongly correlated with bank balance divergence (8/8 ON vs 0/3+ OFF). The divergence affects both EVM value transfers and cosmos-native MsgSend, and can manifest as lost value transfers OR lost fee deductions. Only the bank store diverges; supply remains consistent.

The agents identified several plausible mechanisms:
1. CacheKVStore reads hiding from BlockSTM's conflict detection
2. Non-atomic read-modify-write patterns on bank balances
3. Multiple bank write surfaces per tx creating a large conflict area
4. Possible state leakage across BlockSTM retry attempts

These are hypotheses based on code analysis. The actual root cause requires runtime tracing of BlockSTM's conflict detection to determine exactly where and why the read-write dependency is missed.
