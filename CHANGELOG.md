# CHANGELOG

## UNRELEASED

### DEPENDENCIES

### IMPROVEMENTS

- [\#758](https://github.com/cosmos/evm/pull/758) Cleanup precompiles abi.json.
- [\#810](https://github.com/cosmos/evm/pull/810) Fix integration test suite to resolve lock contention problem from external app injection
- [\#811](https://github.com/cosmos/evm/pull/811) Use sdk's DefaultBondDenom for default evm denom in genesis.
- [\#823](https://github.com/cosmos/evm/pull/823) Remove authz dependency from test suite and EvmApp interface
- [\#829](https://github.com/cosmos/evm/pull/829) Seperate test app interface

### FEATURES

- [\#589](https://github.com/cosmos/evm/pull/589) Remove parallelization blockers via migration from transient to object store, refactoring of gas, indexing, and bloom utilities.
- [\#768](https://github.com/cosmos/evm/pull/768) Added ICS-02 Client Router precompile

### BUG FIXES

- [\#794](https://github.com/cosmos/evm/pull/794) Fix mempool.max-txs flag not using desired default of 0
- [\#748](https://github.com/cosmos/evm/pull/748) Fix DynamicFeeChecker in Cosmos ante handler to respect NoBaseFee feemarkets' parameter.
- [\#690](https://github.com/cosmos/evm/pull/690) Fix Ledger hardware wallet support for coin type 60.
- [\#766](https://github.com/cosmos/evm/pull/766) Align gas estimation logic with go-ethereum v1.16.3
- [\#769](https://github.com/cosmos/evm/pull/769) Fix erc20 ibc middleware to not to validate sender address format.
- [\#756](https://github.com/cosmos/evm/pull/756) Fix error message typo in NewMsgCancelProposal.
- [\#772](https://github.com/cosmos/evm/pull/772) Avoid panic on close if evm mempool not used.
- [\#774](https://github.com/cosmos/evm/pull/774) Emit proper allowance amount in erc20 event.
- [\#790](https://github.com/cosmos/evm/pull/790) fix panic in historical query due to missing EvmCoinInfo.
- [\#800](https://github.com/cosmos/evm/pull/800) Fix denom exponent validation in virtual fee deduct in vm module.
- [\#817](https://github.com/cosmos/evm/pull/817) Align GetCoinbaseAddress to handle empty proposer address in contexts like CheckTx where proposer doesn't exist.
- [\#814](https://github.com/cosmos/evm/pull/814) Fix duplicated events in post tx processor.
- [\#816](https://github.com/cosmos/evm/pull/816) Avoid nil pointer when RPC requests execute before evmCoinInfo initialization in PreBlock with defaultEvmCoinInfo fallback.
- [\#828](https://github.com/cosmos/evm/pull/828) Validate decimals before conversion to prevent panic when coininfo is missing in historical queries.

## v0.5.0

### DEPENDENCIES

### BUG FIXES

- [\#471](https://github.com/cosmos/evm/pull/471) Notify new block for mempool in time
- [\#492](https://github.com/cosmos/evm/pull/492) Duplicate case switch to avoid empty execution block
- [\#509](https://github.com/cosmos/evm/pull/509) Allow value with slashes when query token_pairs
- [\#495](https://github.com/cosmos/evm/pull/495) Allow immediate SIGINT interrupt when mempool is not empty
- [\#416](https://github.com/cosmos/evm/pull/416) Fix regression in CometBlockResultByNumber when height is 0 to use the latest block. This fixes eth_getFilterLogs RPC.
- [\#545](https://github.com/cosmos/evm/pull/545) Check if mempool is not nil before accepting nonce gap error tx.
- [\#585](https://github.com/cosmos/evm/pull/585) Use zero constructor to avoid nil pointer panic when BaseFee is 0d
- [\#591](https://github.com/cosmos/evm/pull/591) CheckTxHandler should handle "invalid nonce" tx
- [\#642](https://github.com/cosmos/evm/pull/642) "tx not found in mempool" error on chain startup
- [\#643](https://github.com/cosmos/evm/pull/643) Support for mnemonic source (file, stdin,etc) flag in key add command.
- [\#645](https://github.com/cosmos/evm/pull/645) Align precise bank keeper for correct decimal conversion in evmd.
- [\#656](https://github.com/cosmos/evm/pull/656) Fix race condition in concurrent usage of mempool StateAt and NotifyNewBlock methods.
- [\#658](https://github.com/cosmos/evm/pull/658) Fix race condition between legacypool's RemoveTx and runReorg.
- [\#687](https://github.com/cosmos/evm/pull/687) Avoid blocking node shutdown when evm indexer is enabled, log startup failures instead of using errgroup.
- [\#689](https://github.com/cosmos/evm/pull/689) Align debug addr for hex address.
- [\#668](https://github.com/cosmos/evm/pull/668) Fix panic in legacy mempool when Reset() was called with a skipped header between old and new block.
- [\#723](https://github.com/cosmos/evm/pull/723), [\#806](https://github.com/cosmos/evm/pull/806) Fix TransactionIndex in receipt generation to use actual EthTxIndex instead of loop index.
- [\#729](https://github.com/cosmos/evm/pull/729) Remove non-deterministic state mutation from EVM pre-blocker.
- [\#725](https://github.com/cosmos/evm/pull/725) Fix inconsistent block hash in json-rpc.
- [\#727](https://github.com/cosmos/evm/pull/727) Avoid nil pointer for `tx evm raw` due to uninitialized EVM coin info.
- [\#730](https://github.com/cosmos/evm/pull/730) Fix panic if evm mempool not used.
- [\#733](https://github.com/cosmos/evm/pull/733) Avoid rejecting tx with unsupported extension option for ExtensionOptionDynamicFeeTx.
- [\#736](https://github.com/cosmos/evm/pull/736) Add InitEvmCoinInfo upgrade to avoid panic when denom is not registered.
- [\#732](https://github.com/cosmos/evm/pull/732) Fix gas meter race condition in integration tests

### IMPROVEMENTS

- [\#708](https://github.com/cosmos/evm/pull/708) Add configurable testnet validator powers
- [\#698](https://github.com/cosmos/evm/pull/698) Expose mempool configuration flags and move mempool configuration in app.go to helper
- [\#538](https://github.com/cosmos/evm/pull/538) Optimize `eth_estimateGas` gRPC path: short-circuit plain transfers, add optimistic gas bound based on `MaxUsedGas`.
- [\#513](https://github.com/cosmos/evm/pull/513) Replace `TestEncodingConfig` with production `EncodingConfig` in encoding package to remove test dependencies from production code.
- [\#467](https://github.com/cosmos/evm/pull/467) Replace GlobalEVMMempool by passing to JSONRPC on initiate.
- [\#352](https://github.com/cosmos/evm/pull/352) Remove the creation of a Geth EVM instance, stateDB during the AnteHandler balance check.
- [\#496](https://github.com/cosmos/evm/pull/496) Simplify mempool instantiation by using configs instead of objects.
- [\#512](https://github.com/cosmos/evm/pull/512) Add integration test for appside mempool.
- [\#568](https://github.com/cosmos/evm/pull/568) Avoid unnecessary block notifications when the event bus is already set up.
- [\#511](https://github.com/cosmos/evm/pull/511) Minor code cleanup for `AddPrecompileFn`.
- [\#576](https://github.com/cosmos/evm/pull/576) Parse logs from the txResult.Data and avoid emitting EVM events to cosmos-sdk events.
- [\#584](https://github.com/cosmos/evm/pull/584) Fill block hash and timestamp for json rpc.
- [\#582](https://github.com/cosmos/evm/pull/582) Add block max-gas (from genesis.json) and new min-tip (from app.toml/flags) ingestion into mempool config
- [\#580](https://github.com/cosmos/evm/pull/580) add appside mempool e2e test
- [\#598](https://github.com/cosmos/evm/pull/598) Reduce number of times CreateQueryContext in mempool.
- [\#606](https://github.com/cosmos/evm/pull/606) Regenerate mock file for bank keeper related test.
- [\#609](https://github.com/cosmos/evm/pull/609) Make `erc20Keeper` optional in the EVM keeper
- [\#624](https://github.com/cosmos/evm/pull/624) Cleanup unnecessary `fix-revert-gas-refund-height`.
- [\#635](https://github.com/cosmos/evm/pull/635) Move DefaultStaticPrecompiles to /evm and allow projects to set it by default alongside the keeper.
- [\#639](https://github.com/cosmos/evm/pull/639) Remove `/types` and move types into respective folders.
- [\#630](https://github.com/cosmos/evm/pull/630) Reduce feemarket parameter loading to minimize memory allocations.
- [\#577](https://github.com/cosmos/evm/pull/577) Cleanup precompiles boilerplate code.
- [\#648](https://github.com/cosmos/evm/pull/648) Move all `ante` logic such as `NewAnteHandler` from the `evmd` package to `evm/ante` so it can be used as library functions.
- [\#659](https://github.com/cosmos/evm/pull/659) Move configs out of EVMD and deduplicate configs
- [\#664](https://github.com/cosmos/evm/pull/664) Add EIP-7702 integration test
- [\#684](https://github.com/cosmos/evm/pull/684) Add unit test cases for EIP-7702
- [\#685](https://github.com/cosmos/evm/pull/685) Add EIP-7702 e2e test
- [\#680](https://github.com/cosmos/evm/pull/680) Introduce a `StaticPrecompiles` builder
- [\#701](https://github.com/cosmos/evm/pull/701) Add address codec support to ERC20 IBC callbacks to handle hex addresses in addition to bech32 addresses.
- [\#704](https://github.com/cosmos/evm/pull/704) Fix EIP-7702 test cases
- [\#709](https://github.com/cosmos/evm/pull/709) Fix mempool e2e test
- [\#710](https://github.com/cosmos/evm/pull/710) Fix EoA-CA Identification logic
- [\#711](https://github.com/cosmos/evm/pull/711) Add debug_traceCall api
- [\#720](https://github.com/cosmos/evm/pull/720) Refactor systemtests
- [\#734](https://github.com/cosmos/evm/pull/734) Disable evm mempool if max-txs set to -1.
- [\#743](https://github.com/cosmos/evm/pull/743) Apply state overrides to eth_estimateGas api

### FEATURES

- [\#665](https://github.com/cosmos/evm/pull/665) Add EvmCodec address codec implementation
- [\#346](https://github.com/cosmos/evm/pull/346) Add eth_createAccessList method and implementation
- [\#337](https://github.com/cosmos/evm/pull/337) Support state overrides in eth_call.
- [\#502](https://github.com/cosmos/evm/pull/502) Add block time in derived logs.
- [\#633](https://github.com/cosmos/evm/pull/633) go-ethereum metrics are now emitted on a separate server. default address: 127.0.0.1:8100.
- [\#650](https://github.com/cosmos/evm/pull/650) Make staking precompile queries return the full validators' description structure.

### STATE BREAKING

### API-BREAKING

- [\#477](https://github.com/cosmos/evm/pull/477) Refactor precompile constructors to accept keeper interfaces instead of concrete implementations, breaking the existing `NewPrecompile` function signatures.
- [\#594](https://github.com/cosmos/evm/pull/594) Remove all usage of x/params
- [\#577](https://github.com/cosmos/evm/pull/577) Changed the way to create a stateful precompile based on the cmn.Precompile, change `NewPrecompile` to not return error.
- [\#661](https://github.com/cosmos/evm/pull/661) Removes evmAppOptions from the repository and moves initialization to genesis. Chains must now have a display and denom metadata set for the defined EVM denom in the bank module's metadata.

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
- [\#599](https://github.com/cosmos/evm/pull/599) Align jsonrpc apis with geth v1.16.3

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
