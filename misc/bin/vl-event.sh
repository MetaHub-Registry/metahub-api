#!/bin/bash
#
# vl-event.sh — Send structured events to VictoriaLogs
#
# Usage:
#   vl-event.sh <event_type> <phase> <message> [extra_json_fields...]
#
# Arguments:
#   event_type   One of: task, artifact, change, test, sonar
#   phase        One of: start, end, create, archive, error
#   message      Human-readable event description
#   extra fields Key=value pairs appended as JSON fields (optional)
#
# Examples:
#   vl-event.sh task start "Implement OAuth handler" change=add-auth repo=metahub-api task_num=3 task_total=7
#   vl-event.sh task end   "Implement OAuth handler" change=add-auth repo=metahub-api task_num=3 task_total=7 duration_s=42
#   vl-event.sh artifact create "Created design.md" change=add-auth repo=metahub-api artifact=design
#   vl-event.sh change archive "Archived add-auth" change=add-auth repo=metahub-api
#
# Environment:
#   VL_HOST     VictoriaLogs host (default: vl.example.com)
#   VL_STREAM   Stream identifier (default: opencode)
#   VL_AGENT    Agent/model identifier (default: unknown)

set -euo pipefail

# --- arguments ---
EVENT_TYPE="${1:?Usage: vl-event.sh <event_type> <phase> <message> [key=value ...]}"
PHASE="${2:?Usage: vl-event.sh <event_type> <phase> <message> [key=value ...]}"
MSG="${3:?Usage: vl-event.sh <event_type> <phase> <message> [key=value ...]}"
shift 3

# --- defaults ---
VL_HOST="${VL_HOST:-vl.example.com}"
VL_STREAM="${VL_STREAM:-opencode}"
VL_AGENT="${VL_AGENT:-unknown}"

# --- detect context from git ---
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
REPO=$(basename "$(git rev-parse --show-toplevel 2>/dev/null)" 2>/dev/null || echo "unknown")

# --- build JSON payload ---
JSON=$(cat <<EOF
{"_msg":"${MSG}","_time":"0","stream":"${VL_STREAM}","event_type":"${EVENT_TYPE}","phase":"${PHASE}","agent":"${VL_AGENT}","repo":"${REPO}","branch":"${BRANCH}"
EOF
)
# strip trailing newline and closing brace — we'll append extra fields
JSON="${JSON%\}}"

# append extra key=value pairs
for arg in "$@"; do
  KEY="${arg%%=*}"
  VAL="${arg#*=}"
  JSON="${JSON},\"${KEY}\":\"${VAL}\""
done

JSON="${JSON}}"

# --- send ---
echo "${JSON}" | curl -sf -X POST \
  -H 'Content-Type: application/stream+json' \
  --data-binary @- \
  "http://${VL_HOST}/insert/jsonline?_stream_fields=stream&_time_field=_time&_msg_field=_msg" \
  >/dev/null 2>&1 || echo "[vl-event] warning: failed to send event to ${VL_HOST}" >&2
