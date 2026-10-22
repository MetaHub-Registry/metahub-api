#!/usr/bin/env bash
# Print the Go third-party notices in Markdown. CI regenerates this file and
# fails when the committed copy differs.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "${ROOT}"

cat <<'HEADER'
# Third-party notices

Generated from the module graph. Do not edit by hand: run
`.github/scripts/third-party-notices.sh > THIRD_PARTY_NOTICES.md`.

`github.com/multiformats/go-base36` is omitted: its LICENSE.md is a
"Permissive License Stack" (MIT OR Apache-2.0) that the classifier cannot
parse, so the licence check ignores it explicitly.

| Module | Licence | Source |
|---|---|---|
HEADER

go run github.com/google/go-licenses/v2@v2.0.1 report ./... \
  --ignore github.com/multiformats/go-base36 2>/dev/null |
  awk -F, -v root="$(go list -m)" '
    $1 == root { next }
    { printf "| `%s` | %s | <%s> |\n", $1, $3, $2 }'
