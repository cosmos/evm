#!/bin/bash

export GOPATH="$HOME"/go
export PATH="$PATH":"$GOPATH"/bin

# remove existing data
rm -rf "$HOME"/.tmp-evmd-solidity-tests

# used to exit on first error (any non-zero exit code)
set -e

# build evmd binary
make install

cd tests/solidity || exit

# Function to install yarn dependencies with retry logic
install_yarn_deps() {
	local max_attempts=3
	local attempt=1
	local delay=5

	while [ $attempt -le $max_attempts ]; do
		echo "Attempting yarn install (attempt $attempt of $max_attempts)..."
		if yarn install; then
			echo "yarn install succeeded"
			return 0
		else
			echo "yarn install failed (attempt $attempt of $max_attempts)"
			if [ $attempt -lt $max_attempts ]; then
				echo "Retrying in $delay seconds..."
				sleep $delay
				delay=$((delay * 2))  # exponential backoff
			fi
			attempt=$((attempt + 1))
		fi
	done

	echo "yarn install failed after $max_attempts attempts"
	return 1
}

if command -v yarn &>/dev/null; then
	install_yarn_deps
else
	curl -sS https://dl.yarnpkg.com/debian/pubkey.gpg | sudo apt-key add -
	echo "deb https://dl.yarnpkg.com/debian/ stable main" | sudo tee /etc/apt/sources.list.d/yarn.list
	sudo apt update && sudo apt install yarn
	install_yarn_deps
fi

yarn test --network epix "$@"
