# Agent Instructions

## After Each Change

At the end of every change, run the following two scripts in order:

1. **Tests** — run the full test suite with coverage:
   ```
   ./test.sh
   ```

2. **SonarQube analysis** — submit coverage results to SonarQube:
   ```
   ./misc/bin/sonar.sh
   ```
3. **SonarQube acceptance** - make sure the current branch within the SonarQube project is accepted.
   use the SonarQube MCP server

Both scripts must pass before a change is considered complete.
