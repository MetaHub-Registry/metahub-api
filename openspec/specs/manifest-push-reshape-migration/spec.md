# manifest-push-reshape-migration Specification

## Purpose
TBD - created by archiving change deprecate-manifest-wrapper. Update Purpose after archive.
## Requirements
### Requirement: Push handler reshapes via the Slot/leaf entrypoint without a wrapper fetch

The Docker/OCI manifest push handler (`handleDockerManifestPut` in `pkg/api/v1/handlers/oci_manifest.go`) SHALL trigger per-profile OCI reshaping by passing the content-leaf `types.Manifest` returned by `StoreManifestUpload` into metalib's Slot/leaf reshape entrypoint (`ComputeManifestForLeaf`). It SHALL NOT call `ManifestWrapperGetByLabel` nor pass a `types.ManifestWrapper` to the reshape pipeline.

#### Scenario: Docker manifest push reshapes without wrapper lookup
- **WHEN** a Docker v2 manifest is pushed and stored, yielding a content-leaf `:Manifest`
- **THEN** the handler SHALL invoke the Slot/leaf reshape entrypoint with the content-leaf
- **AND** SHALL NOT invoke `ManifestWrapperGetByLabel`
- **AND** the resulting per-profile OCI v1 manifests SHALL match those produced before the migration

#### Scenario: Reshape failure handling is preserved
- **WHEN** the Slot/leaf reshape entrypoint returns an error during a push
- **THEN** the handler SHALL surface the same DENIED/error response semantics it surfaced for the previous wrapper-based reshape

### Requirement: metahub-api carries no ManifestWrapper references

After migration, metahub-api production code SHALL contain no references to `ManifestWrapper` (type, methods, or the `:ManifestWrapper` label), and SHALL depend on a metalib version that exposes the Slot/leaf reshape entrypoint.

#### Scenario: No wrapper symbols remain
- **WHEN** the repository is searched for `ManifestWrapper` in non-test production code
- **THEN** there SHALL be zero matches
- **AND** the module SHALL build and its test suite SHALL pass

