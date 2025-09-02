#!/bin/bash

# Epix Chain Setup Script
# This script combines initialization and genesis configuration for the Epix blockchain
# It provides flexible options to run initialization only, configuration only, or both

set -e

# Configuration variables
CHAIN_NAME="Epix"
TICKER="EPIX"
DENOM="aepix"
DISPLAY_DENOM="epix"
DECIMALS=18

# Network configuration
TESTNET_CHAIN_ID="epix_1917-1"
MAINNET_CHAIN_ID="epix_1916-1"
DEFAULT_CHAIN_ID="$TESTNET_CHAIN_ID"

# Tokenomics configuration
GENESIS_SUPPLY="23689538000000000000000000"  # 23,689,538 EPIX with 18 decimals
AIRDROP_ALLOCATION="11844769000000000000000000"  # 11,844,769 EPIX (50% of genesis supply)
COMMUNITY_POOL_ALLOCATION="11844769000000000000000000"  # 11,844,769 EPIX (50% of genesis supply)
VALIDATOR_ALLOCATION="2000000000000000000"  # 2 EPIX for validator (1 to stake, 1 remaining)

# EpixMint configuration (dynamic emission with 25% annual reduction)
INITIAL_ANNUAL_MINT_AMOUNT="10527000000000000000000000000"  # 10.527 billion EPIX in year 1 (in aepix)
ANNUAL_REDUCTION_RATE="0.25"                                # 25% annual reduction
BLOCK_TIME_SECONDS="6"                                      # 6 second blocks
MAX_SUPPLY="42000000000000000000000000000"                  # 42 billion EPIX max supply (in aepix)

# Legacy mint module configuration (disabled)
INITIAL_INFLATION="0.000000000000000"  # Disabled in favor of epixmint
MAX_INFLATION="0.000000000000000"      # Disabled
MIN_INFLATION="0.000000000000000"      # Disabled
INFLATION_RATE_CHANGE="0.000000000000000"  # Disabled
GOAL_BONDED="0.670000000000000"        # Keep for staking

# Node configuration
MONIKER="${MONIKER:-epix-node}"
KEYRING="${KEYRING:-test}"
KEYALGO="${KEYALGO:-eth_secp256k1}"
LOGLEVEL="${LOGLEVEL:-info}"
CHAINDIR="${CHAINDIR:-$HOME/.epixd}"

# Network endpoints
TESTNET_RPC="https://rpc.testnet.epix.zone"
TESTNET_API="https://api.testnet.epix.zone"
MAINNET_RPC="https://rpc.testnet.epix.zone"  # Using testnet for now as specified
MAINNET_API="https://api.testnet.epix.zone"  # Using testnet for now as specified

# BIP44 configuration
COIN_TYPE=60  # Ethereum coin type as specified

# Contract addresses (centralized configuration)
# Preinstalled contracts
CREATE2_FACTORY="0x4e59b44847b379578588920ca78fbf26c0b4956c"
MULTICALL3="0xcA11bde05977b3631167028862bE2a173976CA11"
PERMIT2="0x000000000022D473030F116dDEE9F6B43aC78BA3"
SAFE_SINGLETON_FACTORY="0x914d7Fec6aaC8cd542e72Bca78B30650d45643d7"
EIP2935_HISTORY_STORAGE="0x0aae40965e6800cd9b1f4b05ff21581047e3f91e"

# Cosmos module precompiles
DISTRIBUTION_PRECOMPILE="0x0000000000000000000000000000000000000100"
STAKING_PRECOMPILE="0x0000000000000000000000000000000000000400"
IBC_TRANSFER_PRECOMPILE="0x0000000000000000000000000000000000000800"
ICS20_PRECOMPILE="0x0000000000000000000000000000000000000801"
IBC_CHANNEL_PRECOMPILE="0x0000000000000000000000000000000000000802"
IBC_CLIENT_PRECOMPILE="0x0000000000000000000000000000000000000803"
IBC_CONNECTION_PRECOMPILE="0x0000000000000000000000000000000000000804"
IBC_PORT_PRECOMPILE="0x0000000000000000000000000000000000000805"
AUTHORIZATION_PRECOMPILE="0x0000000000000000000000000000000000000806"
BANK_PRECOMPILE="0x0000000000000000000000000000000000000807"

# ERC20 native precompiles
NATIVE_EPIX_TOKEN="0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE"
WEPIX_TOKEN="0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"

# Cosmos module addresses (Bech32)
DISTRIBUTION_MODULE_ADDRESS="epix1jv65s3grqf6v6jl3dp4t6c9t9rk99cd8j52fwy"

# File paths
CONFIG_TOML="$CHAINDIR/config/config.toml"
APP_TOML="$CHAINDIR/config/app.toml"
GENESIS="$CHAINDIR/config/genesis.json"
TMP_GENESIS="$CHAINDIR/config/tmp_genesis.json"

# Airdrop snapshot file
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CSV_FILE="$SCRIPT_DIR/../artifacts/snapshot/snapshots.csv"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Operation flags
RUN_INIT=true
RUN_CONFIGURE=true
RESET=false
NETWORK="testnet"

# Helper functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if required tools are available
check_dependencies() {
    if ! command -v epixd &> /dev/null; then
        log_error "epixd binary not found. Please build and install epixd first."
        log_info "Run: make install"
        exit 1
    fi
    
    if [[ "$RUN_CONFIGURE" == "true" ]] && ! command -v jq &> /dev/null; then
        log_error "jq is required for genesis configuration but not installed. Please install jq."
        exit 1
    fi
    
    if [[ "$RUN_CONFIGURE" == "true" ]] && ! command -v bc &> /dev/null; then
        log_error "bc is required for calculations but not installed. Please install bc."
        exit 1
    fi
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            --init-only)
                RUN_INIT=true
                RUN_CONFIGURE=false
                shift
                ;;
            --configure-only)
                RUN_INIT=false
                RUN_CONFIGURE=true
                shift
                ;;
            --full)
                RUN_INIT=true
                RUN_CONFIGURE=true
                shift
                ;;
            --network)
                NETWORK="$2"
                shift 2
                ;;
            --chain-id)
                CHAIN_ID="$2"
                shift 2
                ;;
            --reset)
                RESET=true
                shift
                ;;
            --addresses-only)
                output_addresses_only
                exit 0
                ;;
            --verify-contracts)
                verify_contracts_on_chain
                exit 0
                ;;
            --help)
                show_help
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
    
    # Set chain ID based on network if not explicitly provided
    if [[ -z "$CHAIN_ID" ]]; then
        case $NETWORK in
            testnet)
                CHAIN_ID="$TESTNET_CHAIN_ID"
                ;;
            mainnet)
                CHAIN_ID="$MAINNET_CHAIN_ID"
                ;;
            *)
                log_error "Invalid network: $NETWORK. Use 'testnet' or 'mainnet'"
                exit 1
                ;;
        esac
    fi
    
    # Validate configuration dependencies
    if [[ "$RUN_CONFIGURE" == "true" ]] && [[ "$RUN_INIT" == "false" ]]; then
        if [[ ! -f "$GENESIS" ]]; then
            log_error "Genesis file not found at $GENESIS"
            log_info "Cannot run --configure-only without existing initialization."
            log_info "Either run --full or run --init-only first."
            exit 1
        fi
    fi
}

show_help() {
    cat << EOF
Epix Chain Setup Script

Usage: $0 [OPTIONS]

OPTIONS:
    --init-only          Only initialize the chain (skip genesis configuration)
    --configure-only     Only configure genesis (requires existing initialization)
    --full               Run both initialization and configuration (default)
    --network NETWORK    Network to initialize (testnet|mainnet) [default: testnet]
    --chain-id CHAIN_ID  Override default chain ID
    --reset              Reset existing chain data
    --addresses-only     Output contract addresses only (machine readable)
    --verify-contracts   Verify all contracts exist on chain via RPC calls
    --help               Show this help message

EXAMPLES:
    $0                           # Full setup for testnet
    $0 --network mainnet         # Full setup for mainnet
    $0 --init-only               # Only initialize testnet
    $0 --configure-only          # Only configure genesis (requires existing init)
    $0 --reset --full            # Reset and do full setup
    $0 --chain-id custom-1       # Use custom chain ID

TOKENOMICS:
    Genesis Supply: 23,689,538 EPIX
    Airdrop Allocation: From CSV snapshot (artifacts/snapshot/snapshots.csv)
    Community Pool: 11,844,769 EPIX (50%)
    Annual Mint: 2.099 billion EPIX
    Maximum Supply: 42,000,000,000 EPIX

GOVERNANCE & SECURITY:
    Voting Period: 7 days
    Min Deposit: 1,000 EPIX
    Slashing: 5% double-sign, 1% downtime
    Community Tax: 2% (default)

NETWORKS:
    Testnet:  $TESTNET_CHAIN_ID
    Mainnet:  $MAINNET_CHAIN_ID
EOF
}

# Initialize the chain
init_chain() {
    log_info "Initializing Epix chain with ID: $CHAIN_ID"
    
    if [[ "$RESET" == "true" ]] && [[ -d "$CHAINDIR" ]]; then
        log_warn "Resetting existing chain data..."
        rm -rf "$CHAINDIR"
    fi
    
    # Initialize chain
    epixd init "$MONIKER" --chain-id "$CHAIN_ID" --home "$CHAINDIR"
    
    # Set client configuration
    epixd config set client chain-id "$CHAIN_ID" --home "$CHAINDIR"
    epixd config set client keyring-backend "$KEYRING" --home "$CHAINDIR"
    
    log_info "Chain initialization completed successfully!"
}

