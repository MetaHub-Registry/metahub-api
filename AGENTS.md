# Agent Instructions

## Use gitlab 

The command `glab` allows to interact with gitlab.

### Setup 

#### OpenSpec Change

Within each session designs, tasks, etc are organized within openspec changes.
Create an initial one, feel free to add more changes within a session.

#### Branches & Merge request

Create branch for a session in each of the metahub repos 

- metahub-api
- metalib
- metahub-ui

#### Merge Requests && Worktrees

Create a MR in each of the repos for the branch representing the session.

Create session dir within `./sessions` in metahub-api. That's where the session is going to work from.

Create `git worktree` for each repo within the session dir.

```
metahub-api $ git worktree add  sessions/change123/metahub-api change123
metalib $ git worktree add  sessions/change123/metalib change123
metahub-ui $ git worktree add  sessions/change123/metahub-ui change123
```

## After Each Change

At the end of every change, run the following two scripts in order:

1. **Tests** — run the full test suite with coverage:
   Forward the log so that you can search it after it ran.

   ```bash
   ./test.sh 2>&1 > test.log
   ```

2. **SonarQube analysis** — submit coverage results to SonarQube:

   ```bash
   source .env
   ./misc/bin/sonar.sh
   ```

3. **SonarQube acceptance** - make sure the current branch within the SonarQube project is accepted.
   use the SonarQube MCP server

Both scripts must pass before a change is considered complete.

## Partial Test Runs

In case you run a test for a package make sure to foward the log output into a file, so that you can search for text snippets after it ran without running it again.

Don't to this:

```bash
go test -timeout 300s -count=1 -coverprofile=/tmp/neo_cover2.out ./pkg/backends/neo/... 2>&1 | tail -3
```

Instead to this:

```bash
go test -timeout 300s -count=1 -coverprofile=/tmp/neo_cover2.out ./pkg/backends/neo/... 2>&1 > test.log
tail -3 test.log
```

## Documentation

Create a DevDiary within Obsidian (`Metahub/DevDiary`) after each major change in the form of `v0.1.2/YYYY-MM-DD-hhmm-change.md` and describe what has been done in that change.

Use the `Metahub/docs` directory to document specs, graph constructs and such and link them within the DevDiary.