#!/usr/bin/make -f

WAIT_TIME ?= 20s

test:
	go test -failfast -timeout=30m -mod=readonly -tags='system_test' -v ./... --wait-time=$(WAIT_TIME) --binary evmd --chain-id local-4221
	BLOCKTIME=9s go test -failfast -timeout=30m -mod=readonly -tags='system_test' -v ./... -run TestMempoolMixedTxsReplacementEVMAndCosmos --wait-time=$(WAIT_TIME) --binary evmd --chain-id local-4221 --block-time=6s