# Configure basic chain parameters
configure_chain_params() {
    log_info "Configuring basic chain parameters..."

    # Set staking denomination
    jq --arg denom "$DENOM" '.app_state.staking.params.bond_denom = $denom' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set crisis module denomination
    jq --arg denom "$DENOM" '.app_state.crisis.constant_fee.denom = $denom' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set governance deposit denomination
    jq --arg denom "$DENOM" '.app_state.gov.deposit_params.min_deposit[0].denom = $denom' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set expedited governance deposit denomination
    jq --arg denom "$DENOM" '.app_state.gov.params.expedited_min_deposit[0].denom = $denom' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set mint denomination
    jq --arg denom "$DENOM" '.app_state.mint.params.mint_denom = $denom' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
}

# Configure minting parameters
configure_minting() {
    log_info "Configuring EpixMint parameters..."

    # Configure EpixMint module parameters
    jq '.app_state.epixmint.params.mint_denom = "aepix"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
    jq --arg amount "$INITIAL_ANNUAL_MINT_AMOUNT" '.app_state.epixmint.params.initial_annual_mint_amount = $amount' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
    jq --arg rate "$ANNUAL_REDUCTION_RATE" '.app_state.epixmint.params.annual_reduction_rate = $rate' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
    jq --arg time "$BLOCK_TIME_SECONDS" '.app_state.epixmint.params.block_time_seconds = ($time | tonumber)' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
    jq --arg supply "$MAX_SUPPLY" '.app_state.epixmint.params.max_supply = $supply' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Disable the old mint module by setting inflation to 0
    jq --arg inflation "$INITIAL_INFLATION" '.app_state.mint.params.inflation_rate_change = $inflation' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
    jq --arg inflation "$MAX_INFLATION" '.app_state.mint.params.inflation_max = $inflation' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
    jq --arg inflation "$MIN_INFLATION" '.app_state.mint.params.inflation_min = $inflation' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
    jq --arg inflation "$INITIAL_INFLATION" '.app_state.mint.minter.inflation = $inflation' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
    jq --arg goal "$GOAL_BONDED" '.app_state.mint.params.goal_bonded = $goal' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
}

# Configure governance parameters
configure_governance() {
    log_info "Configuring governance parameters..."

    # Set voting period to 7 days
    jq '.app_state.gov.params.voting_period = "604800s"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set expedited voting period to 1 day
    jq '.app_state.gov.params.expedited_voting_period = "86400s"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set minimum deposit
    jq --arg amount "1000000000000000000000" --arg denom "$DENOM" '.app_state.gov.params.min_deposit[0] = {"amount": $amount, "denom": $denom}' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set expedited minimum deposit
    jq --arg amount "5000000000000000000000" --arg denom "$DENOM" '.app_state.gov.params.expedited_min_deposit[0] = {"amount": $amount, "denom": $denom}' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
}

# Configure slashing parameters
configure_slashing() {
    log_info "Configuring slashing parameters..."

    # Set signed blocks window (number of blocks to track for downtime)
    jq '.app_state.slashing.params.signed_blocks_window = "21600"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set minimum signed per window (5% minimum uptime required)
    jq '.app_state.slashing.params.min_signed_per_window = "0.050000000000000000"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set downtime jail duration (how long validator is jailed for downtime)
    jq '.app_state.slashing.params.downtime_jail_duration = "60s"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set slash fraction for double signing (5% of bonded tokens slashed)
    jq '.app_state.slashing.params.slash_fraction_double_sign = "0.050000000000000000"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set slash fraction for downtime (1% of bonded tokens slashed)
    jq '.app_state.slashing.params.slash_fraction_downtime = "0.010000000000000000"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
}

# Process airdrop snapshot CSV and allocate tokens
process_airdrop_csv() {
    log_info "Processing airdrop snapshot from CSV..."

    # Check if CSV file exists
    if [[ ! -f "$CSV_FILE" ]]; then
        log_error "Airdrop snapshot CSV file not found at: $CSV_FILE"
        exit 1
    fi

    log_info "Reading airdrop data from: $CSV_FILE"

    # Initialize counters
    local total_airdrop_amount=0
    local account_count=0
    local line_number=0

    # Read CSV file line by line
    while IFS=',' read -r address balance || [[ -n "$address" ]]; do
        line_number=$((line_number + 1))

        # Skip header line
        if [[ $line_number -eq 1 ]]; then
            continue
        fi

        # Skip empty lines
        if [[ -z "$address" || -z "$balance" ]]; then
            continue
        fi

        # Trim whitespace
        address=$(echo "$address" | tr -d ' \t\r\n')
        balance=$(echo "$balance" | tr -d ' \t\r\n')

        # Validate address format (should start with "epix1")
        if [[ ! "$address" =~ ^epix1[a-z0-9]{38}$ ]]; then
            log_warn "Invalid address format on line $line_number: $address"
            continue
        fi

        # Validate balance is a number
        if ! echo "$balance" | grep -qE '^[0-9]+(\.[0-9]+)?$'; then
            log_warn "Invalid balance format on line $line_number: $balance"
            continue
        fi

        # Convert EPIX to aepix (multiply by 10^18)
        # Use bc for precise decimal arithmetic
        local aepix_amount=$(echo "scale=0; $balance * 10^18 / 1" | bc)

        # Add account to genesis
        epixd genesis add-genesis-account "$address" "${aepix_amount}${DENOM}" --home "$CHAINDIR"

        # Update counters
        total_airdrop_amount=$(echo "scale=0; $total_airdrop_amount + $aepix_amount" | bc)
        account_count=$((account_count + 1))

        # Log progress every 50 accounts
        if [[ $((account_count % 50)) -eq 0 ]]; then
            log_info "Processed $account_count accounts..."
        fi

    done < "$CSV_FILE"

    # Validate total amount matches expected allocation
    local expected_total="$AIRDROP_ALLOCATION"
    if [[ "$total_airdrop_amount" != "$expected_total" ]]; then
        log_warn "Total airdrop amount ($total_airdrop_amount aepix) does not match expected allocation ($expected_total aepix)"
        log_warn "CSV total: $(echo "scale=6; $total_airdrop_amount / 10^18" | bc) EPIX"
        log_warn "Expected: $(echo "scale=6; $expected_total / 10^18" | bc) EPIX"
        log_info "Updating AIRDROP_ALLOCATION to match CSV total..."
        AIRDROP_ALLOCATION="$total_airdrop_amount"
    fi

    log_info "Airdrop processing completed!"
    log_info "Total accounts: $account_count"
    log_info "Total allocation: $(echo "scale=6; $total_airdrop_amount / 10^18" | bc) EPIX"
}

# Configure community pool
configure_community_pool() {
    log_info "Configuring community pool..."

    # Set the community pool balance directly in the distribution module
    jq --arg amount "${COMMUNITY_POOL_ALLOCATION}.000000000000000000" --arg denom "$DENOM" \
        '.app_state.distribution.fee_pool.community_pool = [{"amount": $amount, "denom": $denom}]' \
        "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Use the distribution module address
    # This is generated deterministically from the module name "distribution"
    # Using the same address as in the working old script
    DISTRIBUTION_MODULE_ADDRESS="epix1jv65s3grqf6v6jl3dp4t6c9t9rk99cd8j52fwy"

    # Add the community pool tokens to the bank balances for the distribution module
    # This ensures the tokens are counted in the bank module's supply validation
    jq --arg address "$DISTRIBUTION_MODULE_ADDRESS" --arg amount "$COMMUNITY_POOL_ALLOCATION" --arg denom "$DENOM" \
        '.app_state.bank.balances += [{"address": $address, "coins": [{"denom": $denom, "amount": $amount}]}]' \
        "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    log_info "Community pool allocation: $(echo "scale=0; $COMMUNITY_POOL_ALLOCATION / 10^18" | bc) EPIX"
    log_info "Distribution module address: $DISTRIBUTION_MODULE_ADDRESS"
}



# Configure total supply
configure_total_supply() {
    log_info "Configuring total supply..."

    # Calculate total supply including community pool tokens
    # GENESIS_SUPPLY includes both airdrop and community pool allocations
    # The community pool tokens are in the distribution module, so we need to include them in bank supply
    TOTAL_SUPPLY="$GENESIS_SUPPLY"  # 23,689,538 EPIX with 18 decimals

    # Set total supply in bank module to include all tokens (accounts + community pool)
    jq --arg amount "$TOTAL_SUPPLY" --arg denom "$DENOM" \
        '.app_state.bank.supply = [{"amount": $amount, "denom": $denom}]' \
        "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    log_info "Total supply set to: $(echo "scale=0; $TOTAL_SUPPLY / 10^18" | bc) EPIX"
}

