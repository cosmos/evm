# Repository Guidelines

## Project Structure & Module Organization
- `evmd/`: standalone module and binary (entry: `cmd/evmd`).
- `rpc/`, `mempool/`, `types/`, `utils/`: core EVM JSON‑RPC and plumbing.
- `x/`: Cosmos SDK modules; EVM logic under `x/vm`.
- `proto/`: Protobuf definitions; generated via Docker.
- `contracts/`: Solidity sources; compiled via scripts.
- `tests/`, `evmd/tests/integration/`: unit/integration suites and helpers.
- `scripts/`, `contrib/`: tooling and localnet assets. `build/`: local outputs.

## Build, Test, and Development Commands
- `make build` | `make install`: build to `build/evmd` or install to `$GOPATH/bin`.
- `make test-unit` | `make test-all`: run unit or broader tests.
- `make test-unit-cover`: merge root+evmd coverage into `coverage.txt`.
- `go test ./evmd/tests/integration/... -tags=test`: backend integration tests.
- `make lint` | `make format`: Go, Python, Solidity, shell lint/format.
- `make proto-all`: format, lint, and generate protobuf code.
- `./local_node.sh -y` or `make localnet-start`: start a local network.

## Coding Style & Naming Conventions
- Go formatting via `gofumpt` (`make format-go`); no manual reformatting in PRs.
- Linters: `golangci-lint`, `flake8/pylint`, `solhint`, `shfmt`.
- Packages lowercase; files `snake_case.go`; exported identifiers `CamelCase`.
- Keep diffs focused; avoid unrelated refactors.

## Testing Guidelines
- Framework: Go testing; table-driven preferred; `testify` used in places.
- Place tests next to code (`*_test.go`). Integration under `tests/integration`.
- Ensure `make test-unit-cover` does not regress coverage.
- RPC refactors: run `make test-rpc-compat` when changing JSON‑RPC behavior.

## Commit & Pull Request Guidelines
- Conventional commits: `feat(scope): ...`, `fix(rpc): ...`, `refactor: ...`, `docs:`, `test:`, `chore:`, `ci:`, `perf:`, `deps:`.
- Use imperative mood; include scope when helpful; link issues/PRs.
- Behavior changes (feat/fix/refactor) should update `CHANGELOG.md`; CI enforces entries.
- PRs: clear description, linked issues, tests updated, relevant logs/RPC samples.

## Security & Configuration Tips
- Never commit secrets; `gitleaks` is configured. Run `make vulncheck`.
- Prefer `ContextWithHeight` for deterministic queries in tests.
- Pruning affects BaseFee/ABCI queries; mock `ConsensusParams` and `BlockResults` in tests when needed.

