#!/bin/bash

# JSON-RPC Compatibility Test Runner with Docker Image Optimization
# This script handles Docker image building with content-based caching

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
JSONRPC_DIR="$PROJECT_ROOT/tests/jsonrpc"

echo "🔍 Checking Docker image requirements..."

# Check evmd image and build if needed
if ! docker image inspect cosmos/evmd >/dev/null 2>&1; then
    echo "📦 Building cosmos/evmd image..."
    make -C "$PROJECT_ROOT" localnet-build-env
else
    echo "✓ cosmos/evmd image already exists, skipping build"
fi

# Calculate simulator content hash
echo "📊 Calculating simulator content hash..."
SIMULATOR_HASH=$(find "$JSONRPC_DIR/simulator" -type f \( -name "*.go" -o -name "go.mod" -o -name "go.sum" \) -exec sha256sum {} \; | sort | sha256sum | cut -d' ' -f1)

# Check if simulator image with this hash already exists
USE_EXISTING_IMAGE=false
if docker image inspect "simulator-compat:$SIMULATOR_HASH" >/dev/null 2>&1; then
    echo "✓ Simulator image with hash $SIMULATOR_HASH already exists, skipping build"
    USE_EXISTING_IMAGE=true
    # Temporarily modify docker-compose.yml to use existing image
    cp "$JSONRPC_DIR/docker-compose.yml" "$JSONRPC_DIR/docker-compose.yml.bak"
    sed -i.tmp \
        -e 's|build:|#build:|g' \
        -e 's|context: ../../|#context: ../../|g' \
        -e 's|dockerfile: tests/jsonrpc/Dockerfile|#dockerfile: tests/jsonrpc/Dockerfile|g' \
        "$JSONRPC_DIR/docker-compose.yml"
    sed -i.tmp2 "s|container_name: simulator-compat-test|image: simulator-compat:$SIMULATOR_HASH\n    container_name: simulator-compat-test|" "$JSONRPC_DIR/docker-compose.yml"
    rm -f "$JSONRPC_DIR/docker-compose.yml.tmp" "$JSONRPC_DIR/docker-compose.yml.tmp2"
else
    echo "📦 Building simulator image with hash $SIMULATOR_HASH..."
    # Ensure docker-compose.yml is set up for building
    if [ -f "$JSONRPC_DIR/docker-compose.yml.bak" ]; then
        cp "$JSONRPC_DIR/docker-compose.yml.bak" "$JSONRPC_DIR/docker-compose.yml"
    fi
fi

# Initialize evmd data
echo "🔧 Initializing evmd test data..."

# Ensure the directory exists and has correct permissions  
mkdir -p "$JSONRPC_DIR/.evmd-compat"
chmod 777 "$JSONRPC_DIR/.evmd-compat"

# Run evmd init with root user
docker run --rm --privileged --user root \
    -v "$JSONRPC_DIR/.evmd-compat:/data" cosmos/evmd \
    testnet init-files --validator-count 1 -o /data \
    --starting-ip-address 192.168.10.2 --keyring-backend=test \
    --chain-id=local-4221 --use-docker=true

# Run the compatibility tests - only use --build if we need to build new image
echo "🚀 Running JSON-RPC compatibility tests..."
if [ "$USE_EXISTING_IMAGE" = "true" ]; then
    echo "Using existing simulator image, skipping build..."
    cd "$JSONRPC_DIR" && docker compose up --abort-on-container-exit
else
    echo "Building new simulator image..."
    cd "$JSONRPC_DIR" && docker compose up --build --abort-on-container-exit
fi

# Tag the newly built simulator image with content hash if it was built
if docker image inspect jsonrpc_simulator >/dev/null 2>&1 && ! docker image inspect "simulator-compat:$SIMULATOR_HASH" >/dev/null 2>&1; then
    echo "🏷️  Tagging simulator image with content hash $SIMULATOR_HASH..."
    docker tag jsonrpc_simulator "simulator-compat:$SIMULATOR_HASH"
fi

# Restore original docker-compose.yml
if [ -f "$JSONRPC_DIR/docker-compose.yml.bak" ]; then
    echo "🔄 Restoring original docker-compose.yml..."
    mv "$JSONRPC_DIR/docker-compose.yml.bak" "$JSONRPC_DIR/docker-compose.yml"
fi

echo "✅ JSON-RPC compatibility test completed!"