# Configure EVM parameters
configure_evm() {
    log_info "Configuring EVM parameters..."

    # Set EVM denomination
    jq --arg denom "$DENOM" '.app_state.evm.params.evm_denom = $denom' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Enable all precompiles (using centralized variables)
    jq --arg dist "$DISTRIBUTION_PRECOMPILE" \
       --arg staking "$STAKING_PRECOMPILE" \
       --arg ibc_transfer "$IBC_TRANSFER_PRECOMPILE" \
       --arg ics20 "$ICS20_PRECOMPILE" \
       --arg ibc_channel "$IBC_CHANNEL_PRECOMPILE" \
       --arg ibc_client "$IBC_CLIENT_PRECOMPILE" \
       --arg ibc_connection "$IBC_CONNECTION_PRECOMPILE" \
       --arg ibc_port "$IBC_PORT_PRECOMPILE" \
       --arg auth "$AUTHORIZATION_PRECOMPILE" \
       --arg bank "$BANK_PRECOMPILE" \
       '.app_state.evm.params.active_static_precompiles = [
        $dist, $staking, $ibc_transfer, $ics20, $ibc_channel,
        $ibc_client, $ibc_connection, $ibc_port, $auth, $bank
    ]' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Add preinstalled contracts (using centralized variables)
    log_info "Adding preinstalled contracts to genesis..."
    jq --arg create2_addr "$CREATE2_FACTORY" \
       --arg multicall3_addr "$MULTICALL3" \
       --arg permit2_addr "$PERMIT2" \
       --arg safe_factory_addr "$SAFE_SINGLETON_FACTORY" \
       --arg eip2935_addr "$EIP2935_HISTORY_STORAGE" \
       '.app_state.evm.preinstalls = [
        {
            "name": "Create2",
            "address": $create2_addr,
            "code": "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3"
        },
        {
            "name": "Multicall3",
            "address": $multicall3_addr,
            "code": "0x6080604052600436106100f35760003560e01c80634d2301cc1161008a578063a8b0574e11610059578063a8b0574e1461025a578063bce38bd714610275578063c3077fa914610288578063ee82ac5e1461029b57600080fd5b80634d2301cc146101ec57806372425d9d1461022157806382ad56cb1461023457806386d516e81461024757600080fd5b80633408e470116100c65780633408e47014610191578063399542e9146101a45780633e64a696146101c657806342cbb15c146101d957600080fd5b80630f28c97d146100f8578063174dea711461011a578063252dba421461013a57806327e86d6e1461015b575b600080fd5b34801561010457600080fd5b50425b6040519081526020015b60405180910390f35b61012d610128366004610a85565b6102ba565b6040516101119190610bbe565b61014d610148366004610a85565b6104ef565b604051610111929190610bd8565b34801561016757600080fd5b50437fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff0140610107565b34801561019d57600080fd5b5046610107565b6101b76101b2366004610c60565b610690565b60405161011193929190610cba565b3480156101d257600080fd5b5048610107565b3480156101e557600080fd5b5043610107565b3480156101f857600080fd5b50610107610207366004610ce2565b73ffffffffffffffffffffffffffffffffffffffff163190565b34801561022d57600080fd5b5044610107565b61012d610242366004610a85565b6106ab565b34801561025357600080fd5b5045610107565b34801561026657600080fd5b50604051418152602001610111565b61012d610283366004610c60565b61085a565b6101b7610296366004610a85565b610a1a565b3480156102a757600080fd5b506101076102b6366004610d18565b4090565b60606000828067ffffffffffffffff8111156102d8576102d8610d31565b60405190808252806020026020018201604052801561031e57816020015b6040805180820190915260008152606060208201528152602001906001900390816102f65790505b5092503660005b8281101561047757600085828151811061034157610341610d60565b6020026020010151905087878381811061035d5761035d610d60565b905060200281019061036f9190610d8f565b6040810135958601959093506103886020850185610ce2565b73ffffffffffffffffffffffffffffffffffffffff16816103ac6060870187610dcd565b6040516103ba929190610e32565b60006040518083038185875af1925050503d80600081146103f7576040519150601f19603f3d011682016040523d82523d6000602084013e6103fc565b606091505b50602080850191909152901515808452908501351761046d577f08c379a000000000000000000000000000000000000000000000000000000000600052602060045260176024527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060445260846000fd5b5050600101610325565b508234146104e6576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601a60248201527f4d756c746963616c6c333a2076616c7565206d69736d6174636800000000000060448201526064015b60405180910390fd5b50505092915050565b436060828067ffffffffffffffff81111561050c5761050c610d31565b60405190808252806020026020018201604052801561053f57816020015b606081526020019060019003908161052a5790505b5091503660005b8281101561068657600087878381811061056257610562610d60565b90506020028101906105749190610e42565b92506105836020840184610ce2565b73ffffffffffffffffffffffffffffffffffffffff166105a66020850185610dcd565b6040516105b4929190610e32565b6000604051808303816000865af19150503d80600081146105f1576040519150601f19603f3d011682016040523d82523d6000602084013e6105f6565b606091505b5086848151811061060957610609610d60565b602090810291909101015290508061067d576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060448201526064016104dd565b50600101610546565b5050509250929050565b43804060606106a086868661085a565b905093509350939050565b6060818067ffffffffffffffff8111156106c7576106c7610d31565b60405190808252806020026020018201604052801561070d57816020015b6040805180820190915260008152606060208201528152602001906001900390816106e55790505b5091503660005b828110156104e657600084828151811061073057610730610d60565b6020026020010151905086868381811061074c5761074c610d60565b905060200281019061075e9190610e76565b925061076d6020840184610ce2565b73ffffffffffffffffffffffffffffffffffffffff166107906040850185610dcd565b60405161079e929190610e32565b6000604051808303816000865af19150503d80600081146107db576040519150601f19603f3d011682016040523d82523d6000602084013e6107e0565b606091505b506020808401919091529015158083529084013517610851577f08c379a000000000000000000000000000000000000000000000000000000000600052602060045260176024527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060445260646000fd5b50600101610714565b6060818067ffffffffffffffff81111561087657610876610d31565b6040519080825280602002602001820160405280156108bc57816020015b6040805180820190915260008152606060208201528152602001906001900390816108945790505b5091503660005b82811015610a105760008482815181106108df576108df610d60565b602002602001015190508686838181106108fb576108fb610d60565b905060200281019061090d9190610e42565b925061091c6020840184610ce2565b73ffffffffffffffffffffffffffffffffffffffff1661093f6020850185610dcd565b60405161094d929190610e32565b6000604051808303816000865af19150503d806000811461098a576040519150601f19603f3d011682016040523d82523d6000602084013e61098f565b606091505b506020830152151581528715610a07578051610a07576040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601760248201527f4d756c746963616c6c333a2063616c6c206661696c656400000000000000000060448201526064016104dd565b506001016108c3565b5050509392505050565b6000806060610a2b60018686610690565b919790965090945092505050565b60008083601f840112610a4b57600080fd5b50813567ffffffffffffffff811115610a6357600080fd5b6020830191508360208260051b8501011115610a7e57600080fd5b9250929050565b60008060208385031215610a9857600080fd5b823567ffffffffffffffff811115610aaf57600080fd5b610abb85828601610a39565b90969095509350505050565b6000815180845260005b81811015610aed57602081850181015186830182015201610ad1565b81811115610aff576000602083870101525b50601f017fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0169290920160200192915050565b600082825180855260208086019550808260051b84010181860160005b84811015610bb1578583037fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe001895281518051151584528401516040858501819052610b9d81860183610ac7565b9a86019a9450505090830190600101610b4f565b5090979650505050505050565b602081526000610bd16020830184610b32565b9392505050565b600060408201848352602060408185015281855180845260608601915060608160051b870101935082870160005b82811015610c52577fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffa0888703018452610c40868351610ac7565b95509284019290840190600101610c06565b509398975050505050505050565b600080600060408486031215610c7557600080fd5b83358015158114610c8557600080fd5b9250602084013567ffffffffffffffff811115610ca157600080fd5b610cad86828701610a39565b9497909650939450505050565b838152826020820152606060408201526000610cd96060830184610b32565b95945050505050565b600060208284031215610cf457600080fd5b813573ffffffffffffffffffffffffffffffffffffffff81168114610bd157600080fd5b600060208284031215610d2a57600080fd5b5035919050565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b600082357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff81833603018112610dc357600080fd5b9190910192915050565b60008083357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe1843603018112610e0257600080fd5b83018035915067ffffffffffffffff821115610e1d57600080fd5b602001915036819003821315610a7e57600080fd5b8183823760009101908152919050565b600082357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc1833603018112610dc357600080fd5b600082357fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffa1833603018112610dc357600080fdfea2646970667358221220bb2b5c71a328032f97c676ae39a1ec2148d3e5d6f73d95e9b17910152d61f16264736f6c634300080c0033"
        },
        {
            "name": "Permit2",
            "address": "0x000000000022D473030F116dDEE9F6B43aC78BA3",
            "code": "0x6040608081526004908136101561001557600080fd5b600090813560e01c80630d58b1db1461126c578063137c29fe146110755780632a2d80d114610db75780632b67b57014610bde57806330f28b7a14610ade5780633644e51514610a9d57806336c7851614610a285780633ff9dcb1146109a85780634fe02b441461093f57806365d9723c146107ac57806387517c451461067a578063927da105146105c3578063cc53287f146104a3578063edd9444b1461033a5763fe8ec1a7146100c657600080fd5b346103365760c07ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc3601126103365767ffffffffffffffff833581811161033257610114903690860161164b565b60243582811161032e5761012b903690870161161a565b6101336114e6565b9160843585811161032a5761014b9036908a016115c1565b98909560a43590811161032657610164913691016115c1565b969095815190610173826113ff565b606b82527f5065726d697442617463685769746e6573735472616e7366657246726f6d285460208301527f6f6b656e5065726d697373696f6e735b5d207065726d69747465642c61646472838301527f657373207370656e6465722c75696e74323536206e6f6e63652c75696e74323560608301527f3620646561646c696e652c000000000000000000000000000000000000000000608083015282519a8b9181610222602085018096611f93565b918237018a8152039961025b7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe09b8c8101835282611437565b5190209085515161026b81611ebb565b908a5b8181106102f95750506102f6999a6102ed9183516102a081610294602082018095611f66565b03848101835282611437565b519020602089810151858b015195519182019687526040820192909252336060820152608081019190915260a081019390935260643560c08401528260e081015b03908101835282611437565b51902093611cf7565b80f35b8061031161030b610321938c5161175e565b51612054565b61031b828661175e565b52611f0a565b61026e565b8880fd5b8780fd5b8480fd5b8380fd5b5080fd5b5091346103365760807ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc3601126103365767ffffffffffffffff9080358281116103325761038b903690830161164b565b60243583811161032e576103a2903690840161161a565b9390926103ad6114e6565b9160643590811161049f576103c4913691016115c1565b949093835151976103d489611ebb565b98885b81811061047d5750506102f697988151610425816103f9602082018095611f66565b037fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe08101835282611437565b5190206020860151828701519083519260208401947ffcf35f5ac6a2c28868dc44c302166470266239195f02b0ee408334829333b7668652840152336060840152608083015260a082015260a081526102ed8161141b565b808b61031b8261049461030b61049a968d5161175e565b9261175e565b6103d7565b8680fd5b5082346105bf57602090817ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc3601126103325780359067ffffffffffffffff821161032e576104f49136910161161a565b929091845b848110610504578580f35b8061051a610515600193888861196c565b61197c565b61052f84610529848a8a61196c565b0161197c565b3389528385528589209173ffffffffffffffffffffffffffffffffffffffff80911692838b528652868a20911690818a5285528589207fffffffffffffffffffffffff000000000000000000000000000000000000000081541690558551918252848201527f89b1add15eff56b3dfe299ad94e01f2b52fbcb80ae1a3baea6ae8c04cb2b98a4853392a2016104f9565b8280fd5b50346103365760607ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc36011261033657610676816105ff6114a0565b936106086114c3565b6106106114e6565b73ffffffffffffffffffffffffffffffffffffffff968716835260016020908152848420928816845291825283832090871683528152919020549251938316845260a083901c65ffffffffffff169084015260d09190911c604083015281906060820190565b0390f35b50346103365760807ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc360112610336576106b26114a0565b906106bb6114c3565b916106c46114e6565b65ffffffffffff926064358481169081810361032a5779ffffffffffff0000000000000000000000000000000000000000947fda9fa7c1b00402c17d0161b249b1ab8bbec047c5a52207b9c112deffd817036b94338a5260016020527fffffffffffff0000000000000000000000000000000000000000000000000000858b209873ffffffffffffffffffffffffffffffffffffffff809416998a8d5260205283878d209b169a8b8d52602052868c209486156000146107a457504216925b8454921697889360a01b16911617179055815193845260208401523392a480f35b905092610783565b5082346105bf5760607ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc3601126105bf576107e56114a0565b906107ee6114c3565b9265ffffffffffff604435818116939084810361032a57338852602091600183528489209673ffffffffffffffffffffffffffffffffffffffff80911697888b528452858a20981697888a5283528489205460d01c93848711156109175761ffff9085840316116108f05750907f55eb90d810e1700b35a8e7e25395ff7f2b2259abd7415ca2284dfb1c246418f393929133895260018252838920878a528252838920888a5282528389209079ffffffffffffffffffffffffffffffffffffffffffffffffffff7fffffffffffff000000000000000000000000000000000000000000000000000083549260d01b16911617905582519485528401523392a480f35b84517f24d35a26000000000000000000000000000000000000000000000000000000008152fd5b5084517f756688fe000000000000000000000000000000000000000000000000000000008152fd5b503461033657807ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc360112610336578060209273ffffffffffffffffffffffffffffffffffffffff61098f6114a0565b1681528084528181206024358252845220549051908152f35b5082346105bf57817ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc3601126105bf577f3704902f963766a4e561bbaab6e6cdc1b1dd12f6e9e99648da8843b3f46b918d90359160243533855284602052818520848652602052818520818154179055815193845260208401523392a280f35b8234610a9a5760807ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc360112610a9a57610a606114a0565b610a686114c3565b610a706114e6565b6064359173ffffffffffffffffffffffffffffffffffffffff8316830361032e576102f6936117a1565b80fd5b503461033657817ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc36011261033657602090610ad7611b1e565b9051908152f35b508290346105bf576101007ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc3601126105bf57610b1a3661152a565b90807fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7c36011261033257610b4c611478565b9160e43567ffffffffffffffff8111610bda576102f694610b6f913691016115c1565b939092610b7c8351612054565b6020840151828501519083519260208401947f939c21a48a8dbe3a9a2404a1d46691e4d39f6583d6ec6b35714604c986d801068652840152336060840152608083015260a082015260a08152610bd18161141b565b51902091611c25565b8580fd5b509134610336576101007ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc36011261033657610c186114a0565b7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffdc360160c08112610332576080855191610c51836113e3565b1261033257845190610c6282611398565b73ffffffffffffffffffffffffffffffffffffffff91602435838116810361049f578152604435838116810361049f57602082015265ffffffffffff606435818116810361032a5788830152608435908116810361049f576060820152815260a435938285168503610bda576020820194855260c4359087830182815260e43567ffffffffffffffff811161032657610cfe90369084016115c1565b929093804211610d88575050918591610d786102f6999a610d7e95610d238851611fbe565b90898c511690519083519260208401947ff3841cd1ff0085026a6327b620b67997ce40f282c88a8e905a7a5626e310f3d086528401526060830152608082015260808152610d70816113ff565b519020611bd9565b916120c7565b519251169161199d565b602492508a51917fcd21db4f000000000000000000000000000000000000000000000000000000008352820152fd5b5091346103365760607ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc93818536011261033257610df36114a0565b9260249081359267ffffffffffffffff9788851161032a578590853603011261049f578051978589018981108282111761104a578252848301358181116103265785019036602383011215610326578382013591610e50836115ef565b90610e5d85519283611437565b838252602093878584019160071b83010191368311611046578801905b828210610fe9575050508a526044610e93868801611509565b96838c01978852013594838b0191868352604435908111610fe557610ebb90369087016115c1565b959096804211610fba575050508998995151610ed681611ebb565b908b5b818110610f9757505092889492610d7892610f6497958351610f02816103f98682018095611f66565b5190209073ffffffffffffffffffffffffffffffffffffffff9a8b8b51169151928551948501957faf1b0d30d2cab0380e68f0689007e3254993c596f2fdd0aaa7f4d04f794408638752850152830152608082015260808152610d70816113ff565b51169082515192845b848110610f78578580f35b80610f918585610f8b600195875161175e565b5161199d565b01610f6d565b80610311610fac8e9f9e93610fb2945161175e565b51611fbe565b9b9a9b610ed9565b8551917fcd21db4f000000000000000000000000000000000000000000000000000000008352820152fd5b8a80fd5b6080823603126110465785608091885161100281611398565b61100b85611509565b8152611018838601611509565b838201526110278a8601611607565b8a8201528d611037818701611607565b90820152815201910190610e7a565b8c80fd5b84896041867f4e487b7100000000000000000000000000000000000000000000000000000000835252fd5b5082346105bf576101407ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc3601126105bf576110b03661152a565b91807fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff7c360112610332576110e2611478565b67ffffffffffffffff93906101043585811161049f5761110590369086016115c1565b90936101243596871161032a57611125610bd1966102f6983691016115c1565b969095825190611134826113ff565b606482527f5065726d69745769746e6573735472616e7366657246726f6d28546f6b656e5060208301527f65726d697373696f6e73207065726d69747465642c6164647265737320737065848301527f6e6465722c75696e74323536206e6f6e63652c75696e7432353620646561646c60608301527f696e652c0000000000000000000000000000000000000000000000000000000060808301528351948591816111e3602085018096611f93565b918237018b8152039361121c7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe095868101835282611437565b5190209261122a8651612054565b6020878101518589015195519182019687526040820192909252336060820152608081019190915260a081019390935260e43560c08401528260e081016102e1565b5082346105bf576020807ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc36011261033257813567ffffffffffffffff92838211610bda5736602383011215610bda5781013592831161032e576024906007368386831b8401011161049f57865b8581106112e5578780f35b80821b83019060807fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffdc83360301126103265761139288876001946060835161132c81611398565b611368608461133c8d8601611509565b9485845261134c60448201611509565b809785015261135d60648201611509565b809885015201611509565b918291015273ffffffffffffffffffffffffffffffffffffffff80808093169516931691166117a1565b016112da565b6080810190811067ffffffffffffffff8211176113b457604052565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052604160045260246000fd5b6060810190811067ffffffffffffffff8211176113b457604052565b60a0810190811067ffffffffffffffff8211176113b457604052565b60c0810190811067ffffffffffffffff8211176113b457604052565b90601f7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0910116810190811067ffffffffffffffff8211176113b457604052565b60c4359073ffffffffffffffffffffffffffffffffffffffff8216820361149b57565b600080fd5b6004359073ffffffffffffffffffffffffffffffffffffffff8216820361149b57565b6024359073ffffffffffffffffffffffffffffffffffffffff8216820361149b57565b6044359073ffffffffffffffffffffffffffffffffffffffff8216820361149b57565b359073ffffffffffffffffffffffffffffffffffffffff8216820361149b57565b7ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffc01906080821261149b576040805190611563826113e3565b8082941261149b57805181810181811067ffffffffffffffff8211176113b457825260043573ffffffffffffffffffffffffffffffffffffffff8116810361149b578152602435602082015282526044356020830152606435910152565b9181601f8401121561149b5782359167ffffffffffffffff831161149b576020838186019501011161149b57565b67ffffffffffffffff81116113b45760051b60200190565b359065ffffffffffff8216820361149b57565b9181601f8401121561149b5782359167ffffffffffffffff831161149b576020808501948460061b01011161149b57565b91909160608184031261149b576040805191611666836113e3565b8294813567ffffffffffffffff9081811161149b57830182601f8201121561149b578035611693816115ef565b926116a087519485611437565b818452602094858086019360061b8501019381851161149b579086899897969594939201925b8484106116e3575050505050855280820135908501520135910152565b90919293949596978483031261149b578851908982019082821085831117611730578a928992845261171487611509565b81528287013583820152815201930191908897969594936116c6565b602460007f4e487b710000000000000000000000000000000000000000000000000000000081526041600452fd5b80518210156117725760209160051b010190565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052603260045260246000fd5b92919273ffffffffffffffffffffffffffffffffffffffff604060008284168152600160205282828220961695868252602052818120338252602052209485549565ffffffffffff8760a01c16804211611884575082871696838803611812575b5050611810955016926118b5565b565b878484161160001461184f57602488604051907ff96fb0710000000000000000000000000000000000000000000000000000000082526004820152fd5b7fffffffffffffffffffffffff000000000000000000000000000000000000000084846118109a031691161790553880611802565b602490604051907fd81b2f2e0000000000000000000000000000000000000000000000000000000082526004820152fd5b9060006064926020958295604051947f23b872dd0000000000000000000000000000000000000000000000000000000086526004860152602485015260448401525af13d15601f3d116001600051141617161561190e57565b60646040517f08c379a000000000000000000000000000000000000000000000000000000000815260206004820152601460248201527f5452414e534645525f46524f4d5f4641494c45440000000000000000000000006044820152fd5b91908110156117725760061b0190565b3573ffffffffffffffffffffffffffffffffffffffff8116810361149b5790565b9065ffffffffffff908160608401511673ffffffffffffffffffffffffffffffffffffffff908185511694826020820151169280866040809401511695169560009187835260016020528383208984526020528383209916988983526020528282209184835460d01c03611af5579185611ace94927fc6a377bfc4eb120024a8ac08eef205be16b817020812c73223e81d1bdb9708ec98979694508715600014611ad35779ffffffffffff00000000000000000000000000000000000000009042165b60a01b167fffffffffffff00000000000000000000000000000000000000000000000000006001860160d01b1617179055519384938491604091949373ffffffffffffffffffffffffffffffffffffffff606085019616845265ffffffffffff809216602085015216910152565b0390a4565b5079ffffffffffff000000000000000000000000000000000000000087611a60565b600484517f756688fe000000000000000000000000000000000000000000000000000000008152fd5b467f000000000000000000000000000000000000000000000000000000000000000103611b69577f866a5aba21966af95d6c7ab78eb2b2fc913915c28be3b9aa07cc04ff903e3f2890565b60405160208101907f8cad95687ba82c2ce50e74f7b754645e5117c3a5bec8151c0726d5857980a86682527f9ac997416e8ff9d2ff6bebeb7149f65cdae5e32e2b90440b566bb3044041d36a604082015246606082015230608082015260808152611bd3816113ff565b51902090565b611be1611b1e565b906040519060208201927f190100000000000000000000000000000000000000000000000000000000000084526022830152604282015260428152611bd381611398565b9192909360a435936040840151804211611cc65750602084510151808611611c955750918591610d78611c6594611c60602088015186611e47565b611bd9565b73ffffffffffffffffffffffffffffffffffffffff809151511692608435918216820361149b57611810936118b5565b602490604051907f3728b83d0000000000000000000000000000000000000000000000000000000082526004820152fd5b602490604051907fcd21db4f0000000000000000000000000000000000000000000000000000000082526004820152fd5b959093958051519560409283830151804211611e175750848803611dee57611d2e918691610d7860209b611c608d88015186611e47565b60005b868110611d42575050505050505050565b611d4d81835161175e565b5188611d5a83878a61196c565b01359089810151808311611dbe575091818888886001968596611d84575b50505050505001611d31565b611db395611dad9273ffffffffffffffffffffffffffffffffffffffff6105159351169561196c565b916118b5565b803888888883611d78565b6024908651907f3728b83d0000000000000000000000000000000000000000000000000000000082526004820152fd5b600484517fff633a38000000000000000000000000000000000000000000000000000000008152fd5b6024908551907fcd21db4f0000000000000000000000000000000000000000000000000000000082526004820152fd5b9073ffffffffffffffffffffffffffffffffffffffff600160ff83161b9216600052600060205260406000209060081c6000526020526040600020818154188091551615611e9157565b60046040517f756688fe000000000000000000000000000000000000000000000000000000008152fd5b90611ec5826115ef565b611ed26040519182611437565b8281527fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0611f0082946115ef565b0190602036910137565b7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8114611f375760010190565b7f4e487b7100000000000000000000000000000000000000000000000000000000600052601160045260246000fd5b805160208092019160005b828110611f7f575050505090565b835185529381019392810192600101611f71565b9081519160005b838110611fab575050016000815290565b8060208092840101518185015201611f9a565b60405160208101917f65626cad6cb96493bf6f5ebea28756c966f023ab9e8a83a7101849d5573b3678835273ffffffffffffffffffffffffffffffffffffffff8082511660408401526020820151166060830152606065ffffffffffff9182604082015116608085015201511660a082015260a0815260c0810181811067ffffffffffffffff8211176113b45760405251902090565b6040516020808201927f618358ac3db8dc274f0cd8829da7e234bd48cd73c4a740aede1adec9846d06a1845273ffffffffffffffffffffffffffffffffffffffff81511660408401520151606082015260608152611bd381611398565b919082604091031261149b576020823592013590565b6000843b61222e5750604182036121ac576120e4828201826120b1565b939092604010156117725760209360009360ff6040608095013560f81c5b60405194855216868401526040830152606082015282805260015afa156121a05773ffffffffffffffffffffffffffffffffffffffff806000511691821561217657160361214c57565b60046040517f815e1d64000000000000000000000000000000000000000000000000000000008152fd5b60046040517f8baa579f000000000000000000000000000000000000000000000000000000008152fd5b6040513d6000823e3d90fd5b60408203612204576121c0918101906120b1565b91601b7f7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff84169360ff1c019060ff8211611f375760209360009360ff608094612102565b60046040517f4be6321b000000000000000000000000000000000000000000000000000000008152fd5b929391601f928173ffffffffffffffffffffffffffffffffffffffff60646020957fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe0604051988997889687947f1626ba7e000000000000000000000000000000000000000000000000000000009e8f8752600487015260406024870152816044870152868601378b85828601015201168101030192165afa9081156123a857829161232a575b7fffffffff000000000000000000000000000000000000000000000000000000009150160361230057565b60046040517fb0669cbc000000000000000000000000000000000000000000000000000000008152fd5b90506020813d82116123a0575b8161234460209383611437565b810103126103365751907fffffffff0000000000000000000000000000000000000000000000000000000082168203610a9a57507fffffffff0000000000000000000000000000000000000000000000000000000090386122d4565b3d9150612337565b6040513d84823e3d90fdfea164736f6c6343000811000a"
        },
        {
            "name": "Safe singleton factory",
            "address": "0x914d7Fec6aaC8cd542e72Bca78B30650d45643d7",
            "code": "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3"
        },
        {
            "name": "EIP-2935 - Serve historical block hashes from state",
            "address": "0x0aae40965e6800cd9b1f4b05ff21581047e3f91e",
            "code": "0x3373fffffffffffffffffffffffffffffffffffffffe14604d57602036146024575f5ffd5b5f35801560495762001fff810690815414603c575f5ffd5b62001fff01545f5260205ff35b5f5ffd5b62001fff42064281555f359062001fff015500"
        }
    ]' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Configure ERC20 native precompiles
    jq '.app_state.erc20.native_precompiles = ["0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE"]' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Configure ERC20 token pairs for native token
    jq --arg denom "$DENOM" '.app_state.erc20.token_pairs = [{
        "contract_owner": 1,
        "erc20_address": "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE",
        "denom": $denom,
        "enabled": true
    }]' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
}

