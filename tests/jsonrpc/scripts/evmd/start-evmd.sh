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

# Initialize single-node testnet using standard test keys (complete local_node.sh setup)
echo -e "${GREEN}Initializing single-node testnet with complete local_node.sh configuration...${NC}"

# Set up variables (exactly as in local_node.sh)
KEYRING="test"
KEYALGO="eth_secp256k1"
CHAINDIR="$DATA_DIR"
GENESIS="$CHAINDIR/config/genesis.json"
TMP_GENESIS="$CHAINDIR/config/tmp_genesis.json"
CONFIG_TOML="$CHAINDIR/config/config.toml"
APP_TOML="$CHAINDIR/config/app.toml"
BASEFEE=10000000

# Standard test keys (same as local_node.sh)
VAL_KEY="mykey"
VAL_MNEMONIC="gesture inject test cycle original hollow east ridge hen combine junk child bacon zero hope comfort vacuum milk pitch cage oppose unhappy lunar seat"

USER1_KEY="dev0"
USER1_MNEMONIC="copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom"

USER2_KEY="dev1"
USER2_MNEMONIC="maximum display century economy unlock van census kite error heart snow filter midnight usage egg venture cash kick motor survey drastic edge muffin visual"

USER3_KEY="dev2"
USER3_MNEMONIC="will wear settle write dance topic tape sea glory hotel oppose rebel client problem era video gossip glide during yard balance cancel file rose"

USER4_KEY="dev3"
USER4_MNEMONIC="doll midnight silk carpet brush boring pluck office gown inquiry duck chief aim exit gain never tennis crime fragile ship cloud surface exotic patch"

# Initialize using single Docker container with initialization script
echo -e "${GREEN}Initializing chain with single Docker container...${NC}"

docker run --rm --privileged -v "$DATA_DIR:/data" --user root --entrypoint="" cosmos/evmd bash -c "
    # Initialize chain
    echo '$VAL_MNEMONIC' | evmd init localtestnet -o --chain-id '$CHAIN_ID' --recover --home /data
    
    # Set client config
    evmd config set client chain-id '$CHAIN_ID' --home /data
    evmd config set client keyring-backend '$KEYRING' --home /data
    
    # Import keys from mnemonics
    echo '$VAL_MNEMONIC' | evmd keys add '$VAL_KEY' --recover --keyring-backend '$KEYRING' --algo '$KEYALGO' --home /data
    echo '$USER1_MNEMONIC' | evmd keys add '$USER1_KEY' --recover --keyring-backend '$KEYRING' --algo '$KEYALGO' --home /data  
    echo '$USER2_MNEMONIC' | evmd keys add '$USER2_KEY' --recover --keyring-backend '$KEYRING' --algo '$KEYALGO' --home /data
    echo '$USER3_MNEMONIC' | evmd keys add '$USER3_KEY' --recover --keyring-backend '$KEYRING' --algo '$KEYALGO' --home /data
    echo '$USER4_MNEMONIC' | evmd keys add '$USER4_KEY' --recover --keyring-backend '$KEYRING' --algo '$KEYALGO' --home /data
"

# Configure genesis file using jq directly on host
echo -e "${GREEN}Configuring genesis file...${NC}"
# Change parameter token denominations to desired value
jq '.app_state["staking"]["params"]["bond_denom"]="atest"' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"
jq '.app_state["gov"]["deposit_params"]["min_deposit"][0]["denom"]="atest"' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"
jq '.app_state["gov"]["params"]["min_deposit"][0]["denom"]="atest"' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"
jq '.app_state["gov"]["params"]["expedited_min_deposit"][0]["denom"]="atest"' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"
jq '.app_state["evm"]["params"]["evm_denom"]="atest"' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"
jq '.app_state["mint"]["params"]["mint_denom"]="atest"' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"

# Add default token metadata to genesis
jq '.app_state["bank"]["denom_metadata"]=[{"description":"The native staking token for evmd.","denom_units":[{"denom":"atest","exponent":0,"aliases":["attotest"]},{"denom":"test","exponent":18,"aliases":[]}],"base":"atest","display":"test","name":"Test Token","symbol":"TEST","uri":"","uri_hash":""}]' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"

