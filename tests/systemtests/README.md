# Getting started with a new system test

## Overview

The systemtests suite is an end-to-end test suite that runs the epixd process and sends RPC requests from separate Ethereum/Cosmos clients. The systemtests for cosmos/evm use the `cosmossdk.io/systemtests` package by default. For more details, please refer to https://github.com/cosmos/cosmos-sdk/tree/main/tests/systemtests.

## Preparation

Build a new binary from current branch and copy it to the `tests/systemtests/binaries` folder by running system tests.

```shell
make test-system
```

Or via manual steps

```shell
make build
mkdir -= ./tests/systemtests/binaries
cp ./build/epixd ./tests/systemtests/binaries
cp ./build/evmd ./tests/systemtests/binaries/v0.4
```

## Run Individual test

### Run test cases for txs ordering

```shell
go test -p 1 -parallel 1 -tags='system_test' -v ./... \
--run TestTxsOrdering --verbose --binary epixd --block-time 5s --chain-id local-4221
```

### Run test cases for txs replacement

```shell
go test -p 1 -parallel 1 -tags='system_test' -v ./... \
--run TestTxsReplacement --verbose --binary epixd --block-time 5s --chain-id local-4221
```

### Run test exceptions

```shell
go test -p 1 -parallel 1 -tags='system_test' -v ./... \
--run TestExceptions --verbose --binary epixd --block-time 5s --chain-id local-4221
```

### Run EIP-7702 test

```shell
go test -p 1 -tags='system_test' -v ./... \
--run TestEIP7702 --verbose --binary epixd --block-time 3s --chain-id local-4221
```

## Run all tests

```shell
make test
```
