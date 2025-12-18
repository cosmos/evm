#!/usr/bin/env bash
set -euo pipefail

ENDPOINT=${1:-http://127.0.0.1:8545}
BLOCKS=${2:-0x40}
ROUNDS=${3:-8}
PCTS=${4:-[25,50,75]}

BODY='{"jsonrpc":"2.0","id":1,"method":"eth_feeHistory","params":["'"$BLOCKS"'","latest",'"$PCTS"']}'

sum=0; min=999999; max=0
echo "eth_feeHistory $BLOCKS, percentiles=$PCTS, rounds=$ROUNDS"
for i in $(seq 1 "$ROUNDS"); do
  t=$(curl -s -o /dev/null -w '%{time_total}\n' -H 'Content-Type: application/json' -d "$BODY" "$ENDPOINT")
  t_ms=$(awk -v t="$t" 'BEGIN { printf("%.0f", t*1000) }')
  echo "Run $i: ${t_ms} ms"
  sum=$((sum + t_ms))
  (( t_ms < min )) && min=$t_ms
  (( t_ms > max )) && max=$t_ms
  sleep 0.15
done
avg=$((sum / ROUNDS))
printf "Avg: %d ms   Min: %d ms   Max: %d ms\n" "$avg" "$min" "$max"


