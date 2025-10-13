#!/usr/bin/make -f

WAIT_TIME ?= 30s
BLOCK_TIME ?= 5s

test:
    go test -mod=readonly -failfast -timeout=5m -p=1 -parallel 1 -tags='system_test' -run TestTxsOrdering ./... --wait-time=$(WAIT_TIME) --block-time=$(BLOCK_TIME) --verbose --binary evmd --chain-id local-4221
    go test -mod=readonly -failfast -timeout=15m -p=1 -parallel 1 -tags='system_test' -run TestTxsReplacement ./... --wait-time=$(WAIT_TIME) --block-time=$(BLOCK_TIME) --verbose --binary evmd --chain-id local-4221
    go test -mod=readonly -failfast -timeout=5m -p=1 -parallel 1 -tags='system_test' -run TestExceptions ./... --wait-time=$(WAIT_TIME) --block-time=$(BLOCK_TIME) --verbose --binary evmd --chain-id local-4221
    go test -mod=readonly -failfast -timeout=5m -p=1 -tags='system_test' -run TestEIP7702 ./... --wait-time=$(WAIT_TIME) --verbose --binary evmd --chain-id local-4221
    go test -mod=readonly -failfast -timeout=5m -p=1 -tags='system_test' -run TestEIP712 ./... --wait-time=$(WAIT_TIME) --verbose --binary evmd --chain-id local-4221
    go test -mod=readonly -failfast -timeout=5m -p=1 -tags='system_test' -run 'TestUpgrade|TestEth' ./... --wait-time=$(WAIT_TIME) --verbose --binary evmd --chain-id local-4221
