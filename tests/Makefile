#!/usr/bin/make -f

TESTS_TO_RUN ?= all

NIX_DEV_CMD = nix develop --accept-flake-config -c

test-e2e-nix:
	@echo "TESTS_TO_RUN=$(TESTS_TO_RUN)"
	@bash scripts/restore_envs.sh
	@CHAIN_CONFIG=$(CHAIN_CONFIG) TESTS_TO_RUN=$(TESTS_TO_RUN) bash scripts/run-integration-tests.sh

dev:
	@$(NIX_DEV_CMD) bash

lint-py:
	@cd integration_tests && uv run flake8 --show-source --count --statistics --format="::error file=%(path)s,line=%(row)d,col=%(col)d::%(path)s:%(row)d:%(col)d: %(code)s %(text)s"

.PHONY: test-e2e-nix dev lint-py
