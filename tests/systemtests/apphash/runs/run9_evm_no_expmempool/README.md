# Run 9 — EVM Value Transfers + BlockSTM ON + Experimental Mempool OFF

**Date:** 2026-03-12
**Branch:** `vlad/indeterminism-test`
**Result:** NO DIVERGENCE through 500+ blocks
**Test:** `TestLiveHotSendsAppHash` (EVM value transfers)

## Settings

- BlockSTM: **ENABLED**
- Experimental EVM Mempool: **DISABLED** (`disableExperimentalMempool = true`)
  - Mempool object still created (JSON-RPC dependency)
  - Custom ABCI handlers (CheckTx, InsertTx, ReapTxs, PrepareProposal) NOT installed
  - `OperateExclusively = false` — CometBFT uses its own clist mempool
- Validators: 4
- Senders: 50
- Txs per batch: 3 (150 txs/batch)
- Send amount: 100 atest
- Tx type: **EVM value transfers**

## Result

- **Height reached:** 506+
- **Divergence:** NONE
- **APP_HASH at h506:** All 4 nodes identical (`AC2A34F8E29B3B65A393D39CDC0F7CB5F8FCE428447CAD2E9CBF4395458FFBC8`)
- Test manually stopped after exceeding 500 blocks

## Significance

With BlockSTM ON but the experimental EVM mempool's custom ABCI handlers disabled, no divergence occurred through 500+ blocks. All previous runs with BlockSTM ON + experimental mempool diverged within 30-350 blocks (8/8). This suggests the experimental mempool's custom tx ordering/proposal building interacts with BlockSTM to produce the lost-update race, rather than BlockSTM alone being the cause.
