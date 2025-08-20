#!/usr/bin/env bash

# Common helper functions for CI compatibility scripts

set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"

# Start evmd node in background
start_node() {
	local print_log="${1:-false}"
	pushd "$ROOT" >/dev/null
	if [ "$print_log" = true ]; then
		./local_node.sh -y &
	else
		./local_node.sh -y >/tmp/evmd.log 2>&1 &
	fi
	NODE_PID=$!
	popd >/dev/null
}

# Wait until the node reaches the given block number
wait_for_node() {
	local target="${1:-10}"
	local rpc="http://127.0.0.1:8545"
	local timeout=60
	local elapsed=0
	echo "Waiting for evmd node to be ready..."
	while [ $elapsed -lt $timeout ]; do
		RESPONSE=$(curl -s -X POST -H "Content-Type: application/json" \
			--data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
			"$rpc" 2>/dev/null || true)
		if [ -n "$RESPONSE" ]; then
			BLOCK_HEX=$(echo "$RESPONSE" | grep -o '"result":"[^"]*"' | cut -d'"' -f4 || true)
			if [ -n "$BLOCK_HEX" ] && [ "$BLOCK_HEX" != "null" ]; then
				if BLOCK_NUMBER=$((16#${BLOCK_HEX#0x})); then
					echo "Current block number: $BLOCK_NUMBER (waiting for >= $target)"
					if [ "$BLOCK_NUMBER" -ge "$target" ]; then
						echo "Node is ready! Block number: $BLOCK_NUMBER"
						break
					fi
				fi
			fi
		fi
		echo "Waiting for node... ($elapsed/$timeout seconds)"
		sleep 2
		elapsed=$((elapsed + 2))
	done
	if [ $elapsed -ge $timeout ]; then
		echo "Error: Node failed to reach block $target within $timeout seconds"
		echo "Last response: $RESPONSE"
		echo "Checking node logs:"
		tail -20 /tmp/evmd.log 2>/dev/null || echo "No evmd logs found"
		exit 1
	fi
}

# Stop the node
cleanup_node() {
	if [ -n "${NODE_PID:-}" ]; then
		echo "Stopping evmd node..."
		kill "$NODE_PID" 2>/dev/null || true
		wait "$NODE_PID" 2>/dev/null || true
	fi
}

# Run the dependency setup script
setup_compatibility_tests() {
	local print_log="${1:-false}"
	echo "Running tests_compatibility_setup.sh..."
	if [ "$print_log" = true ]; then
		"$ROOT/scripts/tests_compatibility_setup.sh"
	else
		"$ROOT/scripts/tests_compatibility_setup.sh" >/tmp/tests_compatibility_setup.log 2>&1
	fi
}
