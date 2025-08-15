#!/usr/bin/env bash

# Installs dependencies and sets up git submodules for compatibility tests.
# Does NOT launch the node or run tests - that should be handled separately.

set -eo pipefail

ROOT="$(git rev-parse --show-toplevel)"
COMPAT_DIR="${COMPAT_DIR:-$ROOT/tests/evm-tools-compatibility}"

# -----------------------------------------------------------------------------
# Tooling installation
# -----------------------------------------------------------------------------

# Install Foundry if forge is not present
if ! command -v forge >/dev/null 2>&1; then
	curl -L https://foundry.paradigm.xyz | bash
	# shellcheck source=/dev/null
	source "$HOME/.bashrc" >/dev/null 2>&1 || true
	foundryup
fi

# Install Node.js and npm if missing
if ! command -v npm >/dev/null 2>&1; then
	curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
	sudo apt-get install -y nodejs
fi

# -----------------------------------------------------------------------------
# Install dependencies for the individual test suites
# -----------------------------------------------------------------------------

# Foundry based projects
for dir in "foundry" "foundry-uniswap-v3"; do
	if [ -d "$COMPAT_DIR/$dir" ]; then
		pushd "$COMPAT_DIR/$dir" >/dev/null

		# Only run forge install if lib directory is empty or doesn't exist
		if [ ! -d "lib" ] || [ -z "$(ls -A lib 2>/dev/null)" ]; then
			echo "Installing foundry dependencies for $dir..."
			forge install
		else
			echo "Foundry dependencies already installed for $dir, skipping..."
		fi

		popd >/dev/null
	fi
done

# Hardhat project
if [ -d "$COMPAT_DIR/hardhat" ]; then
	for subproject in "v3-core" "v3-periphery"; do
		if [ -d "$COMPAT_DIR/hardhat/external/$subproject" ]; then
			pushd "$COMPAT_DIR/hardhat/external/$subproject" >/dev/null

			# Only init submodules if not already initialized
			if [ ! -f ".git" ] && [ ! -d ".git" ]; then
				echo "Initializing git submodules for hardhat/$subproject..."
				git submodule init
				git submodule update
			else
				echo "Git submodules already initialized for hardhat/$subproject, updating..."
				git submodule update
			fi

			# Only install npm dependencies if node_modules doesn't exist
			if [ ! -d "node_modules" ]; then
				echo "Installing npm dependencies for hardhat/$subproject..."
				npm install
			else
				echo "npm dependencies already installed for hardhat/$subproject, skipping..."
			fi

			# Only compile if build artifacts don't exist
			if [ ! -d "artifacts" ] && [ ! -d "cache" ]; then
				echo "Compiling hardhat contracts for $subproject..."
				npx hardhat compile
			else
				echo "Hardhat contracts already compiled for $subproject, skipping..."
			fi

			popd >/dev/null
		fi
	done
fi

# Node based projects (viem, web3.js, sdk examples)
for dir in "$COMPAT_DIR"/sdk/* "$COMPAT_DIR"/viem "$COMPAT_DIR"/web3js; do
	if [ -d "$dir" ] && [ -f "$dir/package.json" ]; then
		pushd "$dir" >/dev/null

		# Only install npm dependencies if node_modules doesn't exist
		if [ ! -d "node_modules" ]; then
			echo "Installing npm dependencies for $(basename "$dir")..."
			npm install
		else
			echo "npm dependencies already installed for $(basename "$dir"), skipping..."
		fi

		popd >/dev/null
	fi
done

echo "Dependencies and git submodules setup completed!"