# Enable precompiles in EVM params
jq '.app_state["evm"]["params"]["active_static_precompiles"]=["0x0000000000000000000000000000000000000100","0x0000000000000000000000000000000000000400","0x0000000000000000000000000000000000000800","0x0000000000000000000000000000000000000801","0x0000000000000000000000000000000000000802","0x0000000000000000000000000000000000000803","0x0000000000000000000000000000000000000804","0x0000000000000000000000000000000000000805", "0x0000000000000000000000000000000000000806", "0x0000000000000000000000000000000000000807"]' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"

# Set EVM config
jq '.app_state["evm"]["params"]["evm_denom"]="atest"' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"

# Enable native denomination as a token pair for STRv2
jq '.app_state.erc20.native_precompiles=["0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE"]' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"
jq '.app_state.erc20.token_pairs=[{contract_owner:1,erc20_address:"0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE",denom:"atest",enabled:true}]' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"

# Set gas limit in genesis
jq '.consensus.params.block.max_gas="10000000"' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"

# Add genesis accounts and generate validator transaction
echo -e "${GREEN}Setting up genesis accounts and validator...${NC}"

docker run --rm --privileged -v "$DATA_DIR:/data" --user root --entrypoint="" cosmos/evmd bash -c "
    # Allocate genesis accounts
    evmd genesis add-genesis-account '$VAL_KEY' 100000000000000000000000000atest --keyring-backend '$KEYRING' --home /data
    evmd genesis add-genesis-account '$USER1_KEY' 1000000000000000000000atest --keyring-backend '$KEYRING' --home /data
    evmd genesis add-genesis-account '$USER2_KEY' 1000000000000000000000atest --keyring-backend '$KEYRING' --home /data
    evmd genesis add-genesis-account '$USER3_KEY' 1000000000000000000000atest --keyring-backend '$KEYRING' --home /data
    evmd genesis add-genesis-account '$USER4_KEY' 1000000000000000000000atest --keyring-backend '$KEYRING' --home /data
    
    # Generate and collect validator transaction
    evmd genesis gentx '$VAL_KEY' 1000000000000000000000atest --gas-prices '${BASEFEE}atest' --keyring-backend '$KEYRING' --chain-id '$CHAIN_ID' --home /data
    evmd genesis collect-gentxs --home /data
    evmd genesis validate-genesis --home /data
"

# Configure node settings using Docker
echo -e "${GREEN}Configuring node settings...${NC}"

docker run --rm --privileged -v "$DATA_DIR:/data" --user root --entrypoint="" cosmos/evmd bash -c "
    # Configure consensus timeouts for faster block times (500ms block time)
    sed -i 's/timeout_propose = \"3s\"/timeout_propose = \"1s\"/g' /data/config/config.toml
    sed -i 's/timeout_propose_delta = \"500ms\"/timeout_propose_delta = \"100ms\"/g' /data/config/config.toml
    sed -i 's/timeout_prevote = \"1s\"/timeout_prevote = \"300ms\"/g' /data/config/config.toml
    sed -i 's/timeout_prevote_delta = \"500ms\"/timeout_prevote_delta = \"100ms\"/g' /data/config/config.toml
    sed -i 's/timeout_precommit = \"1s\"/timeout_precommit = \"300ms\"/g' /data/config/config.toml
    sed -i 's/timeout_precommit_delta = \"500ms\"/timeout_precommit_delta = \"100ms\"/g' /data/config/config.toml
    sed -i 's/timeout_commit = \"5s\"/timeout_commit = \"500ms\"/g' /data/config/config.toml
    sed -i 's/timeout_broadcast_tx_commit = \"10s\"/timeout_broadcast_tx_commit = \"5s\"/g' /data/config/config.toml
    
    # Enable prometheus metrics and all APIs for dev node
    sed -i 's/prometheus = false/prometheus = true/' /data/config/config.toml
    sed -i 's/prometheus-retention-time = 0/prometheus-retention-time = 1000000000000/g' /data/config/app.toml
    sed -i 's/prometheus-retention-time  = \"0\"/prometheus-retention-time = \"1000000000000\"/g' /data/config/app.toml
    sed -i 's/enabled = false/enabled = true/g' /data/config/app.toml
    sed -i 's/enable = false/enable = true/g' /data/config/app.toml
    
    # Configure JSON-RPC for external access
    sed -i 's/address = \"127.0.0.1:8545\"/address = \"0.0.0.0:8545\"/' /data/config/app.toml
    sed -i 's/ws-address = \"127.0.0.1:8546\"/ws-address = \"0.0.0.0:8546\"/' /data/config/app.toml
    
    # Change proposal periods for fast testing
    sed -i 's/\"max_deposit_period\": \"172800s\"/\"max_deposit_period\": \"30s\"/g' /data/config/genesis.json
    sed -i 's/\"voting_period\": \"172800s\"/\"voting_period\": \"30s\"/g' /data/config/genesis.json
    sed -i 's/\"expedited_voting_period\": \"86400s\"/\"expedited_voting_period\": \"15s\"/g' /data/config/genesis.json
    
    # Set pruning to nothing to preserve all blocks for debug APIs
    sed -i 's/pruning = \"default\"/pruning = \"nothing\"/g' /data/config/app.toml
    sed -i 's/pruning-keep-recent = \"0\"/pruning-keep-recent = \"0\"/g' /data/config/app.toml
    sed -i 's/pruning-interval = \"0\"/pruning-interval = \"0\"/g' /data/config/app.toml
