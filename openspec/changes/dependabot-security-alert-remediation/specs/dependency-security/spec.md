## ADDED Requirements

### Requirement: Dependabot alerts are enabled on every product repository

Every repository in `MetaHub-Registry` that ships a dependency manifest SHALL
have Dependabot alerts enabled. The setting SHALL be verified through the API
(`GET /repos/{owner}/{repo}/vulnerability-alerts` returning `204`), not inferred
from the Security tab.

#### Scenario: A new repository is added to the organisation

- **WHEN** a repository with a `go.mod`, `package.json`, or `requirements.txt`
  joins the organisation
- **THEN** enabling Dependabot alerts SHALL be part of its bootstrap checklist
- **AND** the weekly re-check SHALL flag it while alerts are still disabled

### Requirement: No open critical or high alerts on a default branch

The default branch of every product repository SHALL have zero open Dependabot
alerts of severity `critical` or `high`. The check SHALL be performed with
pagination (`per_page=100`); the default page size of 30 hides alerts and
understates the count.

#### Scenario: A release is cut

- **WHEN** a release branch is promoted to `main`
- **THEN** the open-alert query SHALL return no `critical` or `high` entries
- **AND** any remaining `medium`/`low` entries SHALL be listed in the release
  notes as accepted

#### Scenario: A new critical advisory is published

- **WHEN** an advisory arrives after a release
- **THEN** it SHALL either be remediated in a follow-up change or carry a
  dated acceptance record before the next release

### Requirement: Remediation targets the root module, not the individual alert

A module that produces several advisories SHALL be bumped once to a version
that satisfies all of them. Per-alert dismissals SHALL NOT be used to reach the
zero-critical bar while a patched version exists.

#### Scenario: One module, many advisories

- **WHEN** `golang.org/x/crypto` accounts for eight critical advisories
- **THEN** the fix SHALL be a single `go.mod` bump to a version at or above the
  highest alert's patched version
- **AND** the commit message SHALL name the module and the advisory family, not
  enumerate the alerts

### Requirement: Advisories without an upstream fix carry a dated acceptance

Where no patched release exists, the repository SHALL record an explicit
acceptance instead of a silent dismissal: advisory identifier, the reason the
code path is not reachable (or the mitigation), and a date to revisit. The
record SHALL live in the repository's `CONTEXT.md`.

#### Scenario: Advisory with no fix

- **WHEN** an alert reports `no fix` in `first_patched_version`
- **THEN** the repository's `CONTEXT.md` SHALL contain a dated section naming
  the advisory and the reachability argument
- **AND** the alert MAY be dismissed only after that record exists

### Requirement: Dependency overrides are declared and traceable

A transitive dependency lift SHALL be expressed as an `overrides` entry
(npm) or a `replace`/explicit require (Go) in the manifest, with the advisory
identifier in the commit message. Removing every direct dependency to avoid
one transitive alert is not an acceptable substitute.

#### Scenario: npm transitive advisory

- **WHEN** a dev-only transitive package is below its patched version
- **THEN** the fix SHALL be an `overrides` entry pinning at or above the
  patched version
- **AND** the lockfile SHALL resolve one instance of the package

### Requirement: A version bump refreshes the third-party notices in the same change

Any merged dependency version change SHALL be accompanied by an updated
`THIRD_PARTY_NOTICES.md`, generated with the platform-pinned toolchain, so the
notices do not drift from the lock graph.

#### Scenario: A module is bumped

- **WHEN** a bump changes a version that appears in `THIRD_PARTY_NOTICES.md`
- **THEN** the notices SHALL be regenerated in the same commit
- **AND** the `licenses` CI job SHALL pass without further edits

### Requirement: Alert state is re-checked on a fixed cadence

Alert state SHALL be re-checked at least weekly — by a scheduled workflow or by
an explicit release-checklist line — so that a new advisory is visible before
it reaches a release.

#### Scenario: Weekly re-check runs

- **WHEN** the weekly check executes
- **THEN** it SHALL report the open-alert count per repository and severity
- **AND** any repository with alerts disabled SHALL be reported as a failure,
  not skipped
