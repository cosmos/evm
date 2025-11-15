#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
echo $SCRIPT_DIR

cd "$SCRIPT_DIR/../integration_tests"

restore_env_file() {
  local env_file="$SCRIPT_DIR/.env"
  local env_template="$SCRIPT_DIR/env.template"
  
  if [[ -n "$ENVS" ]]; then
    echo "Restoring .env from ENVS secret"
    IFS=',' read -ra ADDR <<< "$ENVS"
    for kv in "${ADDR[@]}"; do
      key="${kv%%=*}"
      val="${kv#*=}"
      echo "export $key=\"$val\""
    done > "$env_file"
    echo "Environment file restored from ENVS secret"
  elif [[ ! -f "$env_file" ]]; then
    if [[ -f "$env_template" ]]; then
      echo "No ENVS secret found and no .env file exists, copying from template"
      cp "$env_template" "$env_file"
    else
      echo "ERROR: Neither ENVS secret nor env.template found" >&2
      exit 1
    fi
  else
    echo "Using existing .env file"
  fi
}

restore_env_file
