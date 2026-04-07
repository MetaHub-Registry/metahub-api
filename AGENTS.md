# Agent Instructions

## After Each Change

At the end of every change, run the following two scripts in order:

1. **Tests** — run the full test suite with coverage:
   ```
   ./test.sh
   ```

2. **SonarQube analysis** — submit coverage results to SonarQube:
   ```
   source .env
   ./misc/bin/sonar.sh
   ```
3. **SonarQube acceptance** - make sure the current branch within the SonarQube project is accepted.
   use the SonarQube MCP server

Both scripts must pass before a change is considered complete.

## Use OpenSpec in respecive directories.

The directory `metahub-api` is dependend on `metalib` (`~/go/src/github.com/metahub-registry/metahub-core`) which provides the libraries used for the heavy lifting and `metahub-ui` (`~/src/gitlab.com/qnib-frontend/metahub-ui`) for frontend work.

Each directory has its own `openspec` directory. So if you are working on n openspec change in either of the directories with implication to the other two, make sure you create a change within that repo as well and link them.