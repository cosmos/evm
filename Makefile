#!/usr/bin/make -f

PACKAGES_NOSIMULATION=$(shell go list ./... | grep -v '/simulation')
VERSION ?= $(shell echo $(shell git describe --tags --always) | sed 's/^v//')
TMVERSION := $(shell go list -m github.com/cometbft/cometbft | sed 's:.* ::')
COMMIT := $(shell git log -1 --format='%H')
BINDIR ?= $(GOPATH)/bin
EXAMPLE_BINARY = evmd
BUILDDIR ?= $(CURDIR)/build
HTTPS_GIT := https://github.com/cosmos/evm.git
DOCKER := $(shell which docker)

export GO111MODULE = on

# Default target executed when no arguments are given to make.
default_target: all

.PHONY: build default_target

###############################################################################
###                          evmd Build & Install                           ###
###############################################################################

# process build tags
build_tags = netgo

ifeq (cleveldb,$(findstring cleveldb,$(COSMOS_BUILD_OPTIONS)))
  build_tags += gcc
endif
build_tags += $(BUILD_TAGS)
build_tags := $(strip $(build_tags))

# process linker flags

ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=os \
          -X github.com/cosmos/cosmos-sdk/version.AppName=$(EXAMPLE_BINARY) \
          -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
          -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
          -X github.com/cometbft/cometbft/version.TMCoreSemVer=$(TMVERSION)

# DB backend selection
ifeq (cleveldb,$(findstring cleveldb,$(COSMOS_BUILD_OPTIONS)))
  ldflags += -X github.com/cosmos/cosmos-sdk/types.DBBackend=cleveldb
endif

# add build tags to linker flags
whitespace := $(subst ,, )
comma := ,
build_tags_comma_sep := $(subst $(whitespace),$(comma),$(build_tags))
ldflags += -X "github.com/cosmos/cosmos-sdk/version.BuildTags=$(build_tags_comma_sep)"

ifeq (,$(findstring nostrip,$(COSMOS_BUILD_OPTIONS)))
  ldflags += -w -s
endif
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

ifeq (staticlink,$(findstring staticlink,$(COSMOS_BUILD_OPTIONS)))
  ldflags += -linkmode external -extldflags '-static'
endif

BUILD_FLAGS := -tags "$(build_tags)" -ldflags '$(ldflags)'
# check for nostrip option
ifeq (,$(findstring nostrip,$(COSMOS_BUILD_OPTIONS)))
  BUILD_FLAGS += -trimpath
endif

# check if no optimization option is passed
# used for remote debugging
ifneq (,$(findstring nooptimization,$(COSMOS_BUILD_OPTIONS)))
  BUILD_FLAGS += -gcflags "all=-N -l"
endif


BUILD_TARGETS := build install

build: BUILD_ARGS=-o $(BUILDDIR)/
build-linux:
	GOOS=linux GOARCH=amd64 $(MAKE) build

$(BUILD_TARGETS): go.sum $(BUILDDIR)/
	CGO_ENABLED="1" go $@ $(BUILD_FLAGS) $(BUILD_ARGS) ./...

$(BUILDDIR)/:
	mkdir -p $(BUILDDIR)/

###############################################################################
###                          Tools & Dependencies                           ###
###############################################################################

go.sum: go.mod
	echo "Ensure dependencies have not been modified ..." >&2
	go mod verify
	go mod tidy

vulncheck:
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	@govulncheck ./...

###############################################################################
###                           Tests & Simulation                            ###
###############################################################################

test: test-unit
test-all: test-unit test-race

# For unit tests we don't want to execute the upgrade tests in tests/e2e but
# we want to include all unit tests in the subfolders (tests/e2e/*)
PACKAGES_UNIT=$(shell go list ./... | grep -v '/tests/e2e$$')
TEST_PACKAGES=./...
TEST_TARGETS := test-unit test-unit-cover test-race

# Test runs-specific rules. To add a new test target, just add
# a new rule, customise ARGS or TEST_PACKAGES ad libitum, and
# append the new rule to the TEST_TARGETS list.
test-unit: ARGS=-timeout=15m
test-unit: TEST_PACKAGES=$(PACKAGES_UNIT)

test-race: ARGS=-race
test-race: TEST_PACKAGES=$(PACKAGES_NOSIMULATION)
$(TEST_TARGETS): run-tests

test-unit-cover: ARGS=-timeout=15m -coverprofile=coverage.txt -covermode=atomic
test-unit-cover: TEST_PACKAGES=$(PACKAGES_UNIT)
test-unit-cover:
	@echo "Filtering ignored files from coverage.txt..."
	@grep -v -E '/cmd/|/client/|/proto/|/testutil/|/mocks/|/test_.*\.go:|\.pb\.go:|\.pb\.gw\.go:|/x/[^/]+/module\.go:|/scripts/|/ibc/testing/|/version/|\.md:|\.pulsar\.go:' coverage.txt > tmp_coverage.txt && mv tmp_coverage.txt coverage.txt
	@echo "Function-level coverage summary:"
	@go tool cover -func=coverage.txt


run-tests:
ifneq (,$(shell which tparse 2>/dev/null))
	go test -tags=test -mod=readonly -json $(ARGS) $(EXTRA_ARGS) $(TEST_PACKAGES) | tparse
else
	go test -tags=test -mod=readonly $(ARGS)  $(EXTRA_ARGS) $(TEST_PACKAGES)
endif

# Use the old Apple linker to workaround broken xcode - https://github.com/golang/go/issues/65169
ifeq ($(OS_FAMILY),Darwin)
  FUZZLDFLAGS := -ldflags=-extldflags=-Wl,-ld_classic
