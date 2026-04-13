# Agent Instructions

## Sessions and Git Worktrees

A **session** groups related work across multiple repos under a single branch name.

```
sessions/<main-change-name>/
├── metahub-api/      # git worktree on branch <main-change-name>
├── metalib/          # git worktree on branch <main-change-name>
├── metahub-ui/       # git worktree on branch <main-change-name>
└── metahub-admin/    # git worktree on branch <main-change-name>
```

### Rules

- Each worktree has its own OpenSpec change, all sharing the same branch name `<main-change-name>`. The **shared branch name** is the coordination link across repos — no manifest needed.
- Each worktree is independent; changes in one do not block changes in another.
- Within a session, **additional OpenSpec changes** may be created in any repo for:
  - **Sub-changes** — when a feature or migration within the session blows the scope of the main change.
  - **Deferred changes** — future work discovered during the session but not acted on now. These can be carried into a later session.

### Session Lifecycle

1. **Create** — set up worktrees across repos with the same branch name.
2. **Work** — use OpenSpec propose/apply/explore within each worktree as needed.
3. **Complete** — run the "After Each Change" checklist per modified repo, archive changes, push.

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

After each major change, create a DevDiary entry in Obsidian at:

```
Metahub/DevDiary/<version>/YYYY-MM-DD-hhmm-<change-name>.md
```

The **version** is read from `.bumpversion.cfg` (`current_version`), e.g. `v2.8.0`.

Describe what was done, why, and link to any specs or graph constructs.

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
