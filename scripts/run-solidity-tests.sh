#!/bin/bash

export GOPATH="$HOME"/go
export PATH="$PATH":"$GOPATH"/bin

# remove existing data
rm -rf "$HOME"/.tmp-evmd-solidity-tests

# used to exit on first error (any non-zero exit code)
set -e

# build evmd binary
make install

cd tests/solidity || exit

PNPM_VERSION="11.24.0"
if ! command -v pnpm &>/dev/null || [ "$(pnpm --version)" != "$PNPM_VERSION" ]; then
	npm install --global "pnpm@$PNPM_VERSION"
fi

pnpm install
pnpm test --network cosmos "$@"
