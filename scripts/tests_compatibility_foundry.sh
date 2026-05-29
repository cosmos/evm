#!/usr/bin/env bash

# CI script for running foundry compatibility tests
# This script sets up dependencies, submodules, and runs the required forge script commands
# Usage: ./tests_compatibility_foundry.sh [--verbose] [--node-log-print]

set -eo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/tests_compatibility_common.sh
source "$SCRIPT_DIR/tests_compatibility_common.sh"

VERBOSE=false
NODE_LOG_PRINT=false

# Parse command line arguments
while [[ $# -gt 0 ]]; do
	case $1 in
	--verbose | -v)
		VERBOSE=true
		shift
		;;
	--node-log-print)
		NODE_LOG_PRINT=true
		shift
		;;
	*)
		echo "Unknown option: $1"
		echo "Usage: $0 [--verbose] [--node-log-print]"
		exit 1
		;;
	esac
done

ROOT="$(git rev-parse --show-toplevel)"
TEST_DIR="$ROOT/tests/evm-tools-compatibility/foundry"

echo "Setting up foundry compatibility tests..."
setup_compatibility_tests "$NODE_LOG_PRINT"

start_node "$NODE_LOG_PRINT"
trap cleanup_node EXIT
sleep 3

wait_for_node 10

# Change to the test directory
cd "$TEST_DIR"

# Source the environment file
if [ -f ".env" ]; then
	echo "Sourcing .env file..."
	# shellcheck source=/dev/null
	source .env
else
	echo "Error: No .env file found in $TEST_DIR"
	exit 1
fi

# Verify required environment variables are set
if [ -z "$CUSTOM_RPC" ] || [ -z "$CHAIN_ID" ] || [ -z "$PRIVATE_KEY" ]; then
	echo "Error: Required environment variables not set"
	echo "CUSTOM_RPC: $CUSTOM_RPC"
	echo "CHAIN_ID: $CHAIN_ID"
	echo "PRIVATE_KEY: [hidden]"
	exit 1
fi

echo "Running foundry compatibility tests sequentially..."

# Function to extract transaction hash from forge script broadcast files
extract_tx_hash() {
	local script_name="$1"
	local broadcast_dir="$TEST_DIR/broadcast/${script_name}.s.sol"

	# Find the most recent run-latest.json file in any chain directory
	local json_file
	json_file=$(find "$broadcast_dir" -name "run-latest.json" -type f | head -1)

	if [ -f "$json_file" ]; then
		# Extract the first transaction hash from the JSON file
		local tx_hash
		tx_hash=$(jq -r '.transactions[0].hash // empty' "$json_file" 2>/dev/null)
		echo "$tx_hash"
	else
		echo ""
	fi
}

# Function to extract contract address from forge script broadcast files
extract_contract_address() {
	local script_name="$1"
	local broadcast_dir="$TEST_DIR/broadcast/${script_name}.s.sol"

	# Find the most recent run-latest.json file in any chain directory
	local json_file
	json_file=$(find "$broadcast_dir" -name "run-latest.json" -type f | head -1)

	if [ -f "$json_file" ]; then
		# Extract the contract address from the JSON file
		local contract_addr
		contract_addr=$(jq -r '.transactions[0].contractAddress // .transactions[0].additionalContracts[0].address // empty' "$json_file" 2>/dev/null)
		echo "$contract_addr"
	else
		echo ""
	fi
}

