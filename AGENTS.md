# Agent Instructions

## Sessions and Git Worktrees

A **session** groups related work across multiple repos. Each repo gets its own MR-numbered branch (e.g. `99-add-oauth` in metahub-api, `123-add-oauth` in metalib). The session name is the coordination link.

```
sessions/add-oauth/
├── .branches              # metadata: repo=branch mapping + status
├── metahub-api/           # git worktree on branch 99-add-oauth
├── metalib/               # git worktree on branch 123-add-oauth
├── metahub-admin/         # git worktree on branch 45-add-oauth
├── metahub-ui/            # git worktree on branch 4-add-oauth
└── community-edition/     # git worktree on branch 12-add-oauth
```

### Rules

- Each worktree has its own OpenSpec change(s). The **session name** is the coordination link across repos.
- Each worktree is independent; changes in one do not block changes in another.
- Within a session, **additional OpenSpec changes** may be created in any repo for:
  - **Sub-changes** — when a feature or migration within the session blows the scope of the main change.
  - **Deferred changes** — future work discovered during the session but not acted on now. These can be carried into a later session.
- **Version levels are decided at session end**, per-repo, based on actual changes. During the session, versions use `-devN` on the current base version (e.g. `3.0.0-dev1`, not `3.0.1-dev1`).
- Each worktree ships its own **codebase knowledge graph** at `.understand-anything/knowledge-graph.json` (see "Codebase Knowledge Graph" below). Because worktrees branch off `main`, the graph is inherited at session start; treat it as the first stop for orientation and keep it fresh as code changes.

### Tools

Use the MCP servers to do inspection instead of trying it with bash

- neo4j-main: metahub-dev instance which is tied to the GoCD livecycle
- neo4j-local: compose neo4j instance for fast local iterations
- jeager-local: copose jaeger instance to check traces of metahub-api interactions.

Please use the MCP

## Codebase Knowledge Graph

This repo ships a pre-built knowledge graph of its own source at
`.understand-anything/knowledge-graph.json` (nodes, edges, architectural
layers, and a guided tour). **Consult it before doing broad code exploration** —
it is faster and cheaper than blind file/grep search for "where does X live",
"what depends on Y", or "how is the registry wired together".

### How to use it

- **First stop for orientation.** Grep the graph for relevant `name`/`summary`/`tags`
  to find nodes, then follow `imports`/`calls`/`contains` edges for the 1-hop
  subgraph. Don't dump the whole file into context — search the sections you need.
- **Resolve symbols via the graph, not `grep`.** Node IDs are
  `function:<relpath>:<Name>` (methods likewise) and `calls` edges run
  caller → callee, so "who uses this function?" is a graph query — not a source
  grep. Reserve `rg`/grep for string/literal search; for "where is X defined",
  "who calls X", "what depends on X", query the graph first:

  ```bash
  KG=.understand-anything/knowledge-graph.json

  # Where is it defined?
  jq -r '.nodes[] | select(.id|endswith(":ProfileGetOne")) | .id' "$KG"

  # Who calls it? (the usages you'd otherwise grep for)
  jq -r '.edges[] | select(.type=="calls" and (.target|endswith(":ProfileGetOne"))) | .source' "$KG"

  # What does it call / import? (1-hop out)
  jq -r '.edges[] | select((.type=="calls" or .type=="imports") and (.source|endswith(":ProfileGetOne"))) | .target' "$KG"
  ```
- **Graph skills** (in `.opencode/skills/`, available as project skills):
  - `understand-chat` — ask questions about the codebase using the graph.
  - `understand-explain` — deep-dive a specific file/function/module.
  - `understand-diff` — analyze the current git diff against the graph (changed
    + affected components, risks); writes a dashboard overlay.
  - `understand-onboard` — generate an onboarding guide for the project.
- **Scope.** The graph is scoped to code + key infra (see
  `.understand-anything/.understandignore`); OpenSpec docs, Bruno collections,
  and misc yaml are intentionally excluded.

### Keeping it fresh

The graph is tied to a git commit (`.understand-anything/meta.json`) with a
fingerprints baseline for fast incremental updates. After material code changes,
regenerate it with the global `/understand` tool (re-analyzes only changed files).
A stale-but-present graph is still a useful map; note staleness if the commit
hash differs from `HEAD`.

### Session Lifecycle

1. **Create** — `task session:new -- <description>`. Sets up worktrees, branches, draft MRs, bumps to `-dev1`.
2. **Work** — use OpenSpec propose/apply/explore within each worktree as needed. **Before broad code exploration, consult that worktree's knowledge graph** (`.understand-anything/knowledge-graph.json`) or the `understand-*` skills — see "Codebase Knowledge Graph". Note the graph's commit hash against `HEAD`; if it is stale, it is still a useful map.
3. **Iterate** — `task session:dev -- <description>` (optional). Bumps all repos to next `-devN`.
4. **Finish** — `task session:finish -- <description>`. Archives OpenSpec changes, runs tests + sonar, pushes all branches, adds summary comments to MRs.
5. **Version** — per modified repo: `task session:version:<repo>:<level> -- <description>`. Sets the release version, marks MR as ready.
6. **Close** — per unchanged repo: `task session:close:<repo> -- <description>`. Closes the MR without merge.
7. **Merge** — the human reviews and merges each ready MR.

