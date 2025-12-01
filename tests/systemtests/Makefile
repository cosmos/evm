#!/usr/bin/make -f

WAIT_TIME ?= 20s

test:
	go mod tidy
	go test -failfast -timeout=30m -p=1 -tags='system_test' -v ./... -run TestTxsOrdering --wait-time=$(WAIT_TIME) --block-time=3s --binary epixd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -tags='system_test' -v ./... -run TestTxsReplacement --wait-time=$(WAIT_TIME) --block-time=9.5s --binary epixd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -tags='system_test' -v ./... -run TestExceptions --wait-time=$(WAIT_TIME) --block-time=8s --binary epixd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -tags='system_test' -v ./... -run TestEIP7702 --wait-time=$(WAIT_TIME) --block-time=5s --binary epixd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -tags='system_test' -v ./... -run TestEIP712 --wait-time=$(WAIT_TIME) --block-time=5s --binary epixd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -tags='system_test' -v ./... -run 'TestUpgrade|TestEth' --wait-time=$(WAIT_TIME) --block-time=5s --binary epixd --chain-id local-4221
