#!/usr/bin/env bash
set -e

# Usage: setup-session-workspace.sh <session-path> <session-desc> <api-root> <lib-opencode> <api-opencode>
S="$1"
DESC="$2"
API_ROOT="$3"
LIB_OC="$4"
API_OC="$5"
BRANCHES_FILE="${S}/.branches"

# Relative path from session dir back to metahub-api root
# Session lives at <api-root>/sessions/<desc>/ → two levels up
REL_API_ROOT="../.."

echo ""
echo "=== Setting up session workspace ==="

# --- opencode.json (root) — strict session-scoped permissions ---
cat > "${S}/opencode.json" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "permission": {
    "external_directory": {
      "/tmp": "allow"
    }
  },
  "agent": {
    "build": {
      "permission": {
        "edit": {
          "./**": "allow"
        },
        "external_directory": {
          "/tmp": "allow"
        }
      }
    },
    "plan": {
      "permission": {
        "edit": {
          "./**": "deny"
        }
      }
    }
  }
}
EOF

# --- .opencode/ directory ---
mkdir -p "${S}/.opencode"

# .opencode/opencode.json — MCP servers + plugins (no external dirs)
cat > "${S}/.opencode/opencode.json" <<'EOF'
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "neo4j-main": {
      "type": "local",
      "command": [
        "neo4j-mcp", "--neo4j-uri", "bolt://neo4j.dev.example.com:7687",
        "--neo4j-username", "neo4j",
        "--neo4j-password", "neo4j123",
        "--neo4j-database", "neo4j",
        "--neo4j-read-only", "false",
        "--neo4j-telemetry", "false"
      ]
    },
    "gocd": {
      "type": "remote",
      "url": "http://localhost:3003/mcp"
    }
  },
  "plugin": [
    "opencode-claude-auth"
  ]
}
EOF

# .opencode/package.json
cat > "${S}/.opencode/package.json" <<'EOF'
{
  "dependencies": {
    "@opencode-ai/plugin": "1.4.3"
  }
}
EOF

# --- Copy skills (metalib full set + propose + excalidraw from api) ---
mkdir -p "${S}/.opencode/skills"
if [ -d "${LIB_OC}/skills" ]; then
  cp -R "${LIB_OC}/skills/"* "${S}/.opencode/skills/" 2>/dev/null || true
fi
if [ -d "${API_OC}/skills/openspec-propose" ]; then
  cp -R "${API_OC}/skills/openspec-propose" "${S}/.opencode/skills/"
fi
if [ -d "${API_OC}/skills/excalidraw-diagram-skill" ]; then
  cp -R "${API_OC}/skills/excalidraw-diagram-skill" "${S}/.opencode/skills/"
fi

# --- Copy commands (metalib full set + propose from api) ---
mkdir -p "${S}/.opencode/command"
if [ -d "${LIB_OC}/command" ]; then
  cp -R "${LIB_OC}/command/"* "${S}/.opencode/command/" 2>/dev/null || true
fi
if [ -f "${API_OC}/command/opsx-propose.md" ]; then
  cp "${API_OC}/command/opsx-propose.md" "${S}/.opencode/command/"
fi

# .opencode/.gitignore
cat > "${S}/.opencode/.gitignore" <<'EOF'
node_modules
package-lock.json
bun.lock
EOF

# --- Install plugin deps ---
cd "${S}/.opencode"
if command -v bun >/dev/null 2>&1; then
  bun install --silent 2>/dev/null || true
elif command -v npm >/dev/null 2>&1; then
  npm install --silent 2>/dev/null || true
fi
cd "${S}"

echo "[workspace] Skills: $(ls -1 .opencode/skills/ | wc -l | tr -d ' ') copied"
echo "[workspace] Commands: $(ls -1 .opencode/command/ | wc -l | tr -d ' ') copied"