endif

test-fuzz:
	go test -tags=test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzMintCoins ./x/precisebank/keeper
	go test -tags=test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzBurnCoins ./x/precisebank/keeper
	go test -tags=test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzSendCoins ./x/precisebank/keeper
	go test -tags=test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzGenesisStateValidate_NonZeroRemainder ./x/precisebank/types
	go test -tags=test $(FUZZLDFLAGS) -run NOTAREALTEST -v -fuzztime 10s -fuzz=FuzzGenesisStateValidate_ZeroRemainder ./x/precisebank/types

test-scripts:
	@echo "Running scripts tests"
	@pytest -s -vv ./scripts

test-solidity:
	@echo "Beginning solidity tests..."
	./scripts/run-solidity-tests.sh

.PHONY: run-tests test test-all $(TEST_TARGETS)

benchmark:
	@go test -tags=test -mod=readonly -bench=. $(PACKAGES_NOSIMULATION)

.PHONY: benchmark

###############################################################################
###                                Linting                                  ###
###############################################################################
golangci_lint_cmd=golangci-lint
golangci_version=v2.1.6

lint: lint-go lint-python lint-contracts

lint-go:
	@echo "--> Running linter"
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(golangci_version)
	@$(golangci_lint_cmd) run --timeout=10m

lint-python:
	find . -name "*.py" -type f -not -path "*/node_modules/*" | xargs pylint
	flake8

lint-contracts:
	solhint contracts/**/*.sol

lint-fix:
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(golangci_version)
	@$(golangci_lint_cmd) run --timeout=10m --fix

lint-fix-contracts:
	solhint --fix contracts/**/*.sol

.PHONY: lint lint-fix lint-contracts lint-go lint-python

format: format-go format-python format-shell

format-go:
	find . -name '*.go' -type f -not -path "./vendor*" -not -path "*.git*" -not -path "./client/docs/statik/statik.go" -not -name '*.pb.go' -not -name '*.pb.gw.go' -not -name '*.pulsar.go' | xargs gofumpt -w -l

format-python: format-isort format-black

format-black:
	find . -name '*.py' -type f -not -path "*/node_modules/*" | xargs black

format-isort:
	find . -name '*.py' -type f -not -path "*/node_modules/*" | xargs isort

format-shell:
	shfmt -l -w .

.PHONY: format format-go format-python format-black format-isort format-go

###############################################################################
###                                Protobuf                                 ###
###############################################################################

protoVer=0.14.0
protoImageName=ghcr.io/cosmos/proto-builder:$(protoVer)
protoImage=$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace --user 0 $(protoImageName)

protoLintVer=0.44.0
protoLinterImage=yoheimuta/protolint
protoLinter=$(DOCKER) run --rm -v "$(CURDIR):/workspace" --workdir /workspace --user 0 $(protoLinterImage):$(protoLintVer)

# ------
# NOTE: If you are experiencing problems running these commands, try deleting
#       the docker images and execute the desired command again.
#
proto-all: proto-format proto-lint proto-gen

proto-gen:
	@echo "generating implementations from Protobuf files"
	@$(protoImage) sh ./scripts/generate_protos.sh
	@$(protoImage) sh ./scripts/generate_protos_pulsar.sh

proto-format:
	@echo "formatting Protobuf files"
	@$(protoImage) find ./ -name *.proto -exec clang-format -i {} \;

proto-lint:
	@echo "linting Protobuf files"
	@$(protoImage) buf lint --error-format=json
	@$(protoLinter) lint ./proto

proto-check-breaking:
	@echo "checking Protobuf files for breaking changes"
	@$(protoImage) buf breaking --against $(HTTPS_GIT)#branch=main

.PHONY: proto-all proto-gen proto-format proto-lint proto-check-breaking

###############################################################################
###                                Releasing                                ###
###############################################################################

PACKAGE_NAME:=github.com/cosmos/evm
GOLANG_CROSS_VERSION  = v1.22
GOPATH ?= '$(HOME)/go'
release-dry-run:
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-v ${GOPATH}/pkg:/go/pkg \
		-w /go/src/$(PACKAGE_NAME) \
		ghcr.io/goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		--clean --skip validate --skip publish --snapshot

release:
	@if [ ! -f ".release-env" ]; then \
		echo "\033[91m.release-env is required for release\033[0m";\
		exit 1;\
	fi
	docker run \
		--rm \
		--privileged \
		-e CGO_ENABLED=1 \
		--env-file .release-env \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-v `pwd`:/go/src/$(PACKAGE_NAME) \
		-w /go/src/$(PACKAGE_NAME) \
		ghcr.io/goreleaser/goreleaser-cross:${GOLANG_CROSS_VERSION} \
		release --clean --skip validate

.PHONY: release-dry-run release

###############################################################################
###                        Compile Solidity Contracts                       ###
###############################################################################

# Install the necessary dependencies, compile the solidity contracts found in the
# Cosmos EVM repository and then clean up the contracts data.
contracts-all: contracts-compile contracts-clean

# Clean smart contract compilation artifacts, dependencies and cache files
contracts-clean:
	@echo "Cleaning up the contracts directory..."
	@python3 ./scripts/compile_smart_contracts/compile_smart_contracts.py --clean

# Compile the solidity contracts found in the Cosmos EVM repository.
contracts-compile:
	@echo "Compiling smart contracts..."
	@python3 ./scripts/compile_smart_contracts/compile_smart_contracts.py --compile

# Add a new solidity contract to be compiled
contracts-add:
	@echo "Adding a new smart contract to be compiled..."
	@python3 ./scripts/compile_smart_contracts/compile_smart_contracts.py --add $(CONTRACT)
