# Precompile custom errors

Precompile error mappings are declared in Go next to the runtime code that uses
them. Solidity interfaces define the public custom-error signatures, and
`make contracts-compile` regenerates the committed ABI files for the existing
ABI diff check. There is no separate error catalog to update.

## Resolution precedence

1. A call-site mapping that has trusted runtime context, such as the ERC-6093
   balance and allowance errors.
2. The precompile's module or dependency registry.
3. The shared SDK registry inherited from `IPrecompile`.
4. `UnmappedCosmosError(string codespace, uint32 code)` for a registered Cosmos
   error that has no local mapping.
5. The existing internal fallback, such as `MsgServerFailed` or `QueryFailed`,
   for unregistered infrastructure and protocol failures.

Registered errors are identified by `(codespace, code)`. Diagnostic text and
gRPC status messages are never used as error identities or as sources for typed
Solidity arguments.

gRPC failures use an explicit `(boundary, method, status code)` declaration.
These declarations also preserve existing query methods that intentionally
return zero, empty, or default values for `NotFound`.

## Changing mappings

- Add or change module mappings in `precompiles/<module>/errors.go`.
- Add shared SDK mappings and gRPC dispositions in
  `precompiles/common/cosmos_errors.go`.
- Add context-dependent mappings at the call site before generic translation.
- Update the owning Solidity interface and regenerate ABIs with
  `make contracts-compile`.
- Add a behavior test that asserts the returned selector and verifies that the
  result does not fall through to a generic error.

New errors in an SDK or IBC dependency are not public precompile errors by
default. During a dependency upgrade, review the affected keeper, MsgServer,
and QueryServer paths and add mappings only for errors that are actually
reachable and have a stable identity at the precompile boundary.
