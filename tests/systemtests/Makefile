#!/usr/bin/make -f

WAIT_TIME ?= 20s

test:
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run TestCosmosTxCompat --wait-time=$(WAIT_TIME) --block-time=9.5s --binary evmd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run TestTxsReplacement --wait-time=$(WAIT_TIME) --block-time=9.5s --binary evmd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run TestTxsOrdering --wait-time=$(WAIT_TIME) --block-time=3s --binary evmd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run TestExceptions --wait-time=$(WAIT_TIME) --block-time=8s --binary evmd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run TestEIP7702 --wait-time=$(WAIT_TIME) --block-time=5s --binary evmd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run TestEIP712 --wait-time=$(WAIT_TIME) --block-time=5s --binary evmd --chain-id local-4221
	go test -failfast -timeout=30m -p=1 -mod=readonly -tags='system_test' -v ./... -run 'TestUpgrade|TestEth' --wait-time=$(WAIT_TIME) --block-time=5s --binary evmd --chain-id local-4221
