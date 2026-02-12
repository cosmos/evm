#!/bin/bash
set -euo pipefail

echo "Building krakatoa (evmd) natively with zig cross-compilation..."

export GOOS=linux
export GOARCH=arm64
export CC="zig cc -target aarch64-linux-musl"
export CXX="zig c++ -target aarch64-linux-musl"
export CGO_ENABLED=1

make build

cp "build/evmd" "build/poa-binary-e2e"
chmod +x "build/poa-binary-e2e"

echo "Built poa-binary-e2e (linux/arm64)"
