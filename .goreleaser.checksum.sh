#!/bin/bash

set -e

if [ -z "$1" ]; then
  echo "usage: $0 <path>"
  exit 1
fi

SUM=$(shasum -a 256 "$1" | cut -d' ' -f1)
BASENAME=$(basename "$1")
echo -n "${SUM}  ${BASENAME}" > "$1".sha256