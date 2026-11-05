#!/usr/bin/env bash
# Fail unless every required job succeeded.
#
#   check-required-jobs.sh build=success test=skipped ...
#
# A job that is legitimately event-conditional (it never runs on some events)
# is passed in ALLOW_SKIPPED as a comma-separated list. Any other non-success
# result fails, `skipped` included.
set -euo pipefail

allow="${ALLOW_SKIPPED:-}"

if [ "$#" -eq 0 ]; then
  echo "usage: $0 name=result [name=result ...]" >&2
  exit 2
fi

failed=0
for pair in "$@"; do
  name="${pair%%=*}"
  result="${pair#*=}"
  if [ "$result" = "success" ]; then
    continue
  fi
  if [ "$result" = "skipped" ] && printf ',%s,' "$allow" | grep -q ",${name},"; then
    continue
  fi
  echo "::error::required job '${name}' did not succeed (result: ${result})"
  failed=1
done

exit "$failed"
