#!/bin/bash

# Epix Chain Contract Addresses Display Script
# Shows all important contract addresses available on Epix Chain

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Print colored output
print_header() {
    echo -e "${GREEN}$1${NC}"
}

print_section() {
    echo -e "${BLUE}$1${NC}"
}

print_address() {
    echo -e "${CYAN}  $1${NC}"
}

print_info() {
    echo -e "${YELLOW}$1${NC}"
}

# Help function
show_help() {
    cat << EOF
Epix Chain Contract Addresses

Usage: $0 [OPTIONS]

OPTIONS:
    --network TYPE    Show addresses for specific network (mainnet/testnet)
    --json           Output in JSON format
    --copy           Show copy commands for easy use
    --help           Show this help message

EXAMPLES:
    # Show all addresses
    $0

    # Show addresses in JSON format
    $0 --json

    # Show with copy commands
    $0 --copy

EOF
}

# Default values
NETWORK_TYPE=""
JSON_OUTPUT=false
SHOW_COPY=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --network)
            NETWORK_TYPE="$2"
            shift 2
            ;;
        --json)
            JSON_OUTPUT=true
            shift
            ;;
        --copy)
            SHOW_COPY=true
            shift
            ;;
        --help)
            show_help
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Contract addresses
WEPIX_ADDRESS="0x211781849EF6de72acbf1469Ce3808E74D7ce158"
MULTICALL3_ADDRESS="0xcA11bde05977b3631167028862bE2a173976CA11"
CREATE2_FACTORY="0x4e59b44847b379578588920ca78fbf26c0b4956c"
PERMIT2_ADDRESS="0x000000000022D473030F116dDEE9F6B43aC78BA3"
SAFE_FACTORY="0x914d7Fec6aaC8cd542e72Bca78B30650d45643d7"

# Precompile addresses
STAKING_PRECOMPILE="0x0000000000000000000000000000000000000800"
DISTRIBUTION_PRECOMPILE="0x0000000000000000000000000000000000000801"
ICS20_PRECOMPILE="0x0000000000000000000000000000000000000802"
VESTING_PRECOMPILE="0x0000000000000000000000000000000000000803"
BANK_PRECOMPILE="0x0000000000000000000000000000000000000804"
GOV_PRECOMPILE="0x0000000000000000000000000000000000000805"
SLASHING_PRECOMPILE="0x0000000000000000000000000000000000000806"
BECH32_PRECOMPILE="0x0000000000000000000000000000000000000400"
P256_PRECOMPILE="0x0000000000000000000000000000000000000100"

if [ "$JSON_OUTPUT" = true ]; then
    # JSON output
    cat << EOF
{
  "network": "${NETWORK_TYPE:-"all"}",
  "native_tokens": {
    "WEPIX": "$WEPIX_ADDRESS"
  },
  "utility_contracts": {
    "MultiCall3": "$MULTICALL3_ADDRESS",
    "Create2Factory": "$CREATE2_FACTORY",
    "Permit2": "$PERMIT2_ADDRESS",
    "SafeSingletonFactory": "$SAFE_FACTORY"
  },
  "cosmos_precompiles": {
    "Staking": "$STAKING_PRECOMPILE",
    "Distribution": "$DISTRIBUTION_PRECOMPILE",
    "ICS20": "$ICS20_PRECOMPILE",
    "Vesting": "$VESTING_PRECOMPILE",
    "Bank": "$BANK_PRECOMPILE",
    "Governance": "$GOV_PRECOMPILE",
    "Slashing": "$SLASHING_PRECOMPILE",
    "Bech32": "$BECH32_PRECOMPILE",
    "P256Verify": "$P256_PRECOMPILE"
  }
}
EOF
else
    # Human-readable output
    echo
    print_header "🌟 Epix Chain Contract Addresses"
    echo
    
    if [ -n "$NETWORK_TYPE" ]; then
        print_info "Network: $NETWORK_TYPE"
        echo
    fi

    print_section "🪙 Native Token Contracts:"
    print_address "WEPIX (Wrapped EPIX):     $WEPIX_ADDRESS"
    echo

    print_section "🛠️  Utility Contracts (Available at Genesis):"
    print_address "MultiCall3:               $MULTICALL3_ADDRESS"
    print_address "Create2 Factory:          $CREATE2_FACTORY"
    print_address "Permit2:                  $PERMIT2_ADDRESS"
    print_address "Safe Singleton Factory:   $SAFE_FACTORY"
    echo

    print_section "⚡ Cosmos Precompiles:"
    print_address "Staking:                  $STAKING_PRECOMPILE"
    print_address "Distribution:             $DISTRIBUTION_PRECOMPILE"
    print_address "ICS20 (IBC Transfer):     $ICS20_PRECOMPILE"
    print_address "Vesting:                  $VESTING_PRECOMPILE"
    print_address "Bank:                     $BANK_PRECOMPILE"
    print_address "Governance:               $GOV_PRECOMPILE"
    print_address "Slashing:                 $SLASHING_PRECOMPILE"
    print_address "Bech32:                   $BECH32_PRECOMPILE"
    print_address "P256 Verify:              $P256_PRECOMPILE"

    if [ "$SHOW_COPY" = true ]; then
        echo
        print_section "📋 Copy Commands:"
        echo "# Export as environment variables"
        echo "export WEPIX_ADDRESS=\"$WEPIX_ADDRESS\""
        echo "export MULTICALL3_ADDRESS=\"$MULTICALL3_ADDRESS\""
        echo "export CREATE2_FACTORY=\"$CREATE2_FACTORY\""
        echo "export PERMIT2_ADDRESS=\"$PERMIT2_ADDRESS\""
        echo "export SAFE_FACTORY=\"$SAFE_FACTORY\""
        echo
        echo "# Test commands (replace RPC_URL with your endpoint)"
        echo "cast call \$WEPIX_ADDRESS \"name()\" --rpc-url \$RPC_URL"
        echo "cast call \$WEPIX_ADDRESS \"symbol()\" --rpc-url \$RPC_URL"
        echo "cast call \$WEPIX_ADDRESS \"decimals()\" --rpc-url \$RPC_URL"
    fi

    echo
    print_section "📚 Usage Examples:"
    echo "  # Test WEPIX functionality"
    echo "  cast call $WEPIX_ADDRESS \"name()\" --rpc-url http://localhost:8545"
    echo "  cast call $WEPIX_ADDRESS \"balanceOf(address)\" \$USER_ADDRESS --rpc-url http://localhost:8545"
    echo
    echo "  # Use MultiCall3 for batch operations"
    echo "  cast call $MULTICALL3_ADDRESS \"getBlockNumber()\" --rpc-url http://localhost:8545"
    echo
    echo "  # Deploy contracts using Create2"
    echo "  # (See Create2 factory documentation for usage)"
    echo
    echo "  # Interact with Cosmos modules via precompiles"
    echo "  cast call $STAKING_PRECOMPILE \"delegation(address,string)\" \$DELEGATOR \$VALIDATOR --rpc-url http://localhost:8545"

    echo
    print_info "💡 All these contracts are available immediately when the Epix chain starts!"
    print_info "   No deployment needed - they're built into the genesis state."
    echo
fi
