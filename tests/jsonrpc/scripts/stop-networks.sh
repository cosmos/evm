#!/bin/bash

# Stop both evmd and geth nodes

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Stopping both evmd and geth...${NC}"

# Stop evmd
echo -e "${YELLOW}Stopping evmd...${NC}"
"$SCRIPT_DIR/evmd/stop-evmd.sh"

echo
echo -e "${YELLOW}Stopping geth...${NC}"
"$SCRIPT_DIR/geth/stop-geth.sh"

echo
echo -e "${GREEN}Both nodes stopped successfully${NC}"