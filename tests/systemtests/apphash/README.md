# Apphash Indeterminism Reproducers

System tests that reproduce non-deterministic apphash divergence between nodes. Divergence correlates strongly with BlockSTM parallel execution being enabled, but the exact mechanism has not been confirmed at the instruction level.

## Tests

### EVM Value Transfers (`TestLiveHotSendsAppHash`)

50 senders blast EVM value transfers (100 atest each) to a single recipient. Diverges consistently when BlockSTM is enabled.

```bash
cd tests/systemtests
ulimit -n 10240
EVM_RUN_LIVE_APPHASH_REPRO=1 go test -failfast -timeout=30m \
  -mod=readonly -tags=system_test -count=1 -v \
  -run TestLiveHotSendsAppHash ./... \
  --wait-time=20s --binary evmd --chain-id local-4221
```

### Bank MsgSend (`TestLiveBankSendsAppHash`)

Same contention pattern but uses cosmos-native `MsgSend` through the standard `SendCoins` path. No EVM involvement. Also diverges with BlockSTM enabled, suggesting the issue is not specific to the EVM statedb layer.

```bash
cd tests/systemtests
ulimit -n 10240
EVM_RUN_BANK_APPHASH_REPRO=1 go test -failfast -timeout=30m \
  -mod=readonly -tags=system_test -count=1 -v \
  -run TestLiveBankSendsAppHash ./... \
  --wait-time=20s --binary evmd --chain-id local-4221
```

## Toggling BlockSTM

Comment/uncomment `bApp.SetBlockSTMTxRunner(...)` in `evmd/app.go` ~line 259. Rebuild after:

```bash
cd evmd && go build -o ../tests/systemtests/binaries/evmd ./cmd/evmd
```

## Monitoring

Poll all 4 nodes' APP_HASH every N seconds while a test is running:

```bash
bash apphash/poll.sh 10
```

Stops automatically when nodes stop.

## Test Config

- 4 validators, 500ms commit timeout
- 50 senders, 3 txs per sender per batch (150 txs/batch)
- 100 atest per send
- `no_base_fee=true`, gas price 100 gwei
- Mempool: global-slots=50000, global-queue=10000

## Structure

```
apphash/
  live_repro.go     # EVM value transfer reproducer
  bank_repro.go     # Cosmos MsgSend reproducer
  poll.sh           # APP_HASH polling script
  analyses/         # Hypothesized root cause analysis (10-agent research)
  runs/             # Labeled output folders per test run
    run7_bstm_on/   # EVM, BlockSTM ON, diverged h341
    run8_bank_bstm_on/  # Bank MsgSend, BlockSTM ON, diverged h66
```

## Results Summary

Divergence correlates with BlockSTM: ON diverges within 30-350 blocks (8/8 runs), OFF never diverges (3/3 runs + 3805-block overnight). Both EVM and cosmos-native tx paths are affected. The divergence always manifests in the bank store only — all other stores remain consistent across nodes.

See `apphash_investigation.md` in the repo root for full writeup and `analyses/` for hypothesized mechanisms.
