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

# 1) Send via cast and require a failing exit status
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

# 2) Try parse JSON
if [[ "$OUTPUT" == *"{"* ]]; then
	printf '%s\n' "$OUTPUT" | jq -er '
    if .success == false and (.errors | length > 0) and
      all(.errors[]; (.message | contains("execution reverted")) and
        (.message | contains("ERC20InsufficientBalance("))) then
      "✅ Transaction reverted as expected:\n  " + (.errors | map(.message) | join("\n  "))
    else
      error("Unexpected cast response: " + tojson)
    end'
else
	# 3b) No JSON -> fallback to raw text check
	if echo "$OUTPUT" | grep -q 'ERC20InsufficientBalance('; then
		echo "✅ Transaction reverted as expected"
		echo
		echo "Revert detail:"
		# Newline before 'Error:'
		echo "$OUTPUT" | sed -n 's/.*\(Error:.*\)/\1/p'
	else
		echo "❌ Unexpected response (no JSON, no revert message):"
		echo
		echo "$OUTPUT"
		exit 1
	fi
fi
