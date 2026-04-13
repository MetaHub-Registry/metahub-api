#!/usr/bin/env bash
set -e

# Usage: setup-session-workspace.sh <session-path> <session-desc> <api-root> <lib-opencode> <api-opencode>
S="$1"
DESC="$2"
API_ROOT="$3"
LIB_OC="$4"
API_OC="$5"
BRANCHES_FILE="${S}/.branches"

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

# --- opencode.sh launch wrapper ---
cat > "${S}/opencode.sh" <<'EOF'
#!/usr/bin/env bash
export TMPDIR=/tmp/metahub
export SAFEHOUSE_TRUST_WORKDIR_CONFIG=1
cd "$(dirname "$0")"
safehouse opencode
EOF
chmod +x "${S}/opencode.sh"

# --- Taskfile.yml — forwarding tasks ---
cat > "${S}/Taskfile.yml" <<EOF
version: '3'

tasks:
  dev:
    desc: "Iterate dev version in all repos"
    cmds:
      - task -d "${API_ROOT}" session:dev -- ${DESC}
  finish:
    desc: "Finalize session: test, sonar, archive, push, comment MRs"
    cmds:
      - task -d "${API_ROOT}" session:finish -- ${DESC}
  suspend:
    desc: "Suspend session (remove worktrees, keep branches)"
    cmds:
      - task -d "${API_ROOT}" session:suspend -- ${DESC}
  resume:
    desc: "Resume session (recreate worktrees)"
    cmds:
      - task -d "${API_ROOT}" session:resume -- ${DESC}
EOF

# --- AGENTS.md — auto-generated with session context ---
API_BRANCH=$(grep "^metahub-api=" "${BRANCHES_FILE}" | cut -d= -f2)
LIB_BRANCH=$(grep "^metalib=" "${BRANCHES_FILE}" | cut -d= -f2)
ADMIN_BRANCH=$(grep "^metahub-admin=" "${BRANCHES_FILE}" | cut -d= -f2)
UI_BRANCH=$(grep "^metahub-ui=" "${BRANCHES_FILE}" | cut -d= -f2)

cat > "${S}/AGENTS.md" <<EOF
# Session: ${DESC}

## Overview

This is a self-contained session workspace. All repos are checked out
as worktrees within this directory. The opencode instance running here
can edit only files within this session.

## Repos and Branches

| Repo | Branch | Worktree |
|------|--------|----------|
| metahub-api | \`${API_BRANCH}\` | \`metahub-api/\` |
| metalib | \`${LIB_BRANCH}\` | \`metalib/\` |
| metahub-admin | \`${ADMIN_BRANCH}\` | \`metahub-admin/\` |
| metahub-ui | \`${UI_BRANCH}\` | \`metahub-ui/\` |

## Session Tasks

Run these from within this session directory:

| Task | Command |
|------|---------|
| Iterate dev version | \`task dev\` |
| Finalize session | \`task finish\` |
| Suspend (free disk) | \`task suspend\` |
| Resume | \`task resume\` |

Or from the metahub-api root:

| Task | Command |
|------|---------|
| Set release version | \`task session:version:REPO:LEVEL -- ${DESC}\` |
| Close unchanged repo | \`task session:close:REPO -- ${DESC}\` |

## Commits and Pushes

- **Commit after each completed OpenSpec task** — keep commits small and traceable.
- **Push to the remote branch only at the end** — after all tasks in a change are done and the "After Each Change" checklist passes.
- Do not commit secrets (\`.env\`, credentials, keys). Warn if asked to.

## After Each Change

Run these steps in **only the repo(s) that were actually modified**.

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

## Documentation

### DevDiary (Obsidian)

After each major change, create a DevDiary entry in Obsidian at:

\`\`\`
Metahub/DevDiary/sessions/${DESC}/YYYY-MM-DD-hhmm-<change-name>.md
\`\`\`

### Specs and Docs

Use \`Metahub/docs\` in Obsidian to document specs, graph constructs, and architectural decisions.

## Event Logging (VictoriaLogs)

\`\`\`bash
VL_AGENT="claude-opus-4-6" ${API_ROOT}/misc/bin/vl-event.sh <event_type> <phase> "<message>" [key=value ...]
\`\`\`

See the main AGENTS.md at \`${API_ROOT}/AGENTS.md\` for full event logging conventions.
EOF

echo "[workspace] AGENTS.md generated"
echo "[workspace] opencode.sh generated"
echo "[workspace] Taskfile.yml generated"
