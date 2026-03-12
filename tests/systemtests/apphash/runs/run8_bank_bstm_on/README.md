# Run 8 — Bank MsgSend + BlockSTM ON

**Date:** 2026-03-12
**Branch:** `vlad/indeterminism-test`
**Result:** DIVERGED at height 66 (node3)
**Test:** `TestLiveBankSendsAppHash` (cosmos-native MsgSend, NOT EVM value transfers)

## Settings

- BlockSTM: **ENABLED**
- Validators: 4
- Senders: 50
- Txs per batch: 3 (150 txs/batch)
- Send amount: 100 atest
- Tx type: **cosmos MsgSend** (bank module `SendCoins` path)

## Divergence

- **Height:** 66
- **Divergent node:** node3
- **Batch:** 44 (sent=150, skipped=0 per batch)
- **APP_HASH:**
  - node0: `2C1EF0B1DB8CA7D0DA5B38EC815BD0E543FF50330B34FFCAE41DCE319B1D1C60`
  - node1: `2C1EF0B1DB8CA7D0DA5B38EC815BD0E543FF50330B34FFCAE41DCE319B1D1C60`
  - node2: `2C1EF0B1DB8CA7D0DA5B38EC815BD0E543FF50330B34FFCAE41DCE319B1D1C60`
  - node3: `1F3E3DFCB7CDD4D73BE70449C0F44EBCC4D0D4214B8CE20CA3123B3EECE42C73`
- **Only bank store diverges** — all other stores (acc, evm, staking, distribution, feemarket, etc.) identical
- **0 txs in divergent block** — damage done in a prior block, manifested at h66
- **Pre-divergence (h65):** All 4 nodes identical

## Exact State Diff

Single account diverges: `cosmos17xpfvakm2amg962yls6f84z3kell8c5lserqta`
- node0/1/2: `990006600000000000` atest
- node3: `975006500000000000` atest
- **Difference:** `15000100000000000` atest = exactly **1 tx fee** (`150001 * 100 gwei`)

Supply is identical across all nodes. This is a **lost fee deduction** — one MsgSend tx's fee was not properly deducted on node3 (or was deducted and then refunded/lost during BlockSTM retry). Different from the EVM test where the lost write was the value transfer itself.

## Significance

This proves the BlockSTM lost-update bug is **NOT specific to the EVM statedb layer**. Standard cosmos-native `MsgSend` through `SendCoins` also diverges. The bug is in **BlockSTM's core conflict tracking** for the bank module, not the EVM's `UncheckedSetBalance` bypass.

The fact that the lost write is a *fee deduction* (not the value transfer) suggests BlockSTM's conflict surface extends across the entire tx lifecycle — ante handler fee deduction also races under parallel execution.

## Files

- `node{0-3}_bank_h65.json` — bank genesis export at height 65 (pre-divergence, all identical)
- `node{0-3}_bank_h66.json` — bank genesis export at height 66 (divergence)
- `node{0-3}.out` — full node stdout+stderr logs
- `node{0-3}_txs_h66.txt` — block transactions at height 66 (0 txs in all)
