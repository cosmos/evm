#!/bin/bash

# Container-friendly evmd initialization and startup script
# This runs inside the evmd container, so no Docker commands

set -e

echo "🔧 Starting evmd container initialization..."

# Set up variables (same as start-evmd.sh)
KEYRING="test"
KEYALGO="eth_secp256k1"
CHAINDIR="/data"
GENESIS="$CHAINDIR/config/genesis.json"
TMP_GENESIS="$CHAINDIR/config/tmp_genesis.json"
CHAIN_ID="local-4221"
BASEFEE=10000000

# Standard test keys (same as start-evmd.sh)
VAL_KEY="mykey"
VAL_MNEMONIC="gesture inject test cycle original hollow east ridge hen combine junk child bacon zero hope comfort vacuum milk pitch cage oppose unhappy lunar seat"

USER1_KEY="dev0"
USER1_MNEMONIC="copper push brief egg scan entry inform record adjust fossil boss egg comic alien upon aspect dry avoid interest fury window hint race symptom"

USER2_KEY="dev1"
USER2_MNEMONIC="maximum display century economy unlock van census kite error heart snow filter midnight usage egg venture cash kick motor survey drastic edge muffin visual"

USER3_KEY="dev2"
USER3_MNEMONIC="will wear settle write dance topic tape sea glory hotel oppose rebel client problem era video gossip glide during yard balance cancel file rose"

USER4_KEY="dev3"
USER4_MNEMONIC="doll midnight silk carpet brush boring pluck office gown inquiry duck chief aim exit gain never tennis crime fragile ship cloud surface exotic patch"

# Initialize chain directly (no Docker wrapper)
echo "🔧 Initializing chain..."
echo "$VAL_MNEMONIC" | evmd init localtestnet -o --chain-id "$CHAIN_ID" --recover --home "$CHAINDIR"

# Set client config
evmd config set client chain-id "$CHAIN_ID" --home "$CHAINDIR"
evmd config set client keyring-backend "$KEYRING" --home "$CHAINDIR"

# Add keys
echo "🔧 Adding standard test keys..."
echo "$VAL_MNEMONIC" | evmd keys add "$VAL_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" --home "$CHAINDIR"
echo "$USER1_MNEMONIC" | evmd keys add "$USER1_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" --home "$CHAINDIR"
echo "$USER2_MNEMONIC" | evmd keys add "$USER2_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" --home "$CHAINDIR"
echo "$USER3_MNEMONIC" | evmd keys add "$USER3_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" --home "$CHAINDIR"
echo "$USER4_MNEMONIC" | evmd keys add "$USER4_KEY" --recover --keyring-backend "$KEYRING" --algo "$KEYALGO" --home "$CHAINDIR"

# Configure genesis file
echo "🔧 Configuring genesis file..."
jq '.app_state["staking"]["params"]["bond_denom"]="atest"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
jq '.app_state["gov"]["deposit_params"]["min_deposit"][0]["denom"]="atest"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
jq '.app_state["gov"]["params"]["min_deposit"][0]["denom"]="atest"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
jq '.app_state["gov"]["params"]["expedited_min_deposit"][0]["denom"]="atest"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
jq '.app_state["bank"]["denom_metadata"]=[{"description":"The native staking token for evmd.","denom_units":[{"denom":"atest","exponent":0,"aliases":["attotest"]},{"denom":"test","exponent":18,"aliases":[]}],"base":"atest","display":"test","name":"Test Token","symbol":"TEST","uri":"","uri_hash":""}]' "$GENESIS" >"$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
jq '.app_state["evm"]["params"]["evm_denom"]="atest"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
jq '.app_state["mint"]["params"]["mint_denom"]="atest"' "$GENESIS" > "$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"

# Add genesis accounts
echo "🔧 Setting up genesis accounts..."
evmd genesis add-genesis-account "$VAL_KEY" 100000000000000000000000000atest --keyring-backend "$KEYRING" --home "$CHAINDIR"
evmd genesis add-genesis-account "$USER1_KEY" 1000000000000000000000atest --keyring-backend "$KEYRING" --home "$CHAINDIR"
evmd genesis add-genesis-account "$USER2_KEY" 1000000000000000000000atest --keyring-backend "$KEYRING" --home "$CHAINDIR"
evmd genesis add-genesis-account "$USER3_KEY" 1000000000000000000000atest --keyring-backend "$KEYRING" --home "$CHAINDIR"
evmd genesis add-genesis-account "$USER4_KEY" 1000000000000000000000atest --keyring-backend "$KEYRING" --home "$CHAINDIR"

# Generate validator transaction
evmd genesis gentx "$VAL_KEY" 1000000000000000000000atest --gas-prices "${BASEFEE}atest" --keyring-backend "$KEYRING" --chain-id "$CHAIN_ID" --home "$CHAINDIR"
evmd genesis collect-gentxs --home "$CHAINDIR"
evmd genesis validate-genesis --home "$CHAINDIR"

# Reduce block time by adjusting consensus timeouts
CONFIG_TOML="$CHAINDIR/config/config.toml"
sed -i 's/timeout_commit = "5s"/timeout_commit = "500ms"/g' "$CONFIG_TOML"
sed -i 's/timeout_propose = "3s"/timeout_propose = "1s"/g' "$CONFIG_TOML"
sed -i 's/timeout_propose_delta = "500ms"/timeout_propose_delta = "100ms"/g' "$CONFIG_TOML"
sed -i 's/timeout_prevote = "1s"/timeout_prevote = "300ms"/g' "$CONFIG_TOML"
sed -i 's/timeout_prevote_delta = "500ms"/timeout_prevote_delta = "100ms"/g' "$CONFIG_TOML"
sed -i 's/timeout_precommit = "1s"/timeout_precommit = "300ms"/g' "$CONFIG_TOML"
sed -i 's/timeout_precommit_delta = "500ms"/timeout_precommit_delta = "100ms"/g' "$CONFIG_TOML"

echo "🚀 Starting evmd..."
exec evmd start \
    --home "$CHAINDIR" \
    --minimum-gas-prices=0.0001atest \
    --json-rpc.enable \
    --json-rpc.api eth,txpool,personal,net,debug,web3 \
    --json-rpc.address 0.0.0.0:8545 \
    --json-rpc.ws-address 0.0.0.0:8546 \
    --json-rpc.enable-profiling \
    --keyring-backend test \
    --chain-id "$CHAIN_ID"
