#!/bin/bash

# Start single evmd node for JSON-RPC compatibility testing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"

# Configuration
CONTAINER_NAME="evmd-jsonrpc-test"
DATA_DIR="$PROJECT_ROOT/tests/jsonrpc/.evmd"
VALIDATOR_COUNT=1
CHAIN_ID="local-4221"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting evmd for JSON-RPC testing...${NC}"

# Check if Docker image exists
if ! docker image inspect cosmos/evmd >/dev/null 2>&1; then
    echo -e "${RED}Error: cosmos/evmd Docker image not found${NC}"
    echo -e "${YELLOW}Please run: make localnet-build-env${NC}"
    exit 1
fi

# Stop existing container if running
if docker container inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
    echo -e "${YELLOW}Stopping existing container...${NC}"
    docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
fi

# Clean up existing data
if [ -d "$DATA_DIR" ]; then
    echo -e "${YELLOW}Cleaning up existing testnet data...${NC}"
    rm -rf "$DATA_DIR"
fi

# Initialize testnet with single node
echo -e "${GREEN}Initializing single-node testnet...${NC}"
cd "$PROJECT_ROOT"
docker run --rm \
    -v "$DATA_DIR:/data" \
    cosmos/evmd \
    testnet init-files \
    --validator-count "$VALIDATOR_COUNT" \
    -o /data \
    --starting-ip-address 192.168.10.2 \
    --keyring-backend=test \
    --chain-id="$CHAIN_ID" \
    --use-docker=true

# Start the evmd container
echo -e "${GREEN}Starting evmd container...${NC}"
docker run -d \
    --name "$CONTAINER_NAME" \
    --rm \
    -p 8545:8545 \
    -p 8546:8546 \
    -p 26657:26657 \
    -p 1317:1317 \
    -p 9090:9090 \
    -e ID=0 \
    -v "$DATA_DIR:/data" \
    cosmos/evmd

# Wait for the node to start
echo -e "${GREEN}Waiting for node to start...${NC}"
sleep 5

# Check if container is running
if ! docker container inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
    echo -e "${RED}Error: Container failed to start${NC}"
    exit 1
fi

echo -e "${GREEN}evmd started successfully!${NC}"
echo -e "${YELLOW}Endpoints:${NC}"
echo -e "  JSON-RPC: http://localhost:8545"
echo -e "  WebSocket: ws://localhost:8546"
echo -e "  Cosmos REST: http://localhost:1317"
echo -e "  Tendermint RPC: http://localhost:26657"
echo -e "  gRPC: localhost:9090"
echo
echo -e "${YELLOW}To view logs: docker logs -f $CONTAINER_NAME${NC}"
echo -e "${YELLOW}To stop: $SCRIPT_DIR/stop-evmd.sh${NC}"