# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Cosmos EVM is a plug-and-play solution that adds EVM (Ethereum Virtual Machine) compatibility to Cosmos SDK chains. It's a fork of evmOS maintained by Interchain Labs and the Interchain Foundation.

## Build Commands

### Building
- `make build` - Build evmd binary to build/ directory
- `make build-linux` - Cross-compile for Linux AMD64
- `make install` - Install evmd binary to $GOPATH/bin

### Running
- `./local_node.sh` - Run a local development node (CHAIN_ID defaults to 9001)
  - `-y` flag to overwrite previous chain data
  - `--no-install` to skip building
  - `--remote-debugging` for debugging support

## Test Commands

### Core Tests
- `make test` or `make test-unit` - Run unit tests
- `make test-unit-cover` - Run tests with coverage report
- `make test-evmd` - Run evmd module tests specifically
- `make test-all` - Run all tests
- `make test-race` - Run tests with race detector
- `make test-fuzz` - Run fuzz tests
- `make benchmark` - Run benchmark tests

### Specialized Tests
- `make test-solidity` - Run Solidity tests
- `make test-scripts` - Run Python script tests (uses pytest)

### Testing Single Files/Functions
- `go test -tags=test -timeout=15m -run TestSpecificFunction ./path/to/package`
- For evmd submodule: `cd evmd && go test -tags=test -timeout=15m -run TestSpecificFunction ./path/to/package`

## Lint & Format Commands

### Linting
- `make lint` - Run all linters (Go, Python, Solidity)
- `make lint-go` - Run Go linter (golangci-lint)
- `make lint-python` - Run Python linters (pylint, flake8)
- `make lint-contracts` - Run Solidity linter (solhint)
- `make lint-fix` - Fix Go linting issues
- `make lint-fix-contracts` - Fix Solidity linting issues

### Formatting
- `make format` - Format all code
- `make format-go` - Format Go code (gofumpt)
- `make format-python` - Format Python code (black, isort)

## Architecture

The project has a modular architecture with core Cosmos SDK modules extended for EVM support:

### Module Structure
- **Main module** (`github.com/cosmos/evm`) - Core EVM functionality
- **evmd submodule** (`./evmd/`) - Example chain implementation

### Core Modules (in `/x/`)
- **vm** - EVM implementation with Ethereum state management
- **erc20** - Single Token Representation v2 for unified IBC/ERC20 tokens
- **feemarket** - EIP-1559 dynamic fee market
- **precisebank** - Fractional balance support for 18-decimal EVM tokens
- **ibc** - IBC integration with EVM support

### Key Components
- **Precompiles** (`/precompiles/`) - Solidity interfaces to Cosmos SDK functionality:
  - Bank, Bech32, Distribution, ERC20, Evidence, Gov, ICS20, Slashing, Staking, WERC20
- **Ante Handlers** (`/ante/`) - Transaction validation for both Cosmos and EVM transactions
- **JSON-RPC** (`/rpc/`) - Ethereum-compatible RPC endpoints
- **Crypto** (`/crypto/`) - ethsecp256k1 and secp256r1 implementations

### Development Notes

1. **Go Version**: Requires Go 1.23.8+
2. **Module Structure**: Uses Go workspaces with separate go.mod files for main module and evmd
3. **Testing**: Always verify test commands exist before running (check Makefile or scripts/)
4. **Linting**: Run linters before committing changes
5. **Coverage**: Coverage reports exclude test files, protobuf generated files, and module boilerplate

### Key Technical Details

- **Chain ID Format**: `cosmos_{subchain_id}-{evm_chain_id}` (e.g., `cosmos_262144-1`)
- **Token Precision**: Native Cosmos tokens (6 decimals) converted to EVM's 18 decimals via precisebank
- **State Management**: Dual state model supporting both Cosmos SDK and EVM state
- **Transaction Types**: Supports both Cosmos SDK messages and Ethereum transactions
- **EIP Support**: EIP-712 signing, EIP-1559 fee market, and more

### Common Development Tasks

When implementing features:
1. Check existing precompiles for similar functionality
2. Look at ante handlers for transaction validation patterns
3. Review x/vm for EVM-specific logic
4. Use x/erc20 patterns for token integration
5. Reference integration tests in tests/ for examples