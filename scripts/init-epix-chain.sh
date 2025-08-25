#!/bin/bash

# Epix Chain Initialization Script
# This script helps initialize an Epix blockchain node

set -e

# Default values
CHAIN_ID=""
NODE_HOME="$HOME/.epixd"
MONIKER="epix-node"
KEYRING_BACKEND="test"
VALIDATOR_KEY="validator"
NETWORK_TYPE="testnet"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored output
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Help function
show_help() {
    cat << EOF
Epix Chain Initialization Script

Usage: $0 [OPTIONS]

OPTIONS:
    -n, --network TYPE      Network type: mainnet or testnet (default: testnet)
    -c, --chain-id ID       Custom chain ID (overrides network default)
    -h, --home DIR          Node home directory (default: ~/.epixd)
    -m, --moniker NAME      Node moniker (default: epix-node)
    -k, --keyring BACKEND   Keyring backend (default: test)
    -v, --validator KEY     Validator key name (default: validator)
    --help                  Show this help message

EXAMPLES:
    # Initialize testnet node
    $0 --network testnet

    # Initialize mainnet node
    $0 --network mainnet

    # Initialize with custom settings
    $0 --network testnet --moniker "my-epix-node" --home "/custom/path"

EOF
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -n|--network)
            NETWORK_TYPE="$2"
            shift 2
            ;;
        -c|--chain-id)
            CHAIN_ID="$2"
            shift 2
            ;;
        -h|--home)
            NODE_HOME="$2"
            shift 2
            ;;
        -m|--moniker)
            MONIKER="$2"
            shift 2
            ;;
        -k|--keyring)
            KEYRING_BACKEND="$2"
            shift 2
            ;;
        -v|--validator)
            VALIDATOR_KEY="$2"
            shift 2
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            print_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Set chain ID based on network type if not provided
if [ -z "$CHAIN_ID" ]; then
    case $NETWORK_TYPE in
        mainnet)
            CHAIN_ID="epix_1916-1"
            ;;
        testnet)
            CHAIN_ID="epix_1917-1"
            ;;
        *)
            print_error "Invalid network type: $NETWORK_TYPE. Must be 'mainnet' or 'testnet'"
            exit 1
            ;;
    esac
fi

print_status "Initializing Epix $NETWORK_TYPE node..."
print_status "Chain ID: $CHAIN_ID"
print_status "Node Home: $NODE_HOME"
print_status "Moniker: $MONIKER"

# Check if epixd binary exists and find the correct path
EPIXD_BINARY=""

# Check multiple possible locations for the epixd binary
if command -v epixd &> /dev/null; then
    EPIXD_BINARY="epixd"
elif [ -f "./build/epixd" ]; then
    EPIXD_BINARY="./build/epixd"
elif [ -f "$HOME/go/bin/epixd" ]; then
    EPIXD_BINARY="$HOME/go/bin/epixd"
elif [ -f "$(dirname "$0")/../build/epixd" ]; then
    EPIXD_BINARY="$(dirname "$0")/../build/epixd"
else
    print_error "epixd binary not found. Please build the project first:"
    echo "  make build"
    echo "  # or manually: cd evmd && go build -o epixd ./cmd/evmd/"
    echo ""
    echo "Searched in the following locations:"
    echo "  - System PATH"
    echo "  - ./build/epixd"
    echo "  - $HOME/go/bin/epixd"
    echo "  - $(dirname "$0")/../build/epixd"
    exit 1
fi

print_status "Using epixd binary: $EPIXD_BINARY"

# Remove existing node home if it exists
if [ -d "$NODE_HOME" ]; then
    print_warning "Node home directory already exists: $NODE_HOME"
    read -p "Do you want to remove it and start fresh? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        rm -rf "$NODE_HOME"
        print_status "Removed existing node home directory"
    else
        print_error "Aborted. Please remove the directory manually or use a different home directory."
        exit 1
    fi
fi

# Initialize the node
print_status "Initializing node..."
$EPIXD_BINARY init "$MONIKER" --chain-id "$CHAIN_ID" --home "$NODE_HOME"

# Create or recover validator key
print_status "Setting up validator key..."
if ! $EPIXD_BINARY keys show "$VALIDATOR_KEY" --keyring-backend "$KEYRING_BACKEND" --home "$NODE_HOME" &> /dev/null; then
    print_status "Creating new validator key: $VALIDATOR_KEY"
    $EPIXD_BINARY keys add "$VALIDATOR_KEY" --keyring-backend "$KEYRING_BACKEND" --home "$NODE_HOME"
