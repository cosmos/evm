#!/bin/bash

# Stop evmd node for JSON-RPC testing

set -e

# Configuration
CONTAINER_NAME="evmd-jsonrpc-test"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Stopping evmd for JSON-RPC testing...${NC}"

# Stop container if running
if docker container inspect "$CONTAINER_NAME" >/dev/null 2>&1; then
    echo -e "${YELLOW}Stopping container...${NC}"
    docker stop "$CONTAINER_NAME" >/dev/null 2>&1
    echo -e "${GREEN}evmd stopped successfully${NC}"
else
    echo -e "${YELLOW}Container is not running${NC}"
fi