# Create validator
create_validator() {
    log_info "Creating validator..."

    # Create validator key
    VALIDATOR_KEY="validator"
    if ! epixd keys show "$VALIDATOR_KEY" --keyring-backend "$KEYRING" --home "$CHAINDIR" &>/dev/null; then
        epixd keys add "$VALIDATOR_KEY" --keyring-backend "$KEYRING" --home "$CHAINDIR"
    fi

    # Add validator account to genesis with 2 EPIX
    epixd genesis add-genesis-account "$VALIDATOR_KEY" "${VALIDATOR_ALLOCATION}${DENOM}" --keyring-backend "$KEYRING" --home "$CHAINDIR"

    # Create genesis transaction for validator with 1 EPIX stake
    STAKE_AMOUNT="1000000000000000000${DENOM}"  # 1 EPIX stake
    epixd genesis gentx "$VALIDATOR_KEY" "$STAKE_AMOUNT" \
        --chain-id "$(jq -r '.chain_id' "$GENESIS")" \
        --keyring-backend "$KEYRING" \
        --home "$CHAINDIR"

    # Collect genesis transactions
    epixd genesis collect-gentxs --home "$CHAINDIR"

    VALIDATOR_ADDRESS=$(epixd keys show "$VALIDATOR_KEY" -a --keyring-backend "$KEYRING" --home "$CHAINDIR")
    log_info "Validator address: $VALIDATOR_ADDRESS"
    log_info "Validator stake: 1 EPIX"
}

