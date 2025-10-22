#!/bin/bash
set -e

# explicitly set a short TMPDIR to prevent path too long issue on macosx
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
export TMPDIR=/tmp

cd "$SCRIPT_DIR/../integration_tests"

TESTS_TO_RUN="${TESTS_TO_RUN:-all}"
CHAIN_CONFIG="${CHAIN_CONFIG:-}"

# pytest command with chain-config
build_pytest_cmd() {
  local base_cmd="$1"
  if [[ -n "$CHAIN_CONFIG" ]]; then
    echo "$base_cmd --chain-config $CHAIN_CONFIG"
  else
    echo "$base_cmd"
  fi
}

load_env_file() {
  local env_file="$1"
  if [ -f "$env_file" ]; then
    echo "Loading environment variables from $(basename "$env_file")"
    set -a
    source "$env_file"
    set +a
  else
    echo "ERROR: $(basename "$env_file") not found. Please create it from $(basename "$env_file").template." >&2
    exit 1
  fi
}

load_env_file "$SCRIPT_DIR/.env"

if [[ "$TESTS_TO_RUN" == "all" ]]; then
  echo "run all local tests"
  cmd=$(build_pytest_cmd "pytest -s -vvv -m \"not connect\"")
elif [[ "$TESTS_TO_RUN" == "connect" ]]; then
  echo "run tests matching $TESTS_TO_RUN"
  cmd=$(build_pytest_cmd "pytest -vv -s -m connect")
else
  echo "run tests matching $TESTS_TO_RUN"
  cmd=$(build_pytest_cmd "pytest -vv -s -m \"$TESTS_TO_RUN\"")
fi

eval $cmd