"

echo -e "${GREEN}Configuration completed${NC}"

# Start the evmd container
echo -e "${GREEN}Starting evmd container...${NC}"
CONTAINER_ID=$(docker run -d \
    --name "$CONTAINER_NAME" \
    --rm \
    --user root \
    -p 8545:8545 \
    -p 8546:8546 \
    -p 26657:26657 \
    -p 1317:1317 \
    -p 9090:9090 \
    -e ID=0 \
    -v "$DATA_DIR:/data" \
    cosmos/evmd \
    start \
    --home /data \
    --minimum-gas-prices=0.0001atest \
    --json-rpc.api eth,txpool,personal,net,debug,web3 \
    --json-rpc.address 0.0.0.0:8545 \
    --json-rpc.ws-address 0.0.0.0:8546 \
    --json-rpc.enable-profiling \
    --keyring-backend test \
    --chain-id "$CHAIN_ID" 2>&1)

# Check if docker run command succeeded
if [ $? -ne 0 ]; then
    echo -e "${RED}Error: Failed to start Docker container${NC}"
    echo -e "${YELLOW}Docker error output:${NC}"
    echo "$CONTAINER_ID"
    exit 1
fi

echo "Container started with ID: $CONTAINER_ID"

# Wait for the node to start with progressive checking
echo -e "${GREEN}Waiting for node to start...${NC}"

# Wait up to 60 seconds for the container to be running
for i in {1..12}; do
    echo "Checking container status (attempt $i/12)..."
    
    # Check if container exists and get its status
    if docker container inspect "$CONTAINER_ID" >/dev/null 2>&1; then
        CONTAINER_STATUS=$(docker inspect "$CONTAINER_ID" --format='{{.State.Status}}' 2>/dev/null)
        echo "Container status: $CONTAINER_STATUS"
        
        if [ "$CONTAINER_STATUS" = "running" ]; then
            echo -e "${GREEN}Container is running successfully!${NC}"
            break
        elif [ "$CONTAINER_STATUS" = "exited" ] || [ "$CONTAINER_STATUS" = "dead" ]; then
            echo -e "${RED}Container failed to start properly${NC}"
            echo -e "${YELLOW}Container logs:${NC}"
            docker logs "$CONTAINER_ID" 2>&1 || echo "No logs available"
            echo -e "${YELLOW}Container exit code:${NC}"
            docker inspect "$CONTAINER_ID" --format='{{.State.ExitCode}}' 2>/dev/null || echo "Cannot get exit code"
            exit 1
        fi
    else
        echo -e "${RED}Error: Container not found${NC}"
        exit 1
    fi
    
    # Wait 5 seconds before next check (total 60 seconds max)
    if [ $i -lt 12 ]; then
        echo "Waiting 5 seconds before next check..."
        sleep 5
    fi
done

# Give the container a moment to fully stabilize after confirming it's running
sleep 2

# Double-check container is still running (since we confirmed it was running in the loop above)
FINAL_STATUS=$(docker inspect "$CONTAINER_ID" --format='{{.State.Status}}' 2>/dev/null || echo "unknown")
if [ "$FINAL_STATUS" != "running" ]; then
    echo -e "${RED}Error: Container stopped running after startup (status: $FINAL_STATUS)${NC}"
    echo -e "${YELLOW}Final container logs:${NC}"
    docker logs "$CONTAINER_ID" 2>&1 || echo "No logs available"
    echo -e "${YELLOW}Container exit code:${NC}"
    docker inspect "$CONTAINER_ID" --format='{{.State.ExitCode}}' 2>/dev/null || echo "Cannot get exit code"
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
