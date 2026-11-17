#!/usr/bin/env bash
set -euo pipefail

# Usage: ./error_edge_case.sh
# Requires in .env: CUSTOM_RPC, PRIVATE_KEY, ACCOUNT_2 (recipient), CONTRACT
# shellcheck source=../.env
# shellcheck disable=SC1091
source ../.env
export FOUNDRY_DISABLE_NIGHTLY_WARNING=1

RPC_URL=${CUSTOM_RPC:-http://127.0.0.1:8545}
PK=${PRIVATE_KEY:?}
RECIPIENT=${ACCOUNT_2:?}
CHAIN_ID=${CHAIN_ID:-262144}

# Ensure CONTRACT is set
if [ -z "${CONTRACT:-}" ]; then
	echo "Error: CONTRACT environment variable not set."
	exit 1
fi

# Amount exceeding typical balance (2000 tokens)
AMOUNT=2000000000000000000000

echo "🔄 Sending transfer exceeding balance..."

# ERC20InsufficientBalance(address,uint256,uint256) selector
INSUFFICIENT_BALANCE_SELECTOR=0xe450d38c

# 1) Send via cast; it must exit non-zero
echo "❌ Attempting transfer that should fail:"
echo "$ cast send \"$CONTRACT\" 'transfer(address,uint256)' \"$RECIPIENT\" \"$AMOUNT\" --rpc-url \"$RPC_URL\" --private-key \"[HIDDEN]\" --chain-id \"$CHAIN_ID\" --json"
if OUTPUT=$(cast send \
	"$CONTRACT" \
	'transfer(address,uint256)' "$RECIPIENT" "$AMOUNT" \
	--rpc-url "$RPC_URL" \
	--private-key "$PK" \
	--chain-id "$CHAIN_ID" \
	--json 2>&1); then
	echo "❌ Expected a revert, but transaction succeeded:"
	printf '%s\n' "$OUTPUT"
	exit 1
fi

# 2) Match the revert text, independent of cast's error output format
if printf '%s' "$OUTPUT" | grep -q 'execution reverted' &&
	printf '%s' "$OUTPUT" | grep -qi -e 'ERC20InsufficientBalance' -e "$INSUFFICIENT_BALANCE_SELECTOR"; then
	echo "✅ Transaction reverted as expected"
	echo
	echo "Revert detail:"
	printf '%s\n' "$OUTPUT"
else
	echo "❌ Unexpected response (no ERC20InsufficientBalance revert):"
	echo
	printf '%s\n' "$OUTPUT"
	exit 1
fi
