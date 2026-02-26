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

if command -v pnpm &>/dev/null; then
	pnpm install
else
	corepack enable && corepack prepare pnpm@latest --activate
	pnpm install
fi

pnpm test -- --network cosmos "$@"
