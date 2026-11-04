#!/bin/bash
set -ex 

if [ ! -f ./misc/config/compose-config.yaml ]; then
  echo "ERROR: ./misc/config/compose-config.yaml is missing." >&2
  echo "       Run ./misc/bin/gen-compose-config.sh (or: task compose:config)." >&2
  exit 1
fi

rm -f ./metahub
go build -o metahub
exec ./metahub serve --config ./misc/config/compose-config.yaml --user-db ./misc/sqlite/metahub_20251209.db