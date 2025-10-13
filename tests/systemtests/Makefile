#!/usr/bin/make -f

WAIT_TIME ?= 60s
BLOCK_TIME ?= 8s

test:
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run TestTxsOrdering --wait-time=$(WAIT_TIME) --block-time=$(BLOCK_TIME) --binary evmd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run TestTxsReplacement --wait-time=$(WAIT_TIME) --block-time=$(BLOCK_TIME) --binary evmd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run TestExceptions --wait-time=$(WAIT_TIME) --block-time=$(BLOCK_TIME) --binary evmd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run TestEIP7702 --wait-time=$(WAIT_TIME) --binary evmd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run TestEIP712 --wait-time=$(WAIT_TIME) --binary evmd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run 'TestUpgrade|TestEth' --wait-time=$(WAIT_TIME) --binary evmd --chain-id local-4221
