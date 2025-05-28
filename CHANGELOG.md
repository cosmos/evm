# CHANGELOG

## UNRELEASED

### DEPENDENCIES

- [\#31](https://github.com/cosmos/evm/pull/31) Migrated example_chain to evmd
- Migrated evmos/go-ethereum to cosmos/go-ethereum
- Migrated evmos/cosmos-sdk to cosmos/cosmos-sdk
- [\#95](https://github.com/cosmos/evm/pull/95) Bump up ibc-go from v8 to v10

### BUG FIXES

- Fixed example chain's cmd by adding NoOpEVMOptions to tmpApp in root.go
- Added RPC support for `--legacy` transactions (Non EIP-1559)

### IMPROVEMENTS

- [\#183](https://github.com/cosmos/evm/pull/183) Enforce `msg.sender == requester` on
all precompiles (no more proxy calls)

### FEATURES

- [\#69](https://github.com/cosmos/evm/pull/69) Add new `x/precisebank` module with bank decimal extension for EVM usage.
- [\#84](https://github.com/cosmos/evm/pull/84) permissionless erc20 registration to cosmos coin conversion

### STATE BREAKING

- Refactored evmos/os into cosmos/evm
- Renamed x/evm to x/vm
- Renamed protobuf files from evmos to cosmos org
- [\#83](https://github.com/cosmos/evm/pull/83) Remove base fee v1 from x/feemarket
- [\#93](https://github.com/cosmos/evm/pull/93) Remove legacy subspaces
- [\#95](https://github.com/cosmos/evm/pull/95) Replaced erc20/ with erc20 in native ERC20 denoms prefix for IBC v2
- [\#62](https://github.com/cosmos/evm/pull/62) Remove x/authz dependency from precompiles

### API-Breaking

- Refactored evmos/os into cosmos/evm
- Renamed x/evm to x/vm
- Renamed protobuf files from evmos to cosmos org
- [\#95](https://github.com/cosmos/evm/pull/95) Updated ics20 precompile to use Denom instead of DenomTrace for IBC v2
- [\#183](https://github.com/cosmos/evm/pull/183) **evidence precompile**
    - `SubmitEvidence` now takes the `submitter` address as its first argument (was previously implicit),
and will revert if not called directly by that EOA.