# Function to run corresponding shell script
run_shell_script() {
	local script_name="$1"
	local tx_hash="$2"
	local shell_script_dir="$TEST_DIR/shellscripts"
	local shell_script=""

	# Map forge script names to shell script names
	case "$script_name" in
	"DeployERC20")
		# No corresponding shell script for deployment
		return
		;;
	"NetworkInfo")
		shell_script="get-network-info.sh"
		;;
	"ReadState")
		shell_script="read_state.sh"
		;;
	"Transfer")
		shell_script="transfer.sh"
		;;
	"TransferError")
		shell_script="transfer_error.sh"
		;;
	*)
		echo "No corresponding shell script for $script_name"
		return
		;;
	esac

	if [ -f "$shell_script_dir/$shell_script" ]; then
		echo "Running shell script: $shell_script"

		# Export TX_HASH for scripts that need it
		export TX_HASH="$tx_hash"

		# Run the shell script
		pushd "$shell_script_dir" >/dev/null
		case "$script_name" in
		"NetworkInfo")
			bash "$shell_script" "$CUSTOM_RPC"
			;;
		"ReadState")
			# Need contract address - extract from deployment broadcast JSON
			local contract_addr
			contract_addr=$(extract_contract_address "DeployERC20")
			if [ -n "$contract_addr" ]; then
				bash "$shell_script" "$contract_addr"
			else
				echo "Warning: Could not find contract address for ReadState shell script"
			fi
			;;
		"Transfer")
			# Need contract address and recipient
			local contract_addr recipient_addr
			contract_addr=$(extract_contract_address "DeployERC20")
			recipient_addr="${ACCOUNT_2:-0x0000000000000000000000000000000000000002}"
			if [ -n "$contract_addr" ]; then
				bash "$shell_script" "$contract_addr" "$recipient_addr" "100000000000000000000"
			else
				echo "Warning: Could not find contract address for Transfer shell script"
			fi
			;;
		"TransferError")
			# For error case, ensure CONTRACT is set
			if [ -z "${CONTRACT:-}" ]; then
				# Try to get contract address from deployment broadcast JSON
				local contract_addr
				contract_addr=$(extract_contract_address "DeployERC20")
				if [ -n "$contract_addr" ]; then
					export CONTRACT="$contract_addr"
				fi
			fi
			bash "$shell_script"
			;;
		esac
		popd >/dev/null

		echo "Shell script $shell_script completed"
		echo "---"
	else
		echo "Shell script $shell_script not found in $shell_script_dir"
	fi
}

# Function to run forge script with proper error checking
run_forge_script() {
	local script_name="$1"
	local description="$2"
	local log_file="/tmp/${script_name}_deployment.log"

	echo "$description..."

	# Run forge and tee output to both stdout and log file
	# Temporarily disable exit-on-error for forge command since TransferError is expected to fail
	set +e
	if [ "$VERBOSE" = true ]; then
		forge script "script/${script_name}.s.sol:${script_name}" \
			--rpc-url "$CUSTOM_RPC" \
			--broadcast \
			--chain-id "$CHAIN_ID" 2>&1 | tee "$log_file"
		local exit_code=${PIPESTATUS[0]}
	else
		forge script "script/${script_name}.s.sol:${script_name}" \
			--rpc-url "$CUSTOM_RPC" \
			--broadcast \
			--chain-id "$CHAIN_ID" >"$log_file" 2>&1
		local exit_code=$?
	fi
	# Re-enable exit-on-error
	set -e

	# Give a moment for output to be fully written to log file
	echo "Waiting for output to be written to log file..."
	sleep 3

	# Special handling for TransferError - it's expected to fail with simulation error
	# Check this BEFORE checking exit codes since forge exits with non-zero for simulation failures
	if [ "$script_name" = "TransferError" ]; then
		if grep -q "Error: Simulated execution failed" "$log_file" && grep -q "Script ran successfully" "$log_file"; then
			echo "$description completed successfully! (Expected revert detected)"
		else
			echo "Error: $description should have failed with simulated execution error"
			echo "Last 20 lines of output:"
			tail -20 "$log_file"
			exit 1
		fi
	else
		# Normal success checking for other scripts
		if [ "$exit_code" -ne 0 ]; then
			echo "Error: $description failed with exit code $exit_code"
			echo "Last 20 lines of output:"
			tail -20 "$log_file"
			exit 1
		fi

		# Check for success based on script type
		if grep -q "ONCHAIN EXECUTION COMPLETE & SUCCESSFUL" "$log_file"; then
			echo "$description completed successfully!"
		elif grep -q "Script ran successfully" "$log_file"; then
			echo "$description completed successfully!"
		else
			echo "Error: $description did not complete successfully"
			echo "Last 20 lines of output:"
			tail -20 "$log_file"
			exit 1
		fi
	fi

	# Extract transaction hash and run corresponding shell script
	local tx_hash
	tx_hash=$(extract_tx_hash "$script_name")
	if [ -n "$tx_hash" ]; then
		echo "Extracted TX_HASH: $tx_hash"
		run_shell_script "$script_name" "$tx_hash"
	else
		echo "No transaction hash found in broadcast files"
		# Still run shell script without TX_HASH for scripts that don't need it
		run_shell_script "$script_name" ""
	fi

	# Small delay between tests
	sleep 3
}

# Test 1: Deploy ERC20 contract
run_forge_script "DeployERC20" "Deploying ERC20 contract"

# Test 2: Query Network Info
run_forge_script "NetworkInfo" "Querying network information"

# Test 3: Read State
run_forge_script "ReadState" "Reading contract state"

# Test 4: Transfer
run_forge_script "Transfer" "Executing ERC20 transfer"

# Test 5: Transfer Error (revert case)
run_forge_script "TransferError" "Testing transfer error"

echo "All foundry compatibility tests completed successfully!"
