#!/usr/bin/env bash
# Show latest APP_HASH from each node every N seconds (default 10).
# Usage: bash apphash/check.sh [interval]

INTERVAL=${1:-10}
DIR="$(cd "$(dirname "$0")/.." && pwd)/testnet"

while true; do
  echo "=== $(date +%H:%M:%S) ==="
  for i in 0 1 2 3; do
    line=$(grep "APP_HASH" "$DIR/node${i}.out" 2>/dev/null | tail -1)
    if [ -z "$line" ]; then
      echo "  node${i}: (no output yet)"
    else
      echo "  node${i}: $line"
    fi
  done

  # Check if all nodes stopped
  if ! pgrep -f evmd >/dev/null 2>&1; then
    echo "  ** NODES STOPPED **"
    break
  fi

  sleep "$INTERVAL"
done
