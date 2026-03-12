# Run 10 — EVM Value Transfers + BlockSTM ON + Experimental Mempool ON

**Date:** 2026-03-12
**Branch:** `vlad/indeterminism-test`
**Result:** DIVERGED at height 80 (node1)
**Test:** `TestLiveHotSendsAppHash` (EVM value transfers)

## Settings

- BlockSTM: **ENABLED**
- Experimental EVM Mempool: **ENABLED** (custom CheckTx, InsertTx, ReapTxs, PrepareProposal handlers installed)
- `OperateExclusively = true`
- Validators: 4
- Senders: 50
- Txs per batch: 3 (150 txs/batch)
- Send amount: 100 atest
- Tx type: **EVM value transfers**

## Divergence

- **Height:** 80
- **Divergent node:** node1
- **Pre-divergence (h79):** All 4 nodes identical
- **h80:** node1 diverges (different bank hash), node0/2/3 identical
- **Only bank store diverges** — consistent with all previous runs

## Significance

This is a control run paired with Run 9. Same binary, same BlockSTM setting, only difference is `disableExperimentalMempool`:

| Run | Exp Mempool | BlockSTM | Result |
|-----|-------------|----------|--------|
| 9   | OFF         | ON       | No divergence (500+ blocks) |
| 10  | ON          | ON       | **Diverged h80** |

This narrows the root cause: **BlockSTM alone does not cause divergence**. The experimental EVM mempool's custom ABCI handlers (CheckTx, InsertTx, ReapTxs, PrepareProposal) are a required ingredient. The interaction between the custom mempool's tx ordering/proposal building and BlockSTM's parallel execution produces the lost-update race.

## Files

- `node{0-3}.out` — full node stdout+stderr logs
- `node{0-3}_bank_h79.json` — bank genesis export at height 79 (pre-divergence, all identical)
- `node{0-3}_bank_h80.json` — bank genesis export at height 80 (divergence, node1 differs)
- `node{0-3}_txs_h80.txt` — block transactions at height 80
- `test.log` — full test output
