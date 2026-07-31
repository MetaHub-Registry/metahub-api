# manifest-cpu-target-display Specification

## Purpose
TBD - created by archiving change surface-cpu-target-ui. Update Purpose after archive.
## Requirements
### Requirement: Manifest DTO exposes a structured CPU target
The collection-detail manifest DTO SHALL expose a `CPUTarget` object
parsed from the manifest's `org.supercontainers.hardware.cpu.target*`
annotations. The object SHALL carry the target kind (`platform`,
`microarch`, or `generic`) and, when present, the microarch name and/or
generic family.

#### Scenario: Microarch manifest
- **WHEN** a manifest's annotations include
  `org.supercontainers.hardware.cpu.target=microarch` and
  `org.supercontainers.hardware.cpu.target.microarch=zen2`
- **THEN** the manifest DTO's `CPUTarget` SHALL report kind `microarch`
  and microarch `zen2`

#### Scenario: Generic family manifest
- **WHEN** a manifest's annotations include
  `org.supercontainers.hardware.cpu.target=generic` and
  `org.supercontainers.hardware.cpu.target.generic=x86_64_v4`
- **THEN** the DTO's `CPUTarget` SHALL report kind `generic` and
  generic `x86_64_v4`

### Requirement: Absent CPU-target annotations yield an empty target
The DTO SHALL emit an empty (JSON-omitted) `CPUTarget` for a manifest
that carries no `org.supercontainers.hardware.cpu.target*` annotation,
leaving the rest of the manifest DTO unchanged.

#### Scenario: Plain manifest
- **WHEN** a manifest has no CPU-target annotations
- **THEN** its DTO SHALL omit `CPUTarget` and all other fields SHALL be
  identical to pre-change output

### Requirement: CPU-target surfacing is additive
Adding the CPU-target field SHALL NOT change the existing
`Annotations` map or the `Platform.Features` list derived by
`extractFeatures`.

#### Scenario: Features untouched
- **WHEN** a manifest carries both a `cpu:foo=true` feature annotation
  and a `…cpu.target.microarch=zen2` annotation
- **THEN** `Platform.Features` SHALL still contain `cpu:foo` and the
  new `CPUTarget` SHALL independently report `microarch=zen2`

