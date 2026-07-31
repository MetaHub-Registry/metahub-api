# profile-detail-manipulations Specification

## Purpose
TBD - created by archiving change manifest-expand-and-manipulations-api. Update Purpose after archive.
## Requirements
### Requirement: Profile detail lists linked manipulations
The `GET /api/v1/profiles/{label}` response SHALL include a
`manipulations` array containing every manipulation linked to the profile
via `[:USES]` edges, each entry carrying the manipulation's label plus
its `default` block and `variants` list hydrated from the graph.

#### Scenario: Profile with two manipulations
- **WHEN** a profile has `[:USES]` relationships to two manipulations and
  both carry default + variant configs
- **THEN** the response SHALL contain a `manipulations` array of length 2,
  each entry with `label`, `default`, and `variants` populated

#### Scenario: Profile with a legacy-form manipulation
- **WHEN** a profile has `[:USES]` relationship to a manipulation whose
  YAML used the legacy flat `layers: [{image, rule}]` form
- **THEN** the response SHALL still populate the `layers` field on that
  manipulation (falling back to the old slice when `default` and
  `variants` are empty)

#### Scenario: Profile with no manipulations
- **WHEN** a profile has no linked manipulations
- **THEN** the response SHALL contain `manipulations: []` (an empty array,
  not `null`)

### Requirement: UIManipuCommand carries op + value
Each `entrypoint` or `cmd` field on a manipulation config SHALL surface
as a JSON object with `op` (one of `replace`, `prepend`, `append`) and a
`value` array of strings.

#### Scenario: Replace entrypoint present
- **WHEN** a manipulation's default config has an entrypoint with
  `op=replace, value=["/bin/entry"]`
- **THEN** the response SHALL include
  `manipulations[].default.entrypoint = { "op": "replace", "value": ["/bin/entry"] }`

