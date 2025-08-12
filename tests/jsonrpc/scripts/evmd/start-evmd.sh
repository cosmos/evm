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

# Complete initialization (mirroring local_node.sh exactly)
# Pre-create the directory structure with proper permissions to avoid Docker permission issues
echo -e "${GREEN}Creating directory structure...${NC}"
mkdir -p "$DATA_DIR/config"
mkdir -p "$DATA_DIR/data" 
mkdir -p "$DATA_DIR/keyring-test"
chmod -R 755 "$DATA_DIR"

# First initialize the chain to create directory structure
echo -e "${GREEN}Initializing chain...${NC}"
echo "$VAL_MNEMONIC" | docker run --rm -i -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd init localtestnet -o --chain-id "$CHAIN_ID" --recover --home /data

# Set client config (after init creates the directory structure)
docker run --rm -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd config set client chain-id "$CHAIN_ID" --home /data

docker run --rm -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd config set client keyring-backend "$KEYRING" --home /data

# Import keys from mnemonics
echo -e "${GREEN}Adding standard test keys...${NC}"
echo "$VAL_MNEMONIC" | docker run --rm -i -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd keys add "$VAL_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" --home /data

echo "$USER1_MNEMONIC" | docker run --rm -i -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd keys add "$USER1_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" --home /data

echo "$USER2_MNEMONIC" | docker run --rm -i -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd keys add "$USER2_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" --home /data

echo "$USER3_MNEMONIC" | docker run --rm -i -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd keys add "$USER3_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" --home /data

echo "$USER4_MNEMONIC" | docker run --rm -i -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd keys add "$USER4_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" --home /data

# Configure genesis file using jq directly on host (simpler approach)
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
jq '.app_state.erc20.params.native_precompiles=["0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE"]' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"
jq '.app_state.erc20.token_pairs=[{contract_owner:1,erc20_address:"0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE",denom:"atest",enabled:true}]' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"

# Set gas limit in genesis
jq '.consensus.params.block.max_gas="10000000"' "$DATA_DIR/config/genesis.json" > "$DATA_DIR/config/tmp_genesis.json" && mv "$DATA_DIR/config/tmp_genesis.json" "$DATA_DIR/config/genesis.json"

# Add genesis accounts and generate validator transaction
echo -e "${GREEN}Setting up genesis accounts and validator...${NC}"

# Allocate genesis accounts (cosmos formatted addresses)
docker run --rm -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd genesis add-genesis-account "$VAL_KEY" 100000000000000000000000000atest --keyring-backend "$KEYRING" --home /data

docker run --rm -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd genesis add-genesis-account "$USER1_KEY" 1000000000000000000000atest --keyring-backend "$KEYRING" --home /data

docker run --rm -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd genesis add-genesis-account "$USER2_KEY" 1000000000000000000000atest --keyring-backend "$KEYRING" --home /data

docker run --rm -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd genesis add-genesis-account "$USER3_KEY" 1000000000000000000000atest --keyring-backend "$KEYRING" --home /data

docker run --rm -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd genesis add-genesis-account "$USER4_KEY" 1000000000000000000000atest --keyring-backend "$KEYRING" --home /data

# Sign genesis transaction
docker run --rm -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd genesis gentx "$VAL_KEY" 1000000000000000000000atest --gas-prices "${BASEFEE}atest" --keyring-backend "$KEYRING" --chain-id "$CHAIN_ID" --home /data

# Collect genesis tx
docker run --rm -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd genesis collect-gentxs --home /data

# Run this to ensure everything worked and that the genesis file is setup correctly
docker run --rm -v "$DATA_DIR:/data" --entrypoint="" cosmos/evmd \
    evmd genesis validate-genesis --home /data

