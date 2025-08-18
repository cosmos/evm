#!/usr/bin/env bash

# CI script for running hardhat compatibility tests
# This script sets up dependencies, launches the node, and runs hardhat tests
# Usage: ./tests_compatibility_hardhat.sh [--verbose] [--node-log-print]

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
TEST_DIR="$ROOT/tests/evm-tools-compatibility/hardhat"

echo "Setting up hardhat compatibility tests..."

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

# Install npm dependencies if not already installed
if [ ! -d "node_modules" ]; then
	echo "Installing npm dependencies..."
	npm install
else
	echo "npm dependencies already installed, skipping..."
fi

echo "Running hardhat compatibility tests..."

# Run tests with npx hardhat test (default network)
if [ "$VERBOSE" = true ]; then
	echo "Running: npx hardhat test"
	npx hardhat test 2>&1 | tee /tmp/hardhat-test.log
else
	echo "Running: npx hardhat test"
	npx hardhat test 2>&1 | tee /tmp/hardhat-test.log
fi

# Check if tests passed and no failures occurred
if [ "${PIPESTATUS[0]}" -eq 0 ] && ! grep -i "failing" /tmp/hardhat-test.log >/dev/null; then
	echo "All hardhat compatibility tests (default network) passed successfully!"
else
	echo "Error: Some hardhat tests (default network) failed"
	echo "Test output:"
	tail -20 /tmp/hardhat-test.log
	if grep -i "failing" /tmp/hardhat-test.log >/dev/null; then
		echo "Found 'failing' keyword in test output"
	fi
	exit 1
fi

echo "Running hardhat compatibility tests with localhost network..."

# Run tests with npx hardhat test --network localhost
if [ "$VERBOSE" = true ]; then
	echo "Running: npx hardhat test --network localhost"
	npx hardhat test --network localhost 2>&1 | tee /tmp/hardhat-test-localhost.log
else
	echo "Running: npx hardhat test --network localhost"
	npx hardhat test --network localhost 2>&1 | tee /tmp/hardhat-test-localhost.log
fi

# Check if tests passed and no failures occurred
if [ "${PIPESTATUS[0]}" -eq 0 ] && ! grep -i "failing" /tmp/hardhat-test-localhost.log >/dev/null; then
	echo "All hardhat compatibility tests (localhost network) passed successfully!"
else
	echo "Error: Some hardhat tests (localhost network) failed"
	echo "Test output:"
	tail -20 /tmp/hardhat-test-localhost.log
	if grep -i "failing" /tmp/hardhat-test-localhost.log >/dev/null; then
		echo "Found 'failing' keyword in test output"
	fi
	exit 1
fi
