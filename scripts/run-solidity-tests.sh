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

PNPM_MAJOR_VERSION="11"
if ! command -v pnpm &>/dev/null || [ "$(pnpm --version | cut -d. -f1)" != "$PNPM_MAJOR_VERSION" ]; then
	corepack enable && corepack prepare pnpm@11 --activate
fi

pnpm install
pnpm test --network cosmos "$@"
