# CHANGELOG

## UNRELEASED

### DEPENDENCIES

### BUG FIXES

- [\#471](https://github.com/cosmos/evm/pull/471) Notify new block for mempool in time.
- [\#492](https://github.com/cosmos/evm/pull/492) Duplicate case switch to avoid empty execution block

### IMPROVEMENTS

- [\#467](https://github.com/cosmos/evm/pull/467) Replace GlobalEVMMempool by passing to JSONRPC on initiate.
- [\#352](https://github.com/cosmos/evm/pull/352) Remove the creation of a Geth EVM instance, stateDB during the AnteHandler balance check.

### FEATURES

- [\#346](https://github.com/cosmos/evm/pull/346) Add eth_createAccessList method and implementation

### STATE BREAKING

### API-BREAKING

- [\#477](https://github.com/cosmos/evm/pull/477) Refactor precompile constructors to accept keeper interfaces instead of concrete implementations, breaking the existing `NewPrecompile` function signatures.

## v0.4.1

### DEPENDENCIES

- [\#459](https://github.com/cosmos/evm/pull/459) Update `cosmossdk.io/log` to `v1.6.1` to support Go `v1.25.0+`.
- [\#435](https://github.com/cosmos/evm/pull/435) Update Cosmos SDK to `v0.53.4` and CometBFT to `v0.38.18`.

### BUG FIXES

- [\#179](https://github.com/cosmos/evm/pull/179) Fix compilation error in server/start.go
- [\#245](https://github.com/cosmos/evm/pull/245) Use PriorityMempool with signer extractor to prevent missing signers error in tx execution
- [\#289](https://github.com/cosmos/evm/pull/289) Align revert reason format with go-ethereum (return hex-encoded result)
- [\#291](https://github.com/cosmos/evm/pull/291) Use proper address codecs in precompiles for bech32/hex conversion
- [\#296](https://github.com/cosmos/evm/pull/296) Add sanity checks to trace_tx RPC endpoint
- [\#316](https://github.com/cosmos/evm/pull/316) Fix estimate gas to handle missing fields for new transaction types
- [\#330](https://github.com/cosmos/evm/pull/330) Fix error propagation in BlockHash RPCs and address test flakiness
- [\#332](https://github.com/cosmos/evm/pull/332) Fix non-determinism in state transitions
- [\#350](https://github.com/cosmos/evm/pull/350) Fix p256 precompile test flakiness
- [\#376](https://github.com/cosmos/evm/pull/376) Fix precompile initialization for local node development script
- [\#384](https://github.com/cosmos/evm/pull/384) Fix debug_traceTransaction RPC failing with block height mismatch errors
- [\#441](https://github.com/cosmos/evm/pull/441) Align precompiles map with available static check to Prague.
- [\#452](https://github.com/cosmos/evm/pull/452) Cleanup unused cancel function in filter.
- [\#454](https://github.com/cosmos/evm/pull/454) Align multi decode functions instead of string contains check in HexAddressFromBech32String.
- [\#468](https://github.com/cosmos/evm/pull/468) Add pagination flags to `token-pairs` to improve query flexibility.

### IMPROVEMENTS

- [\#294](https://github.com/cosmos/evm/pull/294) Enforce single EVM transaction per Cosmos transaction for security
- [\#299](https://github.com/cosmos/evm/pull/299) Update dependencies for security and performance improvements
- [\#307](https://github.com/cosmos/evm/pull/307) Preallocate EVM access_list for better performance
- [\#317](https://github.com/cosmos/evm/pull/317) Fix EmitApprovalEvent to use owner address instead of precompile address
- [\#345](https://github.com/cosmos/evm/pull/345) Fix gas cap calculation and fee rounding errors in ante handler benchmarks
- [\#347](https://github.com/cosmos/evm/pull/347) Add loop break labels for optimization
- [\#370](https://github.com/cosmos/evm/pull/370) Use larger CI runners for resource-intensive tests
- [\#373](https://github.com/cosmos/evm/pull/373) Apply security audit patches
- [\#377](https://github.com/cosmos/evm/pull/377) Apply audit-related commit 388b5c0
- [\#382](https://github.com/cosmos/evm/pull/382) Post-audit security fixes (batch 1)
- [\#388](https://github.com/cosmos/evm/pull/388) Post-audit security fixes (batch 2)
- [\#389](https://github.com/cosmos/evm/pull/389) Post-audit security fixes (batch 3)
- [\#392](https://github.com/cosmos/evm/pull/392) Post-audit security fixes (batch 5)
- [\#398](https://github.com/cosmos/evm/pull/398) Post-audit security fixes (batch 4)
- [\#442](https://github.com/cosmos/evm/pull/442) Prevent nil pointer by checking error in gov precompile FromResponse.
- [\#387](https://github.com/cosmos/evm/pull/387) (Experimental) EVM-compatible appside mempool
- [\#476](https://github.com/cosmos/evm/pull/476) Add revert error e2e tests for contract and precompile calls

### FEATURES

- [\#253](https://github.com/cosmos/evm/pull/253) Add comprehensive Solidity-based end-to-end tests for precompiles
- [\#301](https://github.com/cosmos/evm/pull/301) Add 4-node localnet infrastructure for testing multi-validator setups
- [\#304](https://github.com/cosmos/evm/pull/304) Add system test framework for integration testing
- [\#344](https://github.com/cosmos/evm/pull/344) Add txpool RPC namespace stubs in preparation for app-side mempool implementation
- [\#440](https://github.com/cosmos/evm/pull/440) Enforce app creator returning application implement AppWithPendingTxStream in build time.

### STATE BREAKING

### API-BREAKING

- [\#456](https://github.com/cosmos/evm/pull/456) Remove non–go-ethereum JSON-RPC methods to align with Geth’s surface
- [\#443](https://github.com/cosmos/evm/pull/443) Move `ante` logic from the `evmd` Go package to the `evm` package to
be exported as a library.
- [\#422](https://github.com/cosmos/evm/pull/422) Align function and package names for consistency.
- [\#305](https://github.com/cosmos/evm/pull/305) Remove evidence precompile due to lack of use cases
