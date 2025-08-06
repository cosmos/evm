#!/bin/bash

# Start geth node for JSON-RPC compatibility testing

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../.." && pwd)"

# Configuration
CONTAINER_NAME="geth-jsonrpc-test"
GETH_IMAGE="ethereum/client-go:v1.15.10"
DATA_DIR="$PROJECT_ROOT/tests/jsonrpc/.geth-data"
GENESIS_FILE="$PROJECT_ROOT/tests/jsonrpc/geth_genesis.json"
CHAIN_ID=4221
NETWORK_ID=4221

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting geth for JSON-RPC testing...${NC}"

# Pull geth image if not present
if ! docker image inspect "$GETH_IMAGE" >/dev/null 2>&1; then
    echo -e "${YELLOW}Pulling geth Docker image...${NC}"
    docker pull "$GETH_IMAGE"
fi

# Stop existing container if running
if docker container inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
    echo -e "${YELLOW}Stopping existing container...${NC}"
    docker stop "$CONTAINER_NAME" >/dev/null 2>&1 || true
    docker rm "$CONTAINER_NAME" >/dev/null 2>&1 || true
fi

# Clean up existing data
if [ -d "$DATA_DIR" ]; then
    echo -e "${YELLOW}Cleaning up existing geth data...${NC}"
    rm -rf "$DATA_DIR"
fi

# Create data directory
mkdir -p "$DATA_DIR"

# Check if genesis file exists
if [ ! -f "$GENESIS_FILE" ]; then
    echo -e "${RED}Error: Genesis file not found at $GENESIS_FILE${NC}"
    echo -e "${YELLOW}Please run the genesis converter first to create geth_genesis.json${NC}"
    exit 1
fi

# Initialize geth with custom genesis
echo -e "${GREEN}Initializing geth with converted genesis...${NC}"
docker run --rm \
    -v "$DATA_DIR:/data" \
    -v "$GENESIS_FILE:/genesis.json" \
    "$GETH_IMAGE" \
    --datadir /data \
    init /genesis.json

# Start geth container
echo -e "${GREEN}Starting geth container...${NC}"
docker run -d \
    --name "$CONTAINER_NAME" \
    --rm \
    -p 8547:8545 \
    -p 8548:8546 \
    -p 30303:30303 \
    -v "$DATA_DIR:/data" \
    "$GETH_IMAGE" \
    --datadir /data \
    --networkid $NETWORK_ID \
    --http \
    --http.addr 0.0.0.0 \
    --http.port 8545 \
    --http.api eth,net,web3,personal,txpool,debug,admin,miner \
    --http.corsdomain "*" \
    --ws \
    --ws.addr 0.0.0.0 \
    --ws.port 8546 \
    --ws.api eth,net,web3,personal,txpool,debug,admin,miner \
    --ws.origins "*" \
    --allow-insecure-unlock \
    --verbosity 3

# Wait for geth to start
echo -e "${GREEN}Waiting for geth to start...${NC}"
sleep 10

# Check if container is running
if ! docker container inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
    echo -e "${RED}Error: Container failed to start${NC}"
    exit 1
fi

echo -e "${GREEN}geth started successfully!${NC}"
echo -e "${YELLOW}Endpoints:${NC}"
echo -e "  JSON-RPC: http://localhost:8547"
echo -e "  WebSocket: ws://localhost:8548"
echo -e "  Chain ID: $CHAIN_ID"
echo -e "  Network ID: $NETWORK_ID"
echo
echo -e "${YELLOW}To view logs: docker logs -f $CONTAINER_NAME${NC}"
echo -e "${YELLOW}To stop: $SCRIPT_DIR/stop-geth.sh${NC}"