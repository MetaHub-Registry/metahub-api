# manifest-detail-expanded Specification

## Purpose
TBD - created by archiving change manifest-expand-and-manipulations-api. Update Purpose after archive.
## Requirements
### Requirement: Collection detail carries per-manifest config and layer descriptors
The `POST /api/v1/collection` response SHALL include, for every manifest
entry, an optional `config` object with `digest`, `media_type` and `size`
fields, plus an optional `layers` array whose entries carry the same
shape.

#### Scenario: Multi-layer manifest
- **WHEN** a client requests a collection whose manifest has 1 config
  blob and 3 layer blobs
- **THEN** the response SHALL contain a `config` object with the blob's
  digest/media_type/size and a `layers` array of length 3 in layer order

#### Scenario: Manifest without layers (e.g. ORAS artifact)
- **WHEN** a manifest has no layer blobs attached
- **THEN** the response MAY omit the `layers` field or return an empty
  array; clients SHALL accept either form

#### Scenario: Manifest without config
- **WHEN** the config blob could not be loaded
- **THEN** the `config` field SHALL be absent from the response

### Requirement: UIDescriptor shape is shared
The system SHALL use a single `UIDescriptor` shape (`digest`, `media_type`,
`size`) for both the config field and each layer entry so UI consumers
can reuse a single renderer.

#### Scenario: Same shape for config and layers
- **WHEN** the client JSON-decodes the response
- **THEN** `manifests[].config` and each element of `manifests[].layers`
  SHALL have identical field sets

