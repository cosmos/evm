# AGENTS.md

Guidance for AI coding agents working in the Cosmos EVM repository.

## Project Overview

Cosmos EVM adds EVM compatibility to Cosmos SDK chains. It exposes Ethereum
JSON-RPC, runs Solidity contracts, and integrates Cosmos modules (IBC, staking,
bank, etc.) via precompiles. `evmd` is the example chain used for testing and
local development.

Docs: https://evm.cosmos.network/

## Repository Layout

This repo is two Go modules:

- `./` — `github.com/cosmos/evm` — library code (modules, precompiles, RPC).
- `./evmd/` — `github.com/cosmos/evm/evmd` — example chain binary and
  integration tests. Has its own `go.mod`/`go.sum`.

Top-level directories:

- `x/` — Cosmos SDK modules: `vm` (EVM), `erc20`, `feemarket`, `ibc`.
- `precompiles/` — Solidity-callable precompiles bridging into Cosmos modules
  (`bank`, `staking`, `distribution`, `gov`, `ics20`, `ics02`, `slashing`,
  `bech32`, `p256`, `callbacks`, `werc20`, `erc20`).
- `ante/` — Ante handlers for EVM and Cosmos transactions.
- `mempool/` — Custom EVM-aware mempool.
- `rpc/` — Ethereum JSON-RPC server.
- `indexer/` — Block/tx indexer backing JSON-RPC.
- `ethereum/`, `crypto/`, `encoding/`, `server/`, `client/` — supporting
  packages.
- `testutil/` — shared test helpers (network harness, factories, keyring).
- `tests/` — top-level tests: `integration/`, `jsonrpc/`, `solidity/`,
  `systemtests/`, `speedtest/`.
- `evmd/tests/integration/` — integration tests that run against a real
  `evmd` app instance.
- `proto/` — protobuf definitions; generated code lives next to the modules.
- `contracts/` — Solidity sources compiled via `scripts/compile_smart_contracts`.
- `docs/` — including `docs/migrations` for upgrade guides.

## Toolchain

- Go: see `go.mod` (currently 1.25.x).
- CGO required for builds (`CGO_ENABLED=1`).
- `golangci-lint` v2.10.1 (pinned in Makefile).
- Docker required for protobuf generation, localnet, and rpc-compat tests.
- Foundry (`forge`) required for `tests/systemtests/Counter`.
- Python (pylint, flake8, black, isort) for scripts.
- `solhint` for Solidity contracts.

## Common Commands

Run from the repo root unless noted.

Build and install:

```bash
make build          # builds ./build/evmd
make install        # installs evmd to $GOPATH/bin
make build-linux    # cross-compile linux/amd64
```

Tests:

```bash
make test-unit          # unit tests for the root module
make test-evmd          # tests under ./evmd (run from evmd/, with -race)
make test-race          # root module with -race
make test-all           # both modules
make test-unit-cover    # coverage for both modules
make test-solidity      # hardhat/forge tests
make test-system        # system tests (requires forge + builds v0.6 + current)
make test-rpc-compat    # JSON-RPC compatibility (Docker)
make benchmark
```

All Go tests use the `test` build tag (`-tags=test`). Tests under `evmd/`
must be invoked from that directory because it is a separate module:

```bash
cd evmd && go test -tags=test ./tests/integration/...
```

Lint and format:

```bash
make lint           # go + python + contracts
make lint-fix       # auto-fix Go issues
make format         # gofumpt + black + isort + shfmt
```

Protobuf and mocks (require Docker / Go tooling):

```bash
make proto-all      # format + lint + gen
make proto-gen
make mocks
```

Localnet:

```bash
make localnet-start # 4-node docker testnet
make localnet-stop
./local_node.sh     # single-node dev chain
```

## Conventions

- **Two modules, one repo.** When changing public types in the root module
  that `evmd` consumes, run tests in both modules. `go.work` is not used by
  default — be explicit with `cd evmd` for evmd-side changes.
- **Build tag `test`.** Test-only helpers and constructors live behind
  `//go:build test`. Forgetting the tag will produce confusing "undefined"
  errors.
- **No generated edits.** Do not hand-edit `*.pb.go`, `*.pb.gw.go`, or
  `*.pulsar.go`; regenerate with `make proto-gen`.
- **Signed, conventional commits.** PRs require signed commits and titles in
  [conventional commit](https://www.conventionalcommits.org/) form
  (`feat:`, `fix:`, `chore:`, `refactor:`, `test:`, `docs:`). Every PR must
  link an existing GitHub issue (see `CONTRIBUTING.md`).
- **Migrations.** Any breaking change to chain state, module params, or
  storage requires a corresponding migration and a doc entry under
  `docs/migrations`.
- **Precompile changes** must update both Go bindings and the corresponding
  Solidity ABI in `precompiles/*/abi.json` — they are checked at startup.
- **Don't add features beyond the task.** Match the change to what was
  asked; don't refactor adjacent code or add unused abstractions.

## Where to Look First

- New module wiring: `evmd/app.go`.
- EVM keeper / state transitions: `x/vm/keeper/`.
- Adding a precompile: pattern under `precompiles/<name>/` plus registration
  in `precompiles/types/` and `evmd/app.go`.
- JSON-RPC namespaces: `rpc/namespaces/ethereum/`.
- Ante handler order: `ante/`.
- Integration test patterns: `evmd/tests/integration/` and
  `testutil/integration/evm/network/`.

## Things to Avoid

- Hand-editing `coverage.txt`, `go.sum`, or vendored / generated files.
- Skipping git hooks (`--no-verify`) or amending merged commits.
- Pushing to `main` directly; PRs only.
- Touching `tests/systemtests/Counter/lib/forge-std` (git submodule).
- Running `find` from `/` — search from the repo root.
