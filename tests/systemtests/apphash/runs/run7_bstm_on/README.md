# Run 7 — BlockSTM ON

**Date:** 2025-03-12
**Branch:** `vlad/indeterminism-test`
**Result:** DIVERGED at height 341 (node2)

## Settings

- BlockSTM: **ENABLED**
- Validators: 4
- Senders: 50
- Txs per batch: 3
- Send amount: 100 atest
- Hot-path logging: OFF (no APPLY_TX/STATEDB_COMMIT stderr prints)

## Divergence

- **Height:** 341
- **Divergent node:** node2
- **Account:** `cosmos1fx944mzagwdhx0wz7k9tfztc8g3lkfk6pzezqh`
- **Balances:**
  - node0: 1000000000000004729800 atest
  - node1: 1000000000000004729800 atest
  - node2: 1000000000000004729600 atest (SHORT by 200)
  - node3: 1000000000000004729800 atest
- **Difference:** 200 atest = 2 × 100 send amount (2 lost writes)
- **Pre-divergence (h340):** All 4 nodes identical
- **Tx lists (h341):** Identical across all 4 nodes (same hashes, same order, same block hash)

## Transaction Analysis

204 txs in block 341, all targeting the same recipient: `0x498B5AeC5D439b733dC2F58AB489783A23FB26dA` (= `cosmos1fx944mzagwdhx0wz7k9tfztc8g3lkfk6pzezqh`). 50 unique senders, each with nonces 939–942 (3–4 txs per sender per batch). Every tx sends `value=100`.

Since all 204 txs credit the same account, this is a textbook lost-update race under BlockSTM parallel execution: multiple threads read the same balance, compute `balance + 100`, and write back — 2 of those writes were silently dropped on node2.

## Files

- `node{0-3}_bank_h340.json` — bank genesis export at height 340 (pre-divergence)
- `node{0-3}_bank_h341.json` — bank genesis export at height 341 (divergence)
- `node{0-3}_txs_h341.txt` — block transactions at height 341
- `node{0-3}.out` — full node stdout+stderr logs
