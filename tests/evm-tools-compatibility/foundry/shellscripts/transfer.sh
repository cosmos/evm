#!/usr/bin/env bash
set -euo pipefail

# Usage: ./transfer.sh <CONTRACT_ADDR> <TO_ADDR> <AMOUNT>
# Requires: .env with CUSTOM_RPC and PRIVATE_KEY

if [ $# -ne 3 ]; then
	echo "Usage: $0 <CONTRACT_ADDR> <TO_ADDR> <AMOUNT>"
	exit 1
fi

# shellcheck source=../.env
# shellcheck disable=SC1091
source ../.env
CONTRACT=$1
TO=$2
AMOUNT=$3 # e.g. 100000000000000000000 for 100 tokens (18 decimals)
RPC_URL=${CUSTOM_RPC:-http://127.0.0.1:8545}
PRIVATE_KEY=${PRIVATE_KEY:?}
CHAIN_ID=${CHAIN_ID:-262144}

echo "🔗 RPC URL: $RPC_URL"
echo "📦 Sending transfer($TO, $AMOUNT) to $CONTRACT"

# 1) Send transaction with cast
echo "💸 Sending transfer transaction:"
echo "$ cast send \"$CONTRACT\" 'transfer(address,uint256)' \"$TO\" \"$AMOUNT\" --rpc-url \"$RPC_URL\" --private-key \"[HIDDEN]\" --chain-id \"$CHAIN_ID\" --json"
RESPONSE=$(cast send \
	"$CONTRACT" \
	'transfer(address,uint256)' "$TO" "$AMOUNT" \
	--rpc-url "$RPC_URL" \
	--private-key "$PRIVATE_KEY" \
	--chain-id "$CHAIN_ID" \
	--json)

TXHASH=$(echo "$RESPONSE" | jq -r '.hash // .transactionHash')
echo "✅ Transaction sent: $TXHASH"
echo

# 2) Wait for the tx to be mined
echo "⏳ Waiting for 2 seconds..."
sleep 2
echo

# 3) Derive sender address
echo "🔑 Getting sender address:"
echo "$ cast wallet address --private-key \"[HIDDEN]\""
SENDER=$(cast wallet address --private-key "$PRIVATE_KEY")
echo "Sender: $SENDER"
echo

# 4) Verify balances
echo "💰 Checking balances:"
echo "$ cast call --rpc-url \"$RPC_URL\" \"$CONTRACT\" 'balanceOf(address)(uint256)' \"$SENDER\""
SENDER_BALANCE=$(cast call \
	--rpc-url "$RPC_URL" \
	"$CONTRACT" \
	'balanceOf(address)(uint256)' \
	"$SENDER")

echo "$ cast call --rpc-url \"$RPC_URL\" \"$CONTRACT\" 'balanceOf(address)(uint256)' \"$TO\""
RECEIVER_BALANCE=$(cast call \
	--rpc-url "$RPC_URL" \
	"$CONTRACT" \
	'balanceOf(address)(uint256)' \
	"$TO")

echo "👤 Sender ($SENDER) balance:   $SENDER_BALANCE"
echo "👤 Receiver ($TO) balance: $RECEIVER_BALANCE"
