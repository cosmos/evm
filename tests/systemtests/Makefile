#!/usr/bin/make -f

WAIT_TIME ?= 20s

test:
	go test -failfast -timeout=30m -mod=readonly -tags='system_test' -v ./... --wait-time=$(WAIT_TIME) --binary evmd --chain-id local-4221
