#!/usr/bin/env sh
set -x

BINARY=/epixd/${BINARY:-epixd}
ID=${ID:-0}
LOG=${LOG:-epixd.log}

if ! [ -f "${BINARY}" ]; then
	echo "The binary $(basename "${BINARY}") cannot be found. Please add the binary to the shared folder. Please use the BINARY environment variable if the name of the binary is not 'epixd'"
	exit 1
fi

export EPIXDHOME="/data/node${ID}/epixd"

if [ -d "$(dirname "${EPIXDHOME}"/"${LOG}")" ]; then
  "${BINARY}" --home "${EPIXDHOME}" "$@" | tee "${EPIXDHOME}/${LOG}"
else
  "${BINARY}" --home "${EPIXDHOME}" "$@"
fi
