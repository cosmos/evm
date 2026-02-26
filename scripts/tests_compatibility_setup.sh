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

# Install Node.js and pnpm if missing
if ! command -v node >/dev/null 2>&1; then
	curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
	sudo apt-get install -y nodejs
fi
if ! command -v pnpm >/dev/null 2>&1; then
	corepack enable 2>/dev/null || true
	npm install -g corepack 2>/dev/null || true
	corepack enable && corepack prepare pnpm@latest --activate
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

			# Install dependencies (ensures pnpm layout for pnpm exec)
			echo "Installing dependencies for hardhat/$subproject..."
			pnpm install

			# Only compile if build artifacts don't exist
			if [ ! -d "artifacts" ] && [ ! -d "cache" ]; then
				echo "Compiling hardhat contracts for $subproject..."
				pnpm exec hardhat compile
			else
				echo "Hardhat contracts already compiled for $subproject, skipping..."
			fi

			popd >/dev/null
		fi
	done
fi

# Node based projects (viem, web3.js, sdk examples)
for dir in "$COMPAT_DIR"/sdk/* "$COMPAT_DIR"/viem "$COMPAT_DIR"/web3.js; do
	if [ -d "$dir" ] && [ -f "$dir/package.json" ]; then
		pushd "$dir" >/dev/null

		# Install dependencies (ensures pnpm layout for pnpm exec)
		echo "Installing dependencies for $(basename "$dir")..."
		pnpm install

		popd >/dev/null
	fi
done

echo "Dependencies and git submodules setup completed!"