## Use GitLab

The command `glab` allows interaction with GitLab (issues, MRs, pipelines, etc.).

## Commits and Pushes

- **Commit after each completed OpenSpec task** — keep commits small and traceable.
- **Push to the remote branch only at the end** — after all tasks in a change are done and the "After Each Change" checklist passes.
- Do not commit secrets (`.env`, credentials, keys). Warn if asked to.

## After Each Change

Run these steps in **only the repo(s) that were actually modified** during the change.

### 1. Tests — run the full test suite with coverage

Forward the log so that you can search it after it ran.

```bash
./test.sh 2>&1 > test.log
```

### 2. SonarQube analysis — submit coverage results

```bash
source .env
./misc/bin/sonar.sh
```

### 3. SonarQube acceptance

Make sure the current branch within the SonarQube project is accepted.
Use the SonarQube MCP server to verify quality gate status.

All three steps must pass before a change is considered complete.

### 4. Refresh the knowledge graph (if code changed)

If the change touched in-scope source (see `.understand-anything/.understandignore`),
keep the worktree's graph in sync so the next task/agent inherits an accurate map:

- Run the `understand-diff` skill to review changed + affected components and risks
  against the existing graph.
- If the changes are material, regenerate with the global `/understand` tool — it
  re-analyzes only changed files using the fingerprints baseline in
  `.understand-anything/meta.json`.
- Commit the updated `.understand-anything/` artifacts alongside the code so the
  graph's commit hash tracks `HEAD`. Doc-only, Bruno, or yaml-only changes can
  skip this step.

## Partial Test Runs

When running tests for a single package, always forward log output into a file so that you can search for text snippets after it ran without running it again.

**Don't do this:**

```bash
go test -timeout 300s -count=1 -coverprofile=/tmp/neo_cover2.out ./pkg/backends/neo/... 2>&1 | tail -3
```

**Do this instead:**

```bash
go test -timeout 300s -count=1 -coverprofile=/tmp/neo_cover2.out ./pkg/backends/neo/... 2>&1 > test.log
tail -3 test.log
```

## Documentation

### DevDiary (Obsidian)

The **MetaHub session DevDiary** index lives in Obsidian at `Metahub/DevDiary/README.md` (`[[Metahub/DevDiary/README]]`) — it lists every session and links its entries. Register your session there.

After each major change, create a DevDiary entry in Obsidian at:

```
Metahub/DevDiary/sessions/<session-name>/YYYY-MM-DD-hhmm-<change-name>.md
```

The **session name** comes from the session description (e.g. `add-oauth-support`).

Describe what was done, why, and link to any specs or graph constructs.

At release time, create a release note at:

```
Metahub/DevDiary/releases/<repo>/v<X.Y.Z>.md
```

This links back to the session(s) that contributed to the release.

### Specs and Docs

Use `Metahub/docs` in Obsidian to document specs, graph constructs, and architectural decisions. Link these from the DevDiary.

## Event Logging (VictoriaLogs)

Send structured events to VictoriaLogs (`vl.example.com`) to track agent activity. Use the helper script:

```bash
VL_AGENT="claude-opus-4-6" ./misc/bin/vl-event.sh <event_type> <phase> "<message>" [key=value ...]
```

Set `VL_AGENT` to the model ID powering the agent. The script auto-detects `repo` and `branch` from git.

### When to emit events

| Moment | event_type | phase | Extra fields |
|---|---|---|---|
| Before starting an OpenSpec task | `task` | `start` | `change`, `task_num`, `task_total`, `task_desc` |
| After completing an OpenSpec task | `task` | `end` | `change`, `task_num`, `task_total`, `task_desc`, `duration_s` |
| After creating an OpenSpec artifact | `artifact` | `create` | `change`, `artifact` (e.g. `proposal`, `design`, `tasks`) |
| When a new change is created | `change` | `start` | `change` |
| When a change is archived | `change` | `archive` | `change` |

### Examples

```bash
# Task start
VL_AGENT="claude-opus-4-6" ./misc/bin/vl-event.sh task start "Implement OAuth handler" change=add-auth task_num=3 task_total=7

# Task end (with duration)
VL_AGENT="claude-opus-4-6" ./misc/bin/vl-event.sh task end "Implement OAuth handler" change=add-auth task_num=3 task_total=7 duration_s=42

# Artifact created
VL_AGENT="claude-opus-4-6" ./misc/bin/vl-event.sh artifact create "Created design.md" change=add-auth artifact=design

# Change lifecycle
VL_AGENT="claude-opus-4-6" ./misc/bin/vl-event.sh change start "Started add-auth" change=add-auth
VL_AGENT="claude-opus-4-6" ./misc/bin/vl-event.sh change archive "Archived add-auth" change=add-auth
```

The script fails silently (stderr warning only) if VictoriaLogs is unreachable — it never blocks agent work.
