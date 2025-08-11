#!/usr/bin/env sh
set -x

BINARY=/evmd/${BINARY:-evmd}
ID=${ID:-0}
LOG=${LOG:-evmd.log}

if ! [ -f "${BINARY}" ]; then
	echo "The binary $(basename "${BINARY}") cannot be found. Please add the binary to the shared folder. Please use the BINARY environment variable if the name of the binary is not 'evmd'"
	exit 1
fi

export EVMDHOME="/data/node${ID}/evmd"

if [ -d "$(dirname "${EVMDHOME}"/"${LOG}")" ]; then
  "${BINARY}" --home "${EVMDHOME}" "$@" | tee "${EVMDHOME}/${LOG}"
else
  "${BINARY}" --home "${EVMDHOME}" "$@"
fi
