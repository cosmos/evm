#!/usr/bin/make -f

WAIT_TIME ?= 20s

test:
	go test -mod=readonly -failfast -timeout=25m -tags='system_test' ./... --wait-time=$(WAIT_TIME) --verbose --binary evmd --chain-id local-4221

