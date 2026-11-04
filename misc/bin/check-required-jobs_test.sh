#!/usr/bin/env bash
# Fixture for check-required-jobs.sh: proves the aggregator cannot pass when a
# required job was skipped. Run from the repository root or from misc/bin/.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
check="${here}/check-required-jobs.sh"
cases=0

expect() {
  local want="$1"; shift
  cases=$((cases + 1))
  local got=0
  "$check" "$@" >/dev/null 2>&1 || got=1
  if [ "$got" -ne "$want" ]; then
    echo "FAIL: expected exit ${want}, got ${got}: $*" >&2
    exit 1
  fi
}

expect 0 build=success vet=success
expect 1 build=skipped vet=success
expect 1 build=failure vet=success
expect 1 build=cancelled
expect 1 build=success dco=skipped   # no allowlist entry -> must fail

# With the event-conditional allowance, skipped dco is accepted.
ALLOW_SKIPPED=dco "$check" build=success dco=skipped >/dev/null 2>&1 \
  || { echo "FAIL: ALLOW_SKIPPED=dco should accept skipped dco" >&2; exit 1; }
cases=$((cases + 1))

# ...but a skipped build is still fatal even with the allowance.
if ALLOW_SKIPPED=dco "$check" build=skipped dco=skipped >/dev/null 2>&1; then
  echo "FAIL: skipped build passed with ALLOW_SKIPPED=dco" >&2
  exit 1
fi
cases=$((cases + 1))

echo "ok: ${cases} cases"