# Configure timeout settings and enable all APIs (like local_node.sh)
echo -e "${GREEN}Configuring timeout settings and APIs...${NC}"
if [[ "$OSTYPE" == "darwin"* ]]; then
    # Configure consensus timeouts for faster block times
    sed -i '' 's/timeout_propose = "3s"/timeout_propose = "2s"/g' "$DATA_DIR/config/config.toml"
    sed -i '' 's/timeout_propose_delta = "500ms"/timeout_propose_delta = "200ms"/g' "$DATA_DIR/config/config.toml"
    sed -i '' 's/timeout_prevote = "1s"/timeout_prevote = "500ms"/g' "$DATA_DIR/config/config.toml"
    sed -i '' 's/timeout_prevote_delta = "500ms"/timeout_prevote_delta = "200ms"/g' "$DATA_DIR/config/config.toml"
    sed -i '' 's/timeout_precommit = "1s"/timeout_precommit = "500ms"/g' "$DATA_DIR/config/config.toml"
    sed -i '' 's/timeout_precommit_delta = "500ms"/timeout_precommit_delta = "200ms"/g' "$DATA_DIR/config/config.toml"
    sed -i '' 's/timeout_commit = "5s"/timeout_commit = "1s"/g' "$DATA_DIR/config/config.toml"
    sed -i '' 's/timeout_broadcast_tx_commit = "10s"/timeout_broadcast_tx_commit = "5s"/g' "$DATA_DIR/config/config.toml"
    
    # Enable prometheus metrics and all APIs for dev node
    sed -i '' 's/prometheus = false/prometheus = true/' "$DATA_DIR/config/config.toml"
    sed -i '' 's/prometheus-retention-time = 0/prometheus-retention-time  = 1000000000000/g' "$DATA_DIR/config/app.toml"
    sed -i '' 's/enabled = false/enabled = true/g' "$DATA_DIR/config/app.toml"
    sed -i '' 's/enable = false/enable = true/g' "$DATA_DIR/config/app.toml"
    
    # Configure JSON-RPC for external access
    sed -i '' 's/address = "127.0.0.1:8545"/address = "0.0.0.0:8545"/' "$DATA_DIR/config/app.toml"
    sed -i '' 's/ws-address = "127.0.0.1:8546"/ws-address = "0.0.0.0:8546"/' "$DATA_DIR/config/app.toml"
else
    # Configure consensus timeouts for faster block times  
    sed -i 's/timeout_propose = "3s"/timeout_propose = "2s"/g' "$DATA_DIR/config/config.toml"
    sed -i 's/timeout_propose_delta = "500ms"/timeout_propose_delta = "200ms"/g' "$DATA_DIR/config/config.toml"
    sed -i 's/timeout_prevote = "1s"/timeout_prevote = "500ms"/g' "$DATA_DIR/config/config.toml"
    sed -i 's/timeout_prevote_delta = "500ms"/timeout_prevote_delta = "200ms"/g' "$DATA_DIR/config/config.toml"
    sed -i 's/timeout_precommit = "1s"/timeout_precommit = "500ms"/g' "$DATA_DIR/config/config.toml"
    sed -i 's/timeout_precommit_delta = "500ms"/timeout_precommit_delta = "200ms"/g' "$DATA_DIR/config/config.toml"
    sed -i 's/timeout_commit = "5s"/timeout_commit = "1s"/g' "$DATA_DIR/config/config.toml"
    sed -i 's/timeout_broadcast_tx_commit = "10s"/timeout_broadcast_tx_commit = "5s"/g' "$DATA_DIR/config/config.toml"
    
    # Enable prometheus metrics and all APIs for dev node
    sed -i 's/prometheus = false/prometheus = true/' "$DATA_DIR/config/config.toml"
    sed -i 's/prometheus-retention-time  = "0"/prometheus-retention-time  = "1000000000000"/g' "$DATA_DIR/config/app.toml"
    sed -i 's/enabled = false/enabled = true/g' "$DATA_DIR/config/app.toml"
    sed -i 's/enable = false/enable = true/g' "$DATA_DIR/config/app.toml"
    
    # Configure JSON-RPC for external access
    sed -i 's/address = "127.0.0.1:8545"/address = "0.0.0.0:8545"/' "$DATA_DIR/config/app.toml"
    sed -i 's/ws-address = "127.0.0.1:8546"/ws-address = "0.0.0.0:8546"/' "$DATA_DIR/config/app.toml"
fi

# Change proposal periods to pass within a reasonable time for local testing
sed -i.bak 's/"max_deposit_period": "172800s"/"max_deposit_period": "30s"/g' "$DATA_DIR/config/genesis.json"
sed -i.bak 's/"voting_period": "172800s"/"voting_period": "30s"/g' "$DATA_DIR/config/genesis.json"
sed -i.bak 's/"expedited_voting_period": "86400s"/"expedited_voting_period": "15s"/g' "$DATA_DIR/config/genesis.json"

# Set pruning to nothing to preserve all blocks for debug APIs
sed -i.bak 's/pruning = "default"/pruning = "nothing"/g' "$DATA_DIR/config/app.toml"
sed -i.bak 's/pruning-keep-recent = "0"/pruning-keep-recent = "0"/g' "$DATA_DIR/config/app.toml"
sed -i.bak 's/pruning-interval = "0"/pruning-interval = "0"/g' "$DATA_DIR/config/app.toml"

echo -e "${GREEN}Configuration completed${NC}"

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
    cosmos/evmd \
    start \
    --home /data \
    --minimum-gas-prices=0.0001atest \
    --json-rpc.api eth,txpool,personal,net,debug,web3 \
    --keyring-backend test \
    --chain-id "$CHAIN_ID"

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