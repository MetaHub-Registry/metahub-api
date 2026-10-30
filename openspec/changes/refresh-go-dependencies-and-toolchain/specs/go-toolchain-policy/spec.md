## ADDED Requirements

### Requirement: The Go floor is declared in go.mod and matches CI

Every Go module SHALL declare its minimum Go version in the `go` directive, and
the major.minor of that directive SHALL equal the major.minor of the toolchain
the repository's fast lane runs. A module SHALL NOT add a `toolchain` directive;
the CI workflow is the single place that pins the patch level.

#### Scenario: Directive lags the CI toolchain

- **WHEN** `go.mod` declares `go 1.24.0` and the fast lane pins `1.26.x`
- **THEN** the fast lane fails and names both values

#### Scenario: Directive and CI agree

- **WHEN** `go.mod` declares `go 1.26.0` and the fast lane pins `1.26.x`
- **THEN** the check passes

#### Scenario: Toolchain directive added

- **WHEN** `go.mod` contains a `toolchain` directive
- **THEN** the check fails and points at the CI pin as the single source

### Requirement: Dependency refresh lands as one graph per repository

A dependency update that changes the module graph SHALL be applied as one
change per repository: `go.mod`, `go.sum`, the mechanical source fixes and the
regenerated third-party notices land together. Overlapping automated update
branches SHALL be superseded rather than merged in sequence.

#### Scenario: Automated branches overlap

- **WHEN** several update branches rewrite the same `go.mod` and `go.sum` hunks
- **THEN** one consolidated change replaces them and those branches are closed

#### Scenario: Graph changes without notices

- **WHEN** a module graph changes and `THIRD_PARTY_NOTICES.md` is not regenerated
- **THEN** the licence job fails

### Requirement: Static-analysis fallout is fixed in the same change

When raising the Go floor enables a new `go vet` analysis, every finding SHALL
be fixed in the same change. The lane SHALL NOT suppress the analysis and SHALL
NOT raise the floor with findings outstanding.

#### Scenario: Floor raised above the analysis threshold

- **WHEN** the `go` directive is raised above `1.23`
- **THEN** `go vet ./...` passes for the module

#### Scenario: Non-constant format string

- **WHEN** a variable is passed as the format argument of a printf-like helper
- **THEN** a constant format such as `"%s"` is supplied instead of disabling the check

### Requirement: Consumers pin a refreshed metahub-core commit

`metahub-api` and `metahub-admin` SHALL pin the `metahub-core` pseudo-version
produced by the final commit of the refreshed `metahub-core` branch, and SHALL
take that pin only after the branch is final.

#### Scenario: Pin taken after the branch is final

- **WHEN** the `metahub-core` branch is final
- **THEN** the consumers' pseudo-version resolves to its head commit

#### Scenario: Pin taken too early

- **WHEN** the `metahub-core` branch changes after the consumers were pinned
- **THEN** the pin is bumped again in the same change and not left stale

### Requirement: The local workspace tracks the module floor

The session `go.work` SHALL declare a Go version at least as high as the highest
`go` directive among the modules it uses.

#### Scenario: Workspace below a module

- **WHEN** `go.work` declares `go 1.25.0` and a used module declares `go 1.26.0`
- **THEN** `go.work` is raised before the suite is run