# Set gas limits and block parameters
configure_consensus() {
    log_info "Configuring consensus parameters..."

    # Set block gas limit
    jq '.consensus.params.block.max_gas = "30000000"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Set block size limit
    jq '.consensus.params.block.max_bytes = "22020096"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
}

# Configure node settings for development
configure_node_settings() {
    log_info "Configuring node settings for development..."

    # Set pruning to nothing (keep all data)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' 's/pruning = "default"/pruning = "nothing"/' "$APP_TOML"
        sed -i '' 's/pruning = "everything"/pruning = "nothing"/' "$APP_TOML"
        sed -i '' 's/pruning = "custom"/pruning = "nothing"/' "$APP_TOML"
    else
        sed -i 's/pruning = "default"/pruning = "nothing"/' "$APP_TOML"
        sed -i 's/pruning = "everything"/pruning = "nothing"/' "$APP_TOML"
        sed -i 's/pruning = "custom"/pruning = "nothing"/' "$APP_TOML"
    fi

    # Configure JSON-RPC APIs
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' 's/api = "eth,net,web3"/api = "eth,txpool,personal,net,debug,web3"/' "$APP_TOML"
        sed -i '' 's/api = "eth,web3,net"/api = "eth,txpool,personal,net,debug,web3"/' "$APP_TOML"
        sed -i '' 's/api = "web3,eth,net"/api = "eth,txpool,personal,net,debug,web3"/' "$APP_TOML"
    else
        sed -i 's/api = "eth,net,web3"/api = "eth,txpool,personal,net,debug,web3"/' "$APP_TOML"
        sed -i 's/api = "eth,web3,net"/api = "eth,txpool,personal,net,debug,web3"/' "$APP_TOML"
        sed -i 's/api = "web3,eth,net"/api = "eth,txpool,personal,net,debug,web3"/' "$APP_TOML"
    fi

    # Enable unsafe CORS for development
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' 's/enabled-unsafe-cors = false/enabled-unsafe-cors = true/' "$APP_TOML"
    else
        sed -i 's/enabled-unsafe-cors = false/enabled-unsafe-cors = true/' "$APP_TOML"
    fi

    # Set API and JSON-RPC addresses to 0.0.0.0 for external access
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # Set API server address
        sed -i '' 's/address = "tcp:\/\/localhost:1317"/address = "tcp:\/\/0.0.0.0:1317"/' "$APP_TOML"
        sed -i '' 's/address = "tcp:\/\/127.0.0.1:1317"/address = "tcp:\/\/0.0.0.0:1317"/' "$APP_TOML"
        # Set JSON-RPC address
        sed -i '' 's/address = "127.0.0.1:8545"/address = "0.0.0.0:8545"/' "$APP_TOML"
        sed -i '' 's/address = "localhost:8545"/address = "0.0.0.0:8545"/' "$APP_TOML"
        # Set JSON-RPC WS address
        sed -i '' 's/ws-address = "127.0.0.1:8546"/ws-address = "0.0.0.0:8546"/' "$APP_TOML"
        sed -i '' 's/ws-address = "localhost:8546"/ws-address = "0.0.0.0:8546"/' "$APP_TOML"
        # Set RPC laddr in config.toml
        sed -i '' 's/laddr = "tcp:\/\/127.0.0.1:26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' "$CONFIG_TOML"
        sed -i '' 's/laddr = "tcp:\/\/localhost:26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' "$CONFIG_TOML"
    else
        # Set API server address
        sed -i 's/address = "tcp:\/\/localhost:1317"/address = "tcp:\/\/0.0.0.0:1317"/' "$APP_TOML"
        sed -i 's/address = "tcp:\/\/127.0.0.1:1317"/address = "tcp:\/\/0.0.0.0:1317"/' "$APP_TOML"
        # Set JSON-RPC address
        sed -i 's/address = "127.0.0.1:8545"/address = "0.0.0.0:8545"/' "$APP_TOML"
        sed -i 's/address = "localhost:8545"/address = "0.0.0.0:8545"/' "$APP_TOML"
        # Set JSON-RPC WS address
        sed -i 's/ws-address = "127.0.0.1:8546"/ws-address = "0.0.0.0:8546"/' "$APP_TOML"
        sed -i 's/ws-address = "localhost:8546"/ws-address = "0.0.0.0:8546"/' "$APP_TOML"
        # Set RPC laddr in config.toml
        sed -i 's/laddr = "tcp:\/\/127.0.0.1:26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' "$CONFIG_TOML"
        sed -i 's/laddr = "tcp:\/\/localhost:26657"/laddr = "tcp:\/\/0.0.0.0:26657"/' "$CONFIG_TOML"
    fi

    # Enable prometheus metrics
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' 's/prometheus = false/prometheus = true/' "$CONFIG_TOML"
        sed -i '' 's/prometheus-retention-time = 0/prometheus-retention-time  = 1000000000000/g' "$APP_TOML"
        sed -i '' 's/enabled = false/enabled = true/g' "$APP_TOML"
        sed -i '' 's/enable = false/enable = true/g' "$APP_TOML"
    else
        sed -i 's/prometheus = false/prometheus = true/' "$CONFIG_TOML"
        sed -i 's/prometheus-retention-time  = "0"/prometheus-retention-time  = "1000000000000"/g' "$APP_TOML"
        sed -i 's/enabled = false/enabled = true/g' "$APP_TOML"
        sed -i 's/enable = false/enable = true/g' "$APP_TOML"
    fi

    # Enable unsafe RPC endpoints for development (required for dump_consensus_state)
    if [[ "$OSTYPE" == "darwin"* ]]; then
        sed -i '' 's/unsafe = false/unsafe = true/' "$CONFIG_TOML"
        sed -i '' 's/cors_allowed_origins = \[\]/cors_allowed_origins = ["*"]/' "$CONFIG_TOML"
    else
        sed -i 's/unsafe = false/unsafe = true/' "$CONFIG_TOML"
        sed -i 's/cors_allowed_origins = \[\]/cors_allowed_origins = ["*"]/' "$CONFIG_TOML"
    fi

    log_info "Node settings configured for development:"
    log_info "- Pruning: nothing (keep all data)"
    log_info "- JSON-RPC APIs: eth,txpool,personal,net,debug,web3"
    log_info "- CORS: enabled-unsafe-cors = true"
    log_info "- API address: 0.0.0.0:1317 (external access)"
    log_info "- JSON-RPC address: 0.0.0.0:8545 (external access)"
    log_info "- JSON-RPC WS address: 0.0.0.0:8546 (external access)"
    log_info "- RPC address: 0.0.0.0:26657 (external access)"
    log_info "- Prometheus metrics and APIs enabled"
    log_info "- Unsafe RPC endpoints enabled (for dump_consensus_state)"
    log_info "- RPC CORS allowed origins: [\"*\"] (all origins)"
}