# --- .opencodeignore — copy from metahub-api if it exists ---
if [ -f "${API_ROOT}/.opencodeignore" ] && [ ! -f "${S}/.opencodeignore" ]; then
  # Use the session-level ignore from the api repo's sessions template,
  # or fall back to the repo-level one as a base
  if [ -f "${API_ROOT}/sessions/.opencodeignore" ]; then
    cp "${API_ROOT}/sessions/.opencodeignore" "${S}/.opencodeignore"
  else
    cp "${API_ROOT}/.opencodeignore" "${S}/.opencodeignore"
  fi
  echo "[workspace] .opencodeignore copied"
fi

# --- Seed CONTEXT.md from each repo's main branch (if it exists) ---
# Per-repo context: copy CONTEXT.md from each worktree (inherited from main)
for repo_dir in "${S}"/*/; do
  repo_name=$(basename "$repo_dir")
  if [ -f "${repo_dir}/CONTEXT.md" ]; then
    echo "[workspace] Seeded ${repo_name}/CONTEXT.md from repo"
  fi
done

# Session-level context: check if a previous session left one in Obsidian
# The Obsidian MCP server is used at runtime by the agent, not by this script.
# Instead, we create a stub CONTEXT.md that the agent should fill in.
if [ ! -f "${S}/CONTEXT.md" ]; then
  cat > "${S}/CONTEXT.md" <<CTXEOF
# Session Context: ${DESC}

> This file provides context for the AI agent working in this session.
> Fill in the sections below before starting work, or let the agent
> update them as the session progresses.

## Goals

<!-- What does this session set out to achieve? -->

## Relevant Files

<!-- Key files per repo that the agent should read first -->

### metahub-api
<!-- e.g. pkg/api/v1/handlers/repos.go -->

### metalib
<!-- e.g. pkg/backends/neo/collection.go -->

### metahub-admin
<!-- nothing expected -->

### metahub-ui
<!-- e.g. src/pages/Collections.tsx -->

## Architecture Notes

<!-- How the pieces fit together for this session's scope -->

## Patterns to Follow

<!-- "Look at X for an example of how we do Y" -->

## Constraints

<!-- Things to avoid, known pitfalls -->
CTXEOF
  echo "[workspace] Created stub CONTEXT.md — fill in before starting work"
fi

# --- opencode.sh launch wrapper ---
cat > "${S}/opencode.sh" <<'EOF'
#!/usr/bin/env bash
export TMPDIR=/tmp/metahub
export SAFEHOUSE_TRUST_WORKDIR_CONFIG=1
cd "$(dirname "$0")"
safehouse opencode
EOF
chmod +x "${S}/opencode.sh"

# --- Read branch mapping for render line (also used by AGENTS.md below) ---
API_BRANCH=$(grep "^metahub-api=" "${BRANCHES_FILE}" | cut -d= -f2)
LIB_BRANCH=$(grep "^metalib=" "${BRANCHES_FILE}" | cut -d= -f2)
ADMIN_BRANCH=$(grep "^metahub-admin=" "${BRANCHES_FILE}" | cut -d= -f2)
UI_BRANCH=$(grep "^metahub-ui=" "${BRANCHES_FILE}" | cut -d= -f2)
CE_BRANCH=$(grep "^community-edition=" "${BRANCHES_FILE}" | cut -d= -f2)
# Default unused dev-stack slots to main
CE_BRANCH="${CE_BRANCH:-main}"

# --- Taskfile.yml — forwarding tasks using relative path ---
cat > "${S}/Taskfile.yml" <<EOF
version: '3'

tasks:
  dev:
    desc: "Iterate dev version (all repos, or only the repos passed as args)"
    cmds:
      # Forward any extra args as a repo allow-list:
      #   task dev                         -> bump all repos
      #   task dev -- metahub-ui           -> bump only metahub-ui
      #   task dev -- metahub-ui metalib   -> bump only those two
      - task -d "${REL_API_ROOT}" session:dev -- ${DESC} {{.CLI_ARGS}}
  render:
    desc: "Print the 'task render:*' line for the flux dev-stack (slots: API:LIB:ADMIN:CE:OCI:UI)"
    silent: true
    cmds:
      - echo "task render:${API_BRANCH}:${LIB_BRANCH}:${ADMIN_BRANCH}:${CE_BRANCH}:dev:${UI_BRANCH}"
  finish:
    desc: "Finalize session: test, sonar, archive, push, comment MRs"
    cmds:
      - task -d "${REL_API_ROOT}" session:finish -- ${DESC}
  suspend:
    desc: "Suspend session (remove worktrees, keep branches)"
    cmds:
      - task -d "${REL_API_ROOT}" session:suspend -- ${DESC}
  resume:
    desc: "Resume session (recreate worktrees)"
    cmds:
      - task -d "${REL_API_ROOT}" session:resume -- ${DESC}
  version:*:*:
    desc: "Set release version for a repo"
    vars:
      REPO: '{{index .MATCH 0}}'
      LEVEL: '{{index .MATCH 1}}'
    cmds:
      - task -d "${REL_API_ROOT}" session:version:{{.REPO}}:{{.LEVEL}} -- ${DESC}
  close:*:
    desc: "Close MR for unchanged repo"
    vars:
      REPO: '{{index .MATCH 0}}'
    cmds:
      - task -d "${REL_API_ROOT}" session:close:{{.REPO}} -- ${DESC}
EOF

# Append the local (no-network) git helper tasks. These are
# self-contained shell loops driven by the session's .branches file, so
# they need no interpolation — use a QUOTED heredoc so the embedded
# shell \${vars} survive verbatim into the generated Taskfile.
cat >> "${S}/Taskfile.yml" <<'EOF'
  status:
    desc: "Show git status (committed?, branch, ahead/behind) for every worktree"
    silent: true
    cmds:
      - |
        set -uo pipefail
        SESSION_DIR="$(pwd)"
        BRANCHES_FILE="${SESSION_DIR}/.branches"
        if [ ! -f "${BRANCHES_FILE}" ]; then
          echo "ERROR: .branches file not found in ${SESSION_DIR}" >&2
          exit 1
        fi

        DIRTY=0
        printf "%-18s %-30s %-6s %-12s %s\n" "REPO" "BRANCH" "AHEAD" "WORKTREE" "HEAD"
        printf "%-18s %-30s %-6s %-12s %s\n" "----" "------" "-----" "--------" "----"
        while IFS='=' read -r REPO BRANCH; do
          [ -z "${REPO}" ] && continue
          WT="${SESSION_DIR}/${REPO}"
          if [ ! -d "${WT}/.git" ] && [ ! -f "${WT}/.git" ]; then
            printf "%-18s %-30s %-6s %-12s %s\n" "${REPO}" "${BRANCH}" "-" "MISSING" "(suspended?)"
            continue
          fi
          # Refresh remote-tracking ref quietly so ahead-count is accurate.
          git -C "${WT}" fetch -q origin "${BRANCH}" 2>/dev/null || true
          COUNT="$(git -C "${WT}" status --porcelain | wc -l | tr -d ' ')"
          if [ "${COUNT}" = "0" ]; then
            CLEAN="clean"
          else
            CLEAN="dirty(${COUNT})"
            DIRTY=1
          fi
          AHEAD="$(git -C "${WT}" rev-list --count "origin/${BRANCH}..HEAD" 2>/dev/null || echo "?")"
          HEAD="$(git -C "${WT}" log -1 --format='%h %s' 2>/dev/null)"
          printf "%-18s %-30s %-6s %-12s %s\n" "${REPO}" "${BRANCH}" "${AHEAD}" "${CLEAN}" "${HEAD}"
        done < "${BRANCHES_FILE}"

        echo ""
        if [ "${DIRTY}" -ne 0 ]; then
          echo "Some worktrees have uncommitted changes (dirty)."
        else
          echo "All worktrees are committed."
        fi
  push:
    desc: "Verify every worktree is committed, then push all branches"
    silent: true
    cmds:
      - |
        set -euo pipefail
        SESSION_DIR="$(pwd)"
        BRANCHES_FILE="${SESSION_DIR}/.branches"
        if [ ! -f "${BRANCHES_FILE}" ]; then
          echo "ERROR: .branches file not found in ${SESSION_DIR}" >&2
          exit 1
        fi

        # --- Phase 1: verify all worktrees are fully committed -----------
        DIRTY=0
        while IFS='=' read -r REPO BRANCH; do
          [ -z "${REPO}" ] && continue
          WT="${SESSION_DIR}/${REPO}"
          if [ ! -d "${WT}/.git" ] && [ ! -f "${WT}/.git" ]; then
            echo "[${REPO}] worktree missing (suspended?) — skipping"
            continue
          fi
          STATUS="$(git -C "${WT}" status --porcelain)"
          if [ -n "${STATUS}" ]; then
            echo "[${REPO}] UNCOMMITTED changes:"
            echo "${STATUS}" | sed 's/^/    /'
            DIRTY=1
          fi
        done < "${BRANCHES_FILE}"

        if [ "${DIRTY}" -ne 0 ]; then
          echo ""
          echo "ABORT: commit (or stash) the changes above before pushing." >&2
          exit 1
        fi
        echo "All worktrees are committed."

        # --- Phase 2: push each branch ----------------------------------
        echo ""
        FAILED=0
        while IFS='=' read -r REPO BRANCH; do
          [ -z "${REPO}" ] && continue
          WT="${SESSION_DIR}/${REPO}"
          if [ ! -d "${WT}/.git" ] && [ ! -f "${WT}/.git" ]; then
            continue
          fi
          CUR="$(git -C "${WT}" rev-parse --abbrev-ref HEAD)"
          AHEAD="$(git -C "${WT}" rev-list --count "origin/${BRANCH}..HEAD" 2>/dev/null || echo "?")"
          if [ "${AHEAD}" = "0" ]; then
            echo "[${REPO}] ${BRANCH} already up to date — nothing to push"
            continue
          fi
          echo "[${REPO}] pushing ${CUR} -> origin/${BRANCH} (${AHEAD} commit(s))"
          if ! git -C "${WT}" push origin "HEAD:${BRANCH}"; then
            echo "[${REPO}] PUSH FAILED" >&2
            FAILED=1
          fi
        done < "${BRANCHES_FILE}"

        echo ""
        if [ "${FAILED}" -ne 0 ]; then
          echo "DONE with errors: one or more pushes failed (see above)." >&2
          exit 1
        fi
        echo "DONE: all branches pushed."
EOF

# --- AGENTS.md — auto-generated with session context ---
# (branch vars already read above for the render task)
cat > "${S}/AGENTS.md" <<EOF
# Session: ${DESC}

## Overview

This is a self-contained session workspace. All repos are checked out
as worktrees within this directory. The opencode instance running here
can edit only files within this session.

Start opencode from this directory: \`./opencode.sh\`

**Read \`CONTEXT.md\` first** — it contains session goals, relevant
files, and architecture notes. Each repo worktree may also have its
own \`CONTEXT.md\` with repo-specific context from previous sessions.

## Repos and Branches

| Repo | Branch | Worktree |
|------|--------|----------|
| metahub-api | \`${API_BRANCH}\` | \`metahub-api/\` |
| metalib | \`${LIB_BRANCH}\` | \`metalib/\` |
| metahub-admin | \`${ADMIN_BRANCH}\` | \`metahub-admin/\` |
| metahub-ui | \`${UI_BRANCH}\` | \`metahub-ui/\` |

## Session Tasks

All tasks run from this session directory:

| Task | Command |
|------|---------|
| Show git status of all worktrees | \`task status\` |
| Verify committed + push all branches | \`task push\` |
| Iterate dev version (all repos) | \`task dev\` |
| Iterate dev version (only some repos) | \`task dev -- metahub-ui metalib\` |
| Print flux render line | \`task render\` |
| Finalize session | \`task finish\` |
| Set release version | \`task version:REPO:LEVEL\` |
| Close unchanged repo | \`task close:REPO\` |
| Suspend (free disk) | \`task suspend\` |
| Resume | \`task resume\` |

\`task status\` is read-only (committed?, branch, ahead/behind per
repo). \`task push\` aborts if any worktree is dirty, then pushes each
branch that is ahead of its remote. Both iterate the \`.branches\`
mapping, so they stay correct as repos are added or closed.

Examples:
\`\`\`bash
task version:metalib:minor
task version:metahub-api:patch
task close:metahub-admin
task close:metahub-ui
\`\`\`

## Commits and Pushes

- **Commit after each completed OpenSpec task** — keep commits small and traceable.
- **Push to the remote branch only at the end** — after all tasks in a change are done and the "After Each Change" checklist passes.
- Do not commit secrets (\`.env\`, credentials, keys). Warn if asked to.

## After Each Change

Run these steps **from within the repo worktree** that was modified
(e.g. \`cd metahub-ui/\`).

### 1. Tests — run the full test suite with coverage

\`\`\`bash
./test.sh 2>&1 > test.log
\`\`\`

### 2. SonarQube analysis — submit coverage results

\`\`\`bash
source .env
./misc/bin/sonar.sh
\`\`\`

### 3. SonarQube acceptance

Use the SonarQube MCP server to verify quality gate status.

All three steps must pass before a change is considered complete.

## Partial Test Runs

When running tests for a single package, always forward log output into a file:

\`\`\`bash
go test -timeout 300s -count=1 -coverprofile=/tmp/cover.out ./pkg/some/... 2>&1 > test.log
tail -3 test.log
\`\`\`

## Documentation

### DevDiary (Obsidian)

After each major change, create a DevDiary entry in Obsidian at:

\`\`\`
Metahub/DevDiary/sessions/${DESC}/YYYY-MM-DD-hhmm-<change-name>.md
\`\`\`

### Specs and Docs

Use \`Metahub/docs\` in Obsidian to document specs, graph constructs, and architectural decisions.

## Context Notes (CONTEXT.md)

Before calling \`task finish\`, write context notes that help the next
session (or the next agent) understand what was done and where things stand.

### Per-repo CONTEXT.md

In each **modified** repo worktree, write or update \`CONTEXT.md\` with:

- **What changed** — summary of modifications in this repo
- **Key files** — the important files for understanding the changes
- **Architecture notes** — how the changes fit into the repo's structure
- **Open questions** — anything unresolved or deferred
- **Gotchas** — pitfalls discovered during development

This file gets committed when \`task finish\` runs and merges into
\`main\`, so the next session inherits it automatically.

### Session-level CONTEXT.md

Update the \`CONTEXT.md\` in this session root with:

- **What was achieved** — session goals vs actual outcome
- **Cross-repo relationships** — how changes in one repo depend on another
- **Deferred work** — what was discovered but not done
- **Next steps** — what a follow-up session should tackle

This file is written to Obsidian as a DevDiary entry by the agent.

## Event Logging (VictoriaLogs)

\`\`\`bash
VL_AGENT="claude-opus-4-6" ${REL_API_ROOT}/misc/bin/vl-event.sh <event_type> <phase> "<message>" [key=value ...]
\`\`\`

### When to emit events

| Moment | event_type | phase | Extra fields |
|---|---|---|---|
| Before starting an OpenSpec task | \`task\` | \`start\` | \`change\`, \`task_num\`, \`task_total\`, \`task_desc\` |
| After completing an OpenSpec task | \`task\` | \`end\` | \`change\`, \`task_num\`, \`task_total\`, \`task_desc\`, \`duration_s\` |
| After creating an OpenSpec artifact | \`artifact\` | \`create\` | \`change\`, \`artifact\` |
| When a new change is created | \`change\` | \`start\` | \`change\` |
| When a change is archived | \`change\` | \`archive\` | \`change\` |
EOF

echo "[workspace] AGENTS.md generated"
echo "[workspace] opencode.sh generated"
echo "[workspace] Taskfile.yml generated"
