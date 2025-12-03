BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
COMMIT := $(shell git log -1 --format='%H')
APPNAME := evm
BUILDDIR ?= $(CURDIR)/build
DOCKER := $(shell which docker)

# don't override user values
ifeq (,$(VERSION))
  VERSION := $(shell git describe --exact-match 2>/dev/null)
  # if VERSION is empty, then populate it with branch's name and raw commit hash
  ifeq (,$(VERSION))
    VERSION := $(BRANCH)-$(COMMIT)
  endif
endif

help: ## List Commands
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: help

check-all: ## Convenience to make sure everything is all good
#	@$(MAKE) proto-all # Uncomment if we add custom modules
	@$(MAKE) build
	@$(MAKE) test-unit
	@$(MAKE) lint
	@$(MAKE) e2e-contracts
	@$(MAKE) test-e2e

.PHONY: all

################################################################################
##                                   Build                                    ##
################################################################################

# forces Go to use its pure Go implementation of the DNS resolver
build_tags = netgo 
build_tags += $(BUILD_TAGS)
build_tags := $(strip $(build_tags))

whitespace :=
empty = $(whitespace) $(whitespace)
comma := ,
build_tags_comma_sep := $(subst $(empty),$(comma),$(build_tags))

# process linker flags

# TODO: Figure out if '-s -w' is still needed 
# flags '-s -w' resolves an issue with xcode 16 and signing of go binaries
# ref: https://github.com/golang/go/issues/63997
ldflags = -X github.com/cosmos/cosmos-sdk/version.Name=$(APPNAME) \
		  -X github.com/cosmos/cosmos-sdk/version.AppName=$(APPNAME) \
		  -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION) \
		  -X github.com/cosmos/cosmos-sdk/version.Commit=$(COMMIT) \
		  -X "github.com/cosmos/cosmos-sdk/version.BuildTags=$(build_tags_comma_sep)" \
		  -s -w

ifeq ($(LINK_STATICALLY),true)
	ldflags += -linkmode=external -extldflags "-Wl,-z,muldefs -static"
endif
ldflags += $(LDFLAGS)
ldflags := $(strip $(ldflags))

BUILD_FLAGS := -tags "$(build_tags_comma_sep)" -ldflags '$(ldflags)' -trimpath

clean: ## Clean build artifacts
	rm -rf $(BUILDDIR)/ artifacts/

build: go.sum ## Build chain binary
	@echo "Building chain binary"
	go build -mod=readonly $(BUILD_FLAGS) -o $(BUILDDIR)/evmd ./cmd/evmd



.PHONY: build clean

################################################################################
##                                   Test                                     ##
################################################################################

test-unit: ## Run unit tests
	@echo Running unit tests...
	@go test -mod=readonly -v -timeout 30m ./...

test-race: ## Run unit tests with race condition reporting
	@echo Running unit tests with race condition reporting...
	@go test -mod=readonly -v -race -timeout 30m ./...

test-e2e: ## Run e2e tests  
	@echo Running e2e tests...
	@cd e2e && go test -parallel 8 -mod=readonly -v -count=1 -timeout 30m .

.PHONY: test-unit test-race

################################################################################
##                                   Install                                  ##
################################################################################

install: ## Install dependencies
	@echo "--> ensure dependencies have not been modified"
	@go mod verify
	@echo "--> installing $(APPNAME)d"
	@go install $(BUILD_FLAGS) -mod=readonly ./cmd/$(APPNAME)d

.PHONY: install

################################################################################
##                                   Protobuf                                 ##
################################################################################

protoVer=0.17.1
protoImageName=ghcr.io/cosmos/proto-builder:$(protoVer)
protoImage=$(DOCKER) run --rm -v $(CURDIR):/workspace --workdir /workspace $(protoImageName)

proto-all: proto-format proto-lint proto-gen ## Generate Protobuf files

proto-gen: ## Generate Protobuf files
	@echo "Generating Protobuf files"
	@$(protoImage) sh ./scripts/protocgen.sh
	@go mod tidy

proto-format: ## Format Protobuf files
	@echo "Formatting Protobuf files"
	@$(protoImage) find ./ -name "*.proto" -exec clang-format -i {} \;

proto-lint: ## Lint Protobuf files
	@$(protoImage) buf lint --error-format=json

.PHONY: proto-all proto-gen proto-format proto-lint

################################################################################
##                                   Linting                                  ##
################################################################################

golangci_lint_cmd=golangci-lint

lint: ## Run linter
	@echo "--> Running linter"
	@$(golangci_lint_cmd) run ./... --timeout 15m -c .golangci.yml
	@cd e2e && $(golangci_lint_cmd) run ./... --timeout 15m -c ../.golangci.yml

lint-fix: ## Run linter and fix issues
	@echo "--> Running linter and fixing issues"
	@$(golangci_lint_cmd) run ./... --fix --timeout 15m -c .golangci.yml
	@cd e2e && $(golangci_lint_cmd) run ./... --fix --timeout 15m -c ../.golangci.yml

.PHONY: lint lint-fix

################################################################################
##                          E2E Contract Artifacts                            ##
################################################################################

SOLC_IMAGE ?= ethereum/solc:0.8.28
E2E_CONTRACTS_DIR ?= e2e/contracts

e2e-contracts: ## Regenerate e2e embedded contract artifacts (ABI/BIN)
	@echo "--> Regenerating e2e contract artifacts with $(SOLC_IMAGE)"
	@docker run --rm -v $(CURDIR)/$(E2E_CONTRACTS_DIR):/contracts $(SOLC_IMAGE) --abi --bin -o /contracts --overwrite /contracts/Counter.sol /contracts/Reverter.sol

.PHONY: e2e-contracts