# Deploy WEPIX (Wrapped EPIX) contract
deploy_wepix() {
    log_info "Configuring WEPIX (Wrapped EPIX) deployment..."

    # WEPIX contract address (deterministic deployment address)
    WEPIX_ADDRESS="0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"  # Standard WETH address pattern

    # Add WEPIX to ERC20 token pairs
    jq --arg wepix_addr "$WEPIX_ADDRESS" --arg denom "$DENOM" '
        .app_state.erc20.token_pairs += [{
            "contract_owner": 1,
            "erc20_address": $wepix_addr,
            "denom": ("w" + $denom),
            "enabled": true
        }]
    ' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    # Add WEPIX to native precompiles
    jq --arg wepix_addr "$WEPIX_ADDRESS" '
        .app_state.erc20.native_precompiles += [$wepix_addr]
    ' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

    log_info "WEPIX contract configured at address: $WEPIX_ADDRESS"
    log_info "WEPIX denomination: w$DENOM"
}

# Generate comprehensive contract addresses report
generate_contract_addresses_report() {
    log_info "📋 EPIX CHAIN CONTRACT ADDRESSES REPORT"
    log_info "========================================"
    log_info ""

    log_info "🏭 PREINSTALLED CONTRACTS (Available at Genesis):"
    log_info "  Create2 Factory:           0x4e59b44847b379578588920ca78fbf26c0b4956c"
    log_info "  Multicall3:                0xcA11bde05977b3631167028862bE2a173976CA11"
    log_info "  Permit2:                   0x000000000022D473030F116dDEE9F6B43aC78BA3"
    log_info "  Safe Singleton Factory:    0x914d7Fec6aaC8cd542e72Bca78B30650d45643d7"
    log_info "  EIP-2935 History Storage:  0x0aae40965e6800cd9b1f4b05ff21581047e3f91e"
    log_info ""

    log_info "🔧 COSMOS MODULE PRECOMPILES (Stateful):"
    log_info "  Distribution Module:       0x0000000000000000000000000000000000000100"
    log_info "  Staking Module:            0x0000000000000000000000000000000000000400"
    log_info "  IBC Transfer Module:       0x0000000000000000000000000000000000000800"
    log_info "  ICS20 Module:              0x0000000000000000000000000000000000000801"
    log_info "  IBC Channel Module:        0x0000000000000000000000000000000000000802"
    log_info "  IBC Client Module:         0x0000000000000000000000000000000000000803"
    log_info "  IBC Connection Module:     0x0000000000000000000000000000000000000804"
    log_info "  IBC Port Module:           0x0000000000000000000000000000000000000805"
    log_info "  Authorization Module:      0x0000000000000000000000000000000000000806"
    log_info "  Bank Module:               0x0000000000000000000000000000000000000807"
    log_info ""

    log_info "🪙 ERC20 NATIVE TOKEN PRECOMPILES:"
    log_info "  Native EPIX Token:         0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE"
    log_info "  WEPIX (Wrapped EPIX):      0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"
    log_info ""

    log_info "🏛️ COSMOS MODULE ADDRESSES (Bech32):"
    log_info "  Distribution Module:       epix1jv65s3grqf6v6jl3dp4t6c9t9rk99cd8j52fwy"
    log_info "  Validator Address:         $(epixd keys show validator -a --keyring-backend $KEYRING --home $CHAINDIR 2>/dev/null || echo 'Not created yet')"
    log_info ""

    log_info "📋 CONTRACT FUNCTIONALITY SUMMARY:"
    log_info "  • Create2: Deterministic contract deployment"
    log_info "  • Multicall3: Batch multiple contract calls in single transaction"
    log_info "  • Permit2: Universal token approval system"
    log_info "  • Safe Factory: Gnosis Safe wallet deployment"
    log_info "  • EIP-2935: Historical block hash access"
    log_info "  • Distribution: Claim staking rewards, withdraw delegator rewards"
    log_info "  • Staking: Delegate, undelegate, redelegate tokens"
    log_info "  • IBC Modules: Cross-chain communication and token transfers"
    log_info "  • Authorization: Grant and execute authorizations"
    log_info "  • Bank: Send tokens, check balances"
    log_info ""

    log_info "🌐 WEB3 INTEGRATION:"
    log_info "  • All contracts are accessible via Ethereum JSON-RPC"
    log_info "  • Compatible with MetaMask, Hardhat, Foundry, and other tools"
    log_info "  • Native EPIX can be used directly in EVM transactions"
    log_info "  • WEPIX provides standard ERC20 wrapper functionality"
    log_info "  • Solidity contracts can interact with Cosmos modules via precompiles"
    log_info ""

    log_info "📝 USAGE EXAMPLES:"
    log_info "  # Query Multicall3 contract"
    log_info "  curl -X POST $TESTNET_RPC:8545 -H 'Content-Type: application/json' \\"
    log_info "    -d '{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"0xcA11bde05977b3631167028862bE2a173976CA11\",\"latest\"],\"id\":1}'"
    log_info ""
    log_info "  # Check native EPIX balance"
    log_info "  curl -X POST $TESTNET_RPC:8545 -H 'Content-Type: application/json' \\"
    log_info "    -d '{\"jsonrpc\":\"2.0\",\"method\":\"eth_getBalance\",\"params\":[\"YOUR_ADDRESS\",\"latest\"],\"id\":1}'"
    log_info ""

    log_info "========================================"
}

