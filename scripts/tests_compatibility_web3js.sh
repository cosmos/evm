#!/usr/bin/env bash

# CI script for running web3.js compatibility tests
# This script sets up dependencies, launches the node, and runs web3.js tests with mocha
# Usage: ./tests_compatibility_web3js.sh [--verbose] [--node-log-print]

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
TEST_DIR="$ROOT/tests/evm-tools-compatibility/web3.js"

echo "Setting up web3.js compatibility tests..."

# Setup dependencies
setup_compatibility_tests "$NODE_LOG_PRINT"

start_node "$NODE_LOG_PRINT"
trap cleanup_node EXIT
sleep 3

wait_for_node 10

# Change to the test directory
cd "$TEST_DIR"

# Install npm dependencies if not already installed
if [ ! -d "node_modules" ]; then
	echo "Installing npm dependencies..."
	npm install
else
	echo "npm dependencies already installed, skipping..."
fi

echo "Running web3.js compatibility tests..."

# Run tests with npm test
if [ "$VERBOSE" = true ]; then
	echo "Running: npm test"
	npm test
else
	echo "Running: npm test"
	npm test 2>&1 | tee /tmp/web3js-test.log
fi

# Check if tests passed
if [ "${PIPESTATUS[0]}" -eq 0 ]; then
	echo "All web3.js compatibility tests passed successfully!"
else
	echo "Error: Some web3.js tests failed"
	if [ "$VERBOSE" = false ]; then
		echo "Test output:"
		tail -20 /tmp/web3js-test.log
	fi
	exit 1
fi
