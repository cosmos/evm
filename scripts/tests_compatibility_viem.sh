#!/usr/bin/env bash

# CI script for running viem compatibility tests
# This script sets up dependencies, launches the node, and runs viem tests with mocha
# Usage: ./tests_compatibility_viem.sh [--verbose] [--node-log-print]

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
TEST_DIR="$ROOT/tests/evm-tools-compatibility/viem"

echo "Setting up viem compatibility tests..."

# Setup dependencies
setup_compatibility_tests "$NODE_LOG_PRINT"

start_node "$NODE_LOG_PRINT"
trap cleanup_node EXIT
sleep 3

# Wait for the node to be ready
echo "Waiting for evmd node to be ready..."

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

# Install npm dependencies if not already installed
if [ ! -d "node_modules" ]; then
	echo "Installing npm dependencies..."
	npm install
else
	echo "npm dependencies already installed, skipping..."
fi

echo "Running viem compatibility tests..."

# Run tests with npm test
if [ "$VERBOSE" = true ]; then
	echo "Running: npm test"
	npm test
else
	echo "Running: npm test"
	npm test 2>&1 | tee /tmp/viem-test.log
fi

# Check if tests passed
if [ "${PIPESTATUS[0]}" -eq 0 ]; then
	echo "All viem compatibility tests passed successfully!"
else
	echo "Error: Some viem tests failed"
	if [ "$VERBOSE" = false ]; then
		echo "Test output:"
		tail -20 /tmp/viem-test.log
	fi
	exit 1
fi