else
    print_status "Validator key already exists: $VALIDATOR_KEY"
fi

# Get validator address
VALIDATOR_ADDR=$($EPIXD_BINARY keys show "$VALIDATOR_KEY" -a --keyring-backend "$KEYRING_BACKEND" --home "$NODE_HOME")
print_status "Validator address: $VALIDATOR_ADDR"

# Add genesis account
print_status "Adding genesis account..."
$EPIXD_BINARY genesis add-genesis-account "$VALIDATOR_ADDR" 1000000000000000000000000aepix --home "$NODE_HOME"

# Create genesis transaction
print_status "Creating genesis transaction..."
$EPIXD_BINARY genesis gentx "$VALIDATOR_KEY" 1000000000000000000000aepix \
    --chain-id "$CHAIN_ID" \
    --keyring-backend "$KEYRING_BACKEND" \
    --home "$NODE_HOME"

# Collect genesis transactions
print_status "Collecting genesis transactions..."
$EPIXD_BINARY genesis collect-gentxs --home "$NODE_HOME"

# Update genesis file for Epix chain
print_status "Updating genesis configuration for Epix chain..."
GENESIS_FILE="$NODE_HOME/config/genesis.json"

# Update staking module to use aepix as bond denomination
jq '.app_state.staking.params.bond_denom = "aepix"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"

# Update mint module to use aepix as mint denomination
jq '.app_state.mint.params.mint_denom = "aepix"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"

# Update governance module to use aepix for deposits
jq '.app_state.gov.params.min_deposit[0].denom = "aepix"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"
jq '.app_state.gov.params.min_deposit[0].amount = "10000000000000000000"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"
jq '.app_state.gov.params.expedited_min_deposit[0].denom = "aepix"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"
jq '.app_state.gov.params.expedited_min_deposit[0].amount = "50000000000000000000"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"

# Update EVM module to use aepix as EVM denomination
jq '.app_state.evm.params.evm_denom = "aepix"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"

# Update distribution module for Epix reward distribution (Modern Cosmos SDK approach)
# Set community tax to 2% (0.02) - this portion goes to community pool (standard for Cosmos chains)
# The remaining 98% goes to staking rewards distributed equally among all validators
print_status "Configuring Epix reward distribution (98% staking, 2% community pool - Modern approach)..."
jq '.app_state.distribution.params.community_tax = "0.020000000000000000"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"

# Update slashing module for Epix-specific parameters
print_status "Configuring Epix slashing parameters..."

# Set signed blocks window to 21,600 blocks (12-hour rolling window at 2 seconds per block)
jq '.app_state.slashing.params.signed_blocks_window = "21600"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"

# Set minimum signed per window to 5% (validators must sign at least 5% of blocks)
jq '.app_state.slashing.params.min_signed_per_window = "0.050000000000000000"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"

# Set downtime jail duration to 60 seconds
jq '.app_state.slashing.params.downtime_jail_duration = "60s"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"

# Set double sign slash fraction to 5%
jq '.app_state.slashing.params.slash_fraction_double_sign = "0.050000000000000000"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"

# Set downtime slash fraction to 1%
jq '.app_state.slashing.params.slash_fraction_downtime = "0.010000000000000000"' "$GENESIS_FILE" > "$GENESIS_FILE.tmp" && mv "$GENESIS_FILE.tmp" "$GENESIS_FILE"

# Validate genesis
print_status "Validating genesis..."
$EPIXD_BINARY genesis validate-genesis --home "$NODE_HOME"

print_success "Epix $NETWORK_TYPE node initialized successfully!"
print_status "Node home: $NODE_HOME"
print_status "Chain ID: $CHAIN_ID"
print_status "Validator key: $VALIDATOR_KEY"
print_status "Validator address: $VALIDATOR_ADDR"

echo
print_status "To start the node, run:"
echo "  $EPIXD_BINARY start --home $NODE_HOME"

echo
print_status "To check node status, run:"
echo "  $EPIXD_BINARY status --home $NODE_HOME"

echo
print_status "Configuration files are located in:"
echo "  $NODE_HOME/config/"