# Output contract addresses in machine-readable format
output_contract_addresses() {
    log_info ""
    log_info "📄 CONTRACT ADDRESSES (Machine Readable Format):"
    log_info "================================================="

    # Output directly to stdout (not through log_info) so they can be captured
    cat << 'EOF'
CREATE2_FACTORY=0x4e59b44847b379578588920ca78fbf26c0b4956c
MULTICALL3=0xcA11bde05977b3631167028862bE2a173976CA11
PERMIT2=0x000000000022D473030F116dDEE9F6B43aC78BA3
SAFE_SINGLETON_FACTORY=0x914d7Fec6aaC8cd542e72Bca78B30650d45643d7
EIP2935_HISTORY_STORAGE=0x0aae40965e6800cd9b1f4b05ff21581047e3f91e
DISTRIBUTION_PRECOMPILE=0x0000000000000000000000000000000000000100
STAKING_PRECOMPILE=0x0000000000000000000000000000000000000400
IBC_TRANSFER_PRECOMPILE=0x0000000000000000000000000000000000000800
ICS20_PRECOMPILE=0x0000000000000000000000000000000000000801
IBC_CHANNEL_PRECOMPILE=0x0000000000000000000000000000000000000802
IBC_CLIENT_PRECOMPILE=0x0000000000000000000000000000000000000803
IBC_CONNECTION_PRECOMPILE=0x0000000000000000000000000000000000000804
IBC_PORT_PRECOMPILE=0x0000000000000000000000000000000000000805
AUTHORIZATION_PRECOMPILE=0x0000000000000000000000000000000000000806
BANK_PRECOMPILE=0x0000000000000000000000000000000000000807
NATIVE_EPIX_TOKEN=0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE
WEPIX_TOKEN=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2
DISTRIBUTION_MODULE_ADDRESS=epix1jv65s3grqf6v6jl3dp4t6c9t9rk99cd8j52fwy
EOF

    # Dynamic addresses (if available)
    if epixd keys show validator -a --keyring-backend "$KEYRING" --home "$CHAINDIR" &>/dev/null; then
        VALIDATOR_ADDRESS=$(epixd keys show validator -a --keyring-backend "$KEYRING" --home "$CHAINDIR")
        echo "VALIDATOR_ADDRESS=$VALIDATOR_ADDRESS"
    fi

    log_info "================================================="
    log_info "💡 TIP: You can source these addresses in your scripts:"
    log_info "   eval \$(./scripts/setup_epix_chain.sh --addresses-only)"
}

# Output just the contract addresses (no logging)
output_addresses_only() {
    cat << 'EOF'
CREATE2_FACTORY=0x4e59b44847b379578588920ca78fbf26c0b4956c
MULTICALL3=0xcA11bde05977b3631167028862bE2a173976CA11
PERMIT2=0x000000000022D473030F116dDEE9F6B43aC78BA3
SAFE_SINGLETON_FACTORY=0x914d7Fec6aaC8cd542e72Bca78B30650d45643d7
EIP2935_HISTORY_STORAGE=0x0aae40965e6800cd9b1f4b05ff21581047e3f91e
DISTRIBUTION_PRECOMPILE=0x0000000000000000000000000000000000000100
STAKING_PRECOMPILE=0x0000000000000000000000000000000000000400
IBC_TRANSFER_PRECOMPILE=0x0000000000000000000000000000000000000800
ICS20_PRECOMPILE=0x0000000000000000000000000000000000000801
IBC_CHANNEL_PRECOMPILE=0x0000000000000000000000000000000000000802
IBC_CLIENT_PRECOMPILE=0x0000000000000000000000000000000000000803
IBC_CONNECTION_PRECOMPILE=0x0000000000000000000000000000000000000804
IBC_PORT_PRECOMPILE=0x0000000000000000000000000000000000000805
AUTHORIZATION_PRECOMPILE=0x0000000000000000000000000000000000000806
BANK_PRECOMPILE=0x0000000000000000000000000000000000000807
NATIVE_EPIX_TOKEN=0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE
WEPIX_TOKEN=0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2
DISTRIBUTION_MODULE_ADDRESS=epix1jv65s3grqf6v6jl3dp4t6c9t9rk99cd8j52fwy
EOF

    # Dynamic addresses (if available)
    if epixd keys show validator -a --keyring-backend "$KEYRING" --home "$CHAINDIR" &>/dev/null; then
        VALIDATOR_ADDRESS=$(epixd keys show validator -a --keyring-backend "$KEYRING" --home "$CHAINDIR")
        echo "VALIDATOR_ADDRESS=$VALIDATOR_ADDRESS"
    fi
}

