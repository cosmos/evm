#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="$SCRIPT_DIR/.env"

export_envs() {
  local env_file="${1:-$ENV_FILE}"
  local envs_output=""
  
  if [[ ! -f "$env_file" ]]; then
    echo "ERROR: Environment file $env_file not found" >&2
    exit 1
  fi
  
  while IFS= read -r line || [[ -n "$line" ]]; do
    [[ -z "$line" || "$line" =~ ^[[:space:]]*# ]] && continue
    
    line="${line#export }"
    
    [[ "$line" != *"="* ]] && continue
    
    key="${line%%=*}"
    value="${line#*=}"
    value="${value#\"}"
    value="${value%\"}"
    value="${value#\'}"
    value="${value%\'}"
    
    if [[ -n "$envs_output" ]]; then
      envs_output="$envs_output,$key=$value"
    else
      envs_output="$key=$value"
    fi
    
  done < "$env_file"
  echo "$envs_output"
}

ENVS_STRING=$(export_envs "$ENV_FILE")
echo "$ENVS_STRING"
