#!/usr/bin/env bash
# Poll all node APP_HASHes every N seconds.
# Usage: ./poll.sh [interval_seconds] [testnet_dir]
#   interval_seconds: default 10
#   testnet_dir: default ./testnet (relative to systemtests dir)

INTERVAL=${1:-10}
TESTNET=${2:-"$(dirname "$0")/../testnet"}

echo "Polling APP_HASH every ${INTERVAL}s from ${TESTNET}/node*.out"
echo "Press Ctrl+C to stop."
echo ""

while true; do
    echo "=== $(date +%H:%M:%S) ==="
    for n in 0 1 2 3; do
        h=$(grep "APP_HASH" "${TESTNET}/node${n}.out" 2>/dev/null | tail -1)
        if [ -n "$h" ]; then
            echo "  node${n}: $h"
        else
            echo "  node${n}: (no output yet)"
        fi
    done
    if ! pgrep -f "evmd start" > /dev/null 2>&1; then
        echo "  ** NODES STOPPED **"
        # Show final result
        for n in 0 1 2 3; do
            grep "APP_HASH" "${TESTNET}/node${n}.out" 2>/dev/null | tail -1 | xargs -I{} echo "  FINAL node${n}: {}"
        done
        break
    fi
    sleep "$INTERVAL"
done