# Verify contracts exist on chain by querying their bytecode
verify_contracts_on_chain() {
    log_info "🔍 VERIFYING CONTRACTS ON CHAIN"
    log_info "==============================="

    # Always use localhost for contract verification since we're checking the local chain
    # The --verify-contracts option is meant to verify the locally configured chain
    local RPC_URL="http://localhost:8545"

    log_info "Using RPC endpoint: $RPC_URL"

    # Test basic connectivity
    log_info "Testing RPC connectivity..."
    local test_response=$(curl -s -X POST "$RPC_URL" \
        -H "Content-Type: application/json" \
        -d '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' 2>/dev/null)

    if [[ $? -ne 0 ]] || [[ -z "$test_response" ]]; then
        log_error "❌ Cannot connect to RPC endpoint: $RPC_URL"
        log_error "   Make sure the chain is running and the endpoint is accessible"
        return 1
    fi

    # Check if we got a valid response
    if echo "$test_response" | grep -q '"result"'; then
        log_info "✅ RPC endpoint is accessible"
    else
        log_warn "⚠️  RPC endpoint responded but may have issues"
        log_warn "   Response: $test_response"
    fi
    log_info ""

    # Function to check if a contract exists
    check_contract() {
        local name="$1"
        local address="$2"
        local expected_type="$3"

        log_info "Checking $name at $address..."

        # Query the contract bytecode
        local response=$(curl -s -X POST "$RPC_URL" \
            -H "Content-Type: application/json" \
            -d "{\"jsonrpc\":\"2.0\",\"method\":\"eth_getCode\",\"params\":[\"$address\",\"latest\"],\"id\":1}" 2>/dev/null)

        if [[ $? -ne 0 ]]; then
            log_error "  ❌ Failed to connect to RPC endpoint"
            return 1
        fi

        # Debug: show raw response if it's short (likely an error)
        if [[ ${#response} -lt 200 ]]; then
            log_warn "  🔍 Raw response: $response"
        fi

        # Parse the response - handle both jq and manual parsing
        local code=""
        if command -v jq &> /dev/null; then
            code=$(echo "$response" | jq -r '.result' 2>/dev/null)
        else
            # Manual parsing if jq is not available
            code=$(echo "$response" | grep -o '"result":"[^"]*"' | cut -d'"' -f4)
        fi

        if [[ "$code" == "null" ]] || [[ "$code" == "" ]] || [[ "$code" == "undefined" ]]; then
            # Check if there's an error in the response
            local error=$(echo "$response" | grep -o '"error":[^}]*}' 2>/dev/null)
            if [[ -n "$error" ]]; then
                log_error "  ❌ RPC Error: $error"
            else
                log_error "  ❌ Failed to get response or parse JSON"
            fi
            return 1
        elif [[ "$code" == "0x" ]]; then
            log_warn "  ⚠️  No bytecode found (contract not deployed or is EOA)"
            return 1
        else
            local code_length=${#code}
            log_info "  ✅ Contract exists! Bytecode length: $((code_length - 2)) bytes"

            # Additional checks for specific contract types
            case "$expected_type" in
                "multicall")
                    if echo "$code" | grep -q "aggregate"; then
                        log_info "     🎯 Confirmed: Contains multicall functionality"
                    fi
                    ;;
                "create2")
                    if echo "$code" | grep -q "ff"; then
                        log_info "     🎯 Confirmed: Contains CREATE2 opcode patterns"
                    fi
                    ;;
                "precompile")
                    log_info "     🎯 Confirmed: Precompile contract active"
                    ;;
            esac
            return 0
        fi
    }

    # Function to check Cosmos module precompiles via REST API
    check_cosmos_precompiles() {
        local REST_URL="http://localhost:1317"

        log_info "Using REST API endpoint: $REST_URL"

        # Test REST API connectivity
        local test_response=$(curl -s "$REST_URL/cosmos/base/tendermint/v1beta1/node_info" 2>/dev/null)
        if [[ $? -ne 0 ]] || [[ -z "$test_response" ]]; then
            log_warn "  ⚠️  Cannot connect to REST API endpoint: $REST_URL"
            log_warn "     Falling back to EVM bytecode checks for precompiles..."

            # Fallback to EVM checks
            check_contract "Distribution Module" "0x0000000000000000000000000000000000000100" "precompile"
            check_contract "Staking Module" "0x0000000000000000000000000000000000000400" "precompile"
            check_contract "Bank Module" "0x0000000000000000000000000000000000000807" "precompile"
            return
        fi

        log_info "  ✅ REST API is accessible"
        log_info ""

        # Check Distribution Module
        log_info "Checking Distribution Module (rewards/delegation)..."
        local dist_response=$(curl -s "$REST_URL/cosmos/distribution/v1beta1/params" 2>/dev/null)
        if echo "$dist_response" | grep -q '"community_tax"'; then
            log_info "  ✅ Distribution module active - community tax configured"
        else
            log_warn "  ⚠️  Distribution module response unclear"
        fi

        # Check Staking Module
        log_info "Checking Staking Module (validators/delegations)..."
        local staking_response=$(curl -s "$REST_URL/cosmos/staking/v1beta1/params" 2>/dev/null)
        if echo "$staking_response" | grep -q '"bond_denom"'; then
            local bond_denom=$(echo "$staking_response" | grep -o '"bond_denom":"[^"]*"' | cut -d'"' -f4)
            log_info "  ✅ Staking module active - bond denom: $bond_denom"
        else
            log_warn "  ⚠️  Staking module response unclear"
        fi

        # Check Bank Module
        log_info "Checking Bank Module (token transfers)..."
        local bank_response=$(curl -s "$REST_URL/cosmos/bank/v1beta1/params" 2>/dev/null)
        if echo "$bank_response" | grep -q '"send_enabled"'; then
            log_info "  ✅ Bank module active - transfers enabled"
        else
            log_warn "  ⚠️  Bank module response unclear"
        fi

        # Check Gov Module (Authorization precompile uses this)
        log_info "Checking Gov Module (governance/authorization)..."
        local gov_response=$(curl -s "$REST_URL/cosmos/gov/v1beta1/params/voting" 2>/dev/null)
        if echo "$gov_response" | grep -q '"voting_period"'; then
            log_info "  ✅ Gov module active - voting configured"
        else
            log_warn "  ⚠️  Gov module response unclear"
        fi

        # Check IBC Transfer Module
        log_info "Checking IBC Transfer Module (cross-chain transfers)..."
        local ibc_response=$(curl -s "$REST_URL/ibc/apps/transfer/v1/params" 2>/dev/null)
        if echo "$ibc_response" | grep -q '"send_enabled"' || echo "$ibc_response" | grep -q '"receive_enabled"'; then
            log_info "  ✅ IBC Transfer module active"
        else
            log_warn "  ⚠️  IBC Transfer module response unclear"
        fi

        # Check EVM Module (confirms precompiles are configured)
        log_info "Checking EVM Module (precompile configuration)..."
        local evm_response=$(curl -s "$REST_URL/cosmos/evm/vm/v1/params" 2>/dev/null)
        if echo "$evm_response" | grep -q '"active_static_precompiles"'; then
            local precompile_count=$(echo "$evm_response" | grep -o '"0x[^"]*"' | wc -l)
            log_info "  ✅ EVM module active - $precompile_count precompiles configured"
        else
            log_warn "  ⚠️  EVM module response unclear"
        fi

        log_info ""
        log_info "📋 Cosmos Module Status Summary:"
        log_info "  • All modules are accessible via REST API"
        log_info "  • Precompiles are handled by active Cosmos modules"
        log_info "  • No bytecode needed - they're native Go implementations"
    }

    # Check preinstalled contracts
    log_info "📦 PREINSTALLED CONTRACTS:"
    check_contract "Create2 Factory" "0x4e59b44847b379578588920ca78fbf26c0b4956c" "create2"
    check_contract "Multicall3" "0xcA11bde05977b3631167028862bE2a173976CA11" "multicall"
    check_contract "Permit2" "0x000000000022D473030F116dDEE9F6B43aC78BA3" "permit"
    check_contract "Safe Singleton Factory" "0x914d7Fec6aaC8cd542e72Bca78B30650d45643d7" "factory"
    check_contract "EIP-2935 History Storage" "0x0aae40965e6800cd9b1f4b05ff21581047e3f91e" "history"

    log_info ""
    log_info "🔧 COSMOS MODULE PRECOMPILES (via REST API):"
    check_cosmos_precompiles

    log_info ""
    log_info "🪙 ERC20 NATIVE PRECOMPILES:"
    check_contract "Native EPIX Token" "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE" "precompile"
    check_contract "WEPIX (Wrapped EPIX)" "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2" "precompile"

    log_info ""
    log_info "==============================="
    log_info "💡 TIP: If contracts show as not deployed, make sure:"
    log_info "   1. The chain is running: epixd start"
    log_info "   2. The RPC endpoint is accessible: $RPC_URL"
    log_info "   3. The genesis configuration included the preinstalls"
}

# Validate genesis file
validate_genesis() {
    log_info "Validating genesis file..."

    if epixd genesis validate-genesis --home "$CHAINDIR"; then
        log_info "Genesis file validation successful!"
    else
        log_error "Genesis file validation failed!"
        exit 1
    fi
}

# Configure genesis file
configure_genesis() {
    log_info "Starting Epix Genesis Configuration"
    log_info "==================================="

    configure_chain_params
    configure_minting
    configure_governance
    configure_slashing
    process_airdrop_csv
    configure_community_pool
    configure_total_supply
    configure_evm
    deploy_wepix
    configure_consensus
    create_validator
    configure_node_settings
    validate_genesis
    generate_contract_addresses_report
    output_contract_addresses

    log_info "Epix genesis configuration completed successfully!"
}

# Display summary
show_summary() {
    log_info ""
    log_info "=== Epix Chain Setup Summary ==="
    log_info "Chain ID: $CHAIN_ID"
    log_info "Network: $NETWORK"
    log_info "Home Directory: $CHAINDIR"
    log_info "Chain denomination: $DENOM"
    log_info "Display denomination: $DISPLAY_DENOM"

    if [[ "$RUN_CONFIGURE" == "true" ]]; then
        log_info "Genesis supply: $(echo "scale=0; $GENESIS_SUPPLY / 10^18" | bc) EPIX"
        log_info "Airdrop allocation: $(echo "scale=0; $AIRDROP_ALLOCATION / 10^18" | bc) EPIX (from CSV snapshot)"
        log_info "Community pool: $(echo "scale=0; $COMMUNITY_POOL_ALLOCATION / 10^18" | bc) EPIX"
        log_info "Validator allocation: 2 EPIX (1 staked, 1 remaining)"
        log_info "Initial annual mint amount: $(echo "scale=0; $INITIAL_ANNUAL_MINT_AMOUNT / 10^18" | bc) EPIX (year 1)"
        log_info "Annual reduction rate: $(echo "scale=1; $ANNUAL_REDUCTION_RATE * 100" | bc)%"
        log_info "Maximum supply: $(echo "scale=0; $MAX_SUPPLY / 10^18" | bc) EPIX"
        log_info "Initial inflation: $(echo "scale=2; $INITIAL_INFLATION * 100" | bc)%"
        log_info "Target bonded ratio: $(echo "scale=1; $GOAL_BONDED * 100" | bc)%"
        log_info ""
        log_info "EVM Features:"
        log_info "- Native ERC20 precompiles enabled"
        log_info "- WEPIX (Wrapped EPIX) configured"
        log_info "- Complete precompile list activated"
        log_info ""
        log_info "Development Features:"
        log_info "- Prometheus metrics enabled"
        log_info "- All APIs enabled (eth, txpool, personal, net, debug, web3)"
        log_info ""
        log_info "Genesis file configured at: $GENESIS"
    fi

    log_info ""
    log_info "Ready to start the chain with:"
    log_info "epixd start --home $CHAINDIR \\"
    log_info "  --pruning nothing \\"
    log_info "  --minimum-gas-prices=0.0001$DENOM \\"
    log_info "  --json-rpc.api eth,txpool,personal,net,debug,web3 \\"
    log_info "  --chain-id $CHAIN_ID"
}

# Main execution
main() {
    log_info "Starting Epix Chain Setup"
    log_info "========================="

    parse_args "$@"
    check_dependencies

    log_info "Operation: $(if [[ "$RUN_INIT" == "true" && "$RUN_CONFIGURE" == "true" ]]; then echo "Full setup"; elif [[ "$RUN_INIT" == "true" ]]; then echo "Initialization only"; else echo "Configuration only"; fi)"
    log_info "Network: $NETWORK"
    log_info "Chain ID: $CHAIN_ID"
    log_info "Home Directory: $CHAINDIR"

    if [[ "$RUN_INIT" == "true" ]]; then
        init_chain
    fi

    if [[ "$RUN_CONFIGURE" == "true" ]]; then
        configure_genesis
    fi

    show_summary

    log_info "Epix chain setup completed successfully!"
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
