# Run 11 — EVM Value Transfers + BlockSTM ON + Exp Mempool ON + Interblock Cache OFF

**Date:** 2026-03-12
**Branch:** `vlad/indeterminism-test`
**Result:** DIVERGED at height 33 (node2)
**Test:** `TestLiveHotSendsAppHash` (EVM value transfers)

## Settings

- BlockSTM: **ENABLED**
- Experimental EVM Mempool: **ENABLED**
- Interblock Cache: **DISABLED** (`inter-block-cache = false` in all 4 nodes' app.toml)
- Validators: 4
- Senders: 50
- Txs per batch: 3 (150 txs/batch)
- Send amount: 100 atest

## Divergence

- **Height:** 33
- **Divergent node:** node2
- **Pre-divergence (h32):** All 4 nodes identical
- **h33:** node2 diverges (different bank hash), node0/1/3 identical
- **Only bank store diverges**

## Significance

Disabling the interblock cache did not prevent divergence — it diverged even faster (h33 vs h80 in run 10). The interblock cache is **not** a factor in the root cause.

## Files

- `node{0-3}.out` — full node logs
- `node{0-3}_bank_h32.json` — bank export at h32 (pre-divergence, all identical)
- `node{0-3}_bank_h33.json` — bank export at h33 (divergence, node2 differs)
- `node{0-3}_txs_h33.txt` — block transactions at h33
- `test.log` — full test output
