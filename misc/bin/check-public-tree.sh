#!/usr/bin/env bash
# Fail when a tracked path would publish internal working material.
#
# Mirrors the export policy in MetaHub-Registry/openspec
# (templates/export-excludes.txt) plus the internal tooling that policy names.
# Run before a visibility flip and from CI on a repository that is already
# public:  ./misc/bin/check-public-tree.sh [repo-path]
set -euo pipefail

patterns=(
  '(^|/)AGENTS\.md$'
  '(^|/)CONTEXT\.md$'
  '(^|/)openspec/'
  '(^|/)sessions/'
  '(^|/)\.branches$'
  '^\.devcontainer/'
  '^\.opencode/'
  '^\.understand-anything/'
  '(^|/)agent\.compose\.(yaml|lock)$'
  '(^|/)misc/gocd/'
  '(^|/)misc/bruno/'
  '(^|/)misc/compose/'
  '(^|/)docs/session-artifacts/'
  '(^|/)Taskfile_session\.yml$'
  '(^|/)Taskfile_e2e_dev\.yml$'
  '(^|/)docker-compose-monitoring\.yaml$'
  '(^|/)docker-compose-sonarcube\.yaml$'
  '(^|/)sonar-project\.properties$'
  '(^|/)misc/bin/(opencode|setup-session-workspace|vl-event|sonar)\.sh$'
  '(^|/)\.gocd-(run|setup)\.sh$'
  '(^|/)misc/gitlab-ci[^/]*$'
  '(^|/)misc/gitlabci\.yaml$'
  '(^|/)PROD_PUSH\.md$'
  '(^|/)misc/bin/(build-prod|copy)\.sh$'
  '(^|/)misc/docker/mh-api/'
  '(^|/)docs/session-artifacts/'
  '(^|/)\.scannerwork/'
  '(^|/)\.scannerwork/'
  '(^|/)(report\.json|pre_test\.log|test\.log|sonar\.log|coverage\.txt|coverage\.xml)$'
)

target="${1:-.}"

if ! git -C "$target" rev-parse --git-dir >/dev/null 2>&1; then
  echo "not a git repository: $target" >&2
  exit 2
fi

expr=$(IFS='|'; echo "${patterns[*]}")
hits=$(git -C "$target" ls-files | grep -E "$expr" || true)

if [ -n "$hits" ]; then
  echo "FAIL: $target tracks paths excluded from publication:" >&2
  while IFS= read -r path; do
    printf '  %s\n' "$path" >&2
  done <<< "$hits"
  exit 1
fi

# Content-level leaks the path rules cannot see: a private AWS account, an
# internal domain, an absolute maintainer path. Test fixtures use different
# internal names and are handled by the launch-readiness scrub, not here.
content_hits=$(git -C "$target" grep -nE \
  '[0-9]{12}\.dkr\.ecr|metahub(-dev)?\.hvk|~/src/gitlab\.com' \
  -- ':!*.gitleaks.toml' ':!misc/bin/check-public-tree.sh' 2>/dev/null || true)

if [ -n "$content_hits" ]; then
  echo "FAIL: $target tracks content that names internal infrastructure:" >&2
  printf '%s\n' "$content_hits" | while IFS= read -r line; do
    printf '  %s\n' "$line" >&2
  done
  exit 1
fi

echo "ok: $target publishes no excluded paths or internal identifiers"
