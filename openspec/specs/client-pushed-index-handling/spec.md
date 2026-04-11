## Purpose

Specifies how metahub-api handles client-pushed OCI Index manifests via the `PUT /v2/<repo>/manifests/<reference>` endpoint, including dual-storage semantics (verbatim minio storage + graph merge), member-manifest validation, OCI error mapping, and platform-override preservation.

## Requirements

### Requirement: handleIndexPut stores client-pushed indexes with dual-storage semantics

The `PUT /v2/<repo>/manifests/<reference>` handler, when the request body has mediaType `application/vnd.oci.image.index.v1+json` or `application/vnd.docker.distribution.manifest.list.v2+json`, SHALL route to `handleIndexPut` (renamed from `handleOCIManifestListPut`). The handler SHALL call `mag.StoreIndexUpload(ctx, req, buf)` which implements the full Option α dual-storage semantics:

1. Verbatim storage of the client's bytes in minio under the client-computed digest.
2. MERGE of an `:Index {digest, origin: "client-pushed"}` node.
3. Validation that every member Manifest digest exists in the graph.
4. MERGE of `[:REFERENCES]` edges from the Index to each member Manifest with `order` and `platform` properties.
5. Creation of a `[:HAS_PUSHED_INDEX]` edge on the pusher's Slot.
6. Invocation of `ComputeIndexForRepoTag` for per-profile recompute.

The handler SHALL translate metalib errors to OCI error codes:

| Metalib error | HTTP status | OCI error code |
|---------------|-------------|----------------|
| `ErrManifestBlobUnknown` | 400 | `MANIFEST_BLOB_UNKNOWN` |
| `ErrIndexInvalid` | 400 | `MANIFEST_INVALID` |
| (other) | 500 | `INTERNAL_ERROR` |

On success, the handler SHALL return HTTP 201 with `Location`, `Docker-Content-Digest`, and `Content-Length: 0` headers per the OCI distribution spec.

#### Scenario: Client push with all member manifests present succeeds
- **WHEN** a client first pushes an amd64 Manifest M1 and an arm64 Manifest M2
- **AND** the client then pushes an Index body referencing M1 and M2 by digest
- **THEN** the response SHALL be HTTP 201
- **AND** the `Docker-Content-Digest` header SHALL equal the computed digest of the pushed Index bytes
- **AND** an `:Index {origin: "client-pushed"}` node SHALL exist with two `[:REFERENCES]` edges

#### Scenario: Client push with missing member manifest returns MANIFEST_BLOB_UNKNOWN
- **WHEN** a client pushes an Index referencing a Manifest digest that does not exist in the graph
- **THEN** the response SHALL be HTTP 400
- **AND** the response body SHALL contain an OCI error envelope with `code: "MANIFEST_BLOB_UNKNOWN"`
- **AND** no `:Index` node SHALL be created

#### Scenario: Client push triggers ComputeIndexForRepoTag
- **WHEN** a client push succeeds
- **THEN** `ComputeIndexForRepoTag` SHALL be invoked exactly once before the handler returns the 201 response
- **AND** every Profile in the graph whose rules match at least one member Manifest SHALL have its Slot's POINTS_TO recomputed

#### Scenario: Client can verify the push by HEAD-ing the digest
- **WHEN** a client successfully pushes an Index with computed digest D
- **AND** the client immediately sends `HEAD /v2/<repo>/manifests/D` under the same profile
- **THEN** the response SHALL be HTTP 200
- **AND** the `Docker-Content-Digest` header SHALL equal D
- **AND** the `Content-Type` header SHALL match the pushed index's mediaType

### Requirement: Client-pushed index preserves descriptor order and platform overrides

When a client pushes an Index where a descriptor's `platform` field differs from the referenced Manifest's canonical platform (e.g. a custom `os.version` hint), the handler (via metalib's `StoreIndexUpload`) SHALL preserve the client's platform override on the `[:REFERENCES]` edge. Subsequent tag pulls through this client-pushed Index SHALL return the client's original platform hints.

#### Scenario: Platform override survives round-trip
- **WHEN** a client pushes an Index where the amd64 descriptor has `platform.os.version = "5.15"` but the referenced Manifest's canonical platform has no os.version
- **AND** the client later pulls the Index by its digest
- **THEN** the returned body's amd64 descriptor SHALL include `platform.os.version: "5.15"`

### Requirement: handleIndexPut error mapping lives in a shared oci_errors helper

The metahub-api handler package SHALL include an `oci_errors.go` helper file with a single `writeOCIError(w, err)` function that centralises the mapping from metalib sentinel errors to OCI error codes + HTTP status. Every push handler (`handleDockerManifestPut`, `handleOCIManifestPut`, `handleIndexPut`) SHALL use this helper instead of constructing error envelopes inline.

#### Scenario: Shared helper is used by every push handler
- **WHEN** a push handler encounters a metalib error
- **THEN** the handler SHALL call `h.writeOCIError(w, err)` to produce the response
- **AND** SHALL NOT construct the OCI error envelope inline

#### Scenario: Unknown errors map to INTERNAL_ERROR
- **WHEN** a metalib call returns an error that is not one of the known sentinels
- **THEN** `writeOCIError` SHALL return HTTP 500 with `code: "INTERNAL_ERROR"`
- **AND** the original error message SHALL be included in the envelope's `message` field
