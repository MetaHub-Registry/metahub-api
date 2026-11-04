#!/usr/bin/env bash
# Fail when a pull-request-triggered workflow can read a repository secret.
#
# A fork PR does not receive secrets, so any job that needs one is silently
# skipped (or fails). Either the job moves to the integration lane or the secret
# goes away. This check makes that rule enforceable instead of a review habit.
#
# Transitional exception: a workflow that contains the exact marker
#
#   # TRANSITIONAL(private-core): remove after metahub-core is public
#
#   may reference secrets.METAHUB_CORE_RO_TOKEN until metahub-core is public.
#   Any other secret in that file still fails. The marker is deleted with the
#   carve-out, so the exception cannot outlive its reason.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
workflow_dir="${repo_root}/.github/workflows"

if [ ! -d "$workflow_dir" ]; then
  echo "no workflow directory: $workflow_dir" >&2
  exit 2
fi

marker='# TRANSITIONAL(private-core): remove after metahub-core is public'
allowed_secret='METAHUB_CORE_RO_TOKEN'
status=0

for wf in "$workflow_dir"/*.yml "$workflow_dir"/*.yaml; do
  [ -f "$wf" ] || continue
  name="$(basename "$wf")"

  if grep -qE '^[[:space:]]*pull_request_target[[:space:]]*:' "$wf"; then
    echo "FAIL: ${name} uses pull_request_target" >&2
    status=1
    continue
  fi

  # Is this workflow triggered by pull_request?
  if ! awk '
      /^on:/ { in_on = 1; next }
      in_on && /^[^[:space:]]/ { in_on = 0 }
      in_on && /pull_request/ { found = 1 }
      END { exit(found ? 0 : 1) }
    ' "$wf"; then
    continue
  fi

  refs="$(grep -nE '\$\{\{[^}]*secrets\.[A-Za-z_][A-Za-z0-9_]*' "$wf" || true)"
  [ -z "$refs" ] && continue

  while IFS= read -r line; do
    [ -z "$line" ] && continue
    secret="$(printf '%s\n' "$line" | sed -E 's/.*secrets\.([A-Za-z_][A-Za-z0-9_]*).*/\1/' | head -1)"
    if grep -qF "$marker" "$wf" && [ "$secret" = "$allowed_secret" ]; then
      continue
    fi
    echo "FAIL: ${name} is pull-request triggered and references secrets.${secret}" >&2
    echo "      ${line}" >&2
    status=1
  done <<< "$refs"
done

if [ "$status" -ne 0 ]; then
  echo "" >&2
  echo "Pull-request workflows must run without secrets; move the job to the" >&2
  echo "integration lane or remove the secret access." >&2
  exit 1
fi

echo "ok: no pull-request workflow reads a secret"
