## Purpose

Specifies how metahub-api resolves pull requests (Index, Manifest, Blob) through the Slot-centred graph model, including tag-to-Index resolution via POINTS_TO, manifest reachability via Slot edges, blob reachability with UPLOADED fallback, and the post-push compute pipeline.

## Requirements

### Requirement: handleIndex returns a fully-hydrated OCI Index with populated manifests array

The pull handler for OCI Index responses (`handleIndex`, renamed from `handleManifestList`) SHALL return a response body whose `manifests` JSON array is populated with one descriptor per `[:REFERENCES]` edge of the underlying `:Index` node, preserving descriptor order and platform overrides. The response body SHALL NOT contain a `"manifests":null` value.

The handler SHALL call `mag.Database.IndexGetByRequest(ctx, req)` which returns a fully-hydrated `types.Index` struct with the `Manifests` Go slice populated by the metalib Slot+REFERENCES walk. The handler SHALL serialise this struct to its OCI wire-format bytes via an `ToOCIBytes()` helper (or equivalent) that preserves descriptor order, platform hints, and annotations.

#### Scenario: Single-arch tag pull returns index with one descriptor
- **WHEN** a client pushes an amd64 manifest to `qnib/alpine:latest` under profile `generic`
- **AND** compute produces a 1-descriptor Index for the Slot
- **AND** a client sends `GET /v2/qnib/alpine/manifests/latest` with `Accept: application/vnd.oci.image.index.v1+json`
- **THEN** the response SHALL be HTTP 200 with `Content-Type: application/vnd.oci.image.index.v1+json`
- **AND** the response body SHALL parse as an OCI Index
- **AND** `response.manifests` SHALL be a non-null array containing exactly one element
- **AND** the element SHALL have `platform.architecture == "amd64"` and `platform.os == "linux"`

#### Scenario: Multi-arch tag pull returns index with two descriptors in order
- **WHEN** a client pushes amd64 then arm64 manifests to the same `qnib/alpine:latest` under profile `generic`
- **AND** compute produces a 2-descriptor Index for the Slot
- **AND** a client sends `GET /v2/qnib/alpine/manifests/latest` with `Accept: application/vnd.oci.image.index.v1+json`
- **THEN** the response body's `manifests` array SHALL contain exactly two elements
- **AND** the array order SHALL match the `order` property on the `[:REFERENCES]` edges (ascending)
- **AND** the two elements SHALL have distinct `digest` values
- **AND** both elements SHALL have `platform.os == "linux"` with `platform.architecture` in `{amd64, arm64}`

#### Scenario: Tag pull for non-existent Slot returns 404
- **WHEN** a client sends `GET /v2/qnib/alpine/manifests/latest` with profile `cpu#zen2`
- **AND** no Slot exists for `(qnib/alpine, latest, cpu#zen2)` because no pushed content matched zen2's rules
- **THEN** the response SHALL be HTTP 404 with an OCI error envelope containing `{"errors": [{"code": "MANIFEST_UNKNOWN", ...}]}`

#### Scenario: Digest pull serves the verbatim client-pushed index bytes
- **WHEN** a client pushes a pre-assembled OCI Index via `skopeo copy --multi-arch all`
- **AND** the client records the pushed digest D
- **AND** the same client later sends `GET /v2/<repo>/manifests/D`
- **THEN** the response SHALL be HTTP 200
- **AND** the response body SHALL be byte-identical to what the client originally pushed
- **AND** the `Docker-Content-Digest` response header SHALL equal D

### Requirement: handleManifest resolves single-manifest pulls via Slot reachability

The handler for single-manifest pulls SHALL resolve the manifest by walking the Slot reachability chain from `(:Slot {repo, profile})-[:POINTS_TO|HAS_PUSHED_INDEX]->(:Index)-[:REFERENCES]->(:Manifest)` or directly via `(:Slot)-[:HAS_PUSHED]->(:Manifest)` for manifests reachable by push history but not currently in any index.

Historical indexes (`HAS_HISTORY`) SHALL NOT grant manifest reachability for pulls.

#### Scenario: Manifest reachable via current POINTS_TO index returns 200
- **WHEN** a Slot has POINTS_TO to an Index that references Manifest M
- **AND** a client sends `GET /v2/<repo>/manifests/<M.digest>` with the Slot's profile
- **THEN** the response SHALL be HTTP 200 with the manifest bytes

#### Scenario: Manifest reachable via HAS_PUSHED (but not in any current index) returns 200
- **WHEN** a Manifest M has been pushed to a Slot via HAS_PUSHED
- **AND** no current Index (POINTS_TO or HAS_PUSHED_INDEX) references M
- **AND** a client sends `GET /v2/<repo>/manifests/<M.digest>` with the Slot's profile
- **THEN** the response SHALL be HTTP 200

#### Scenario: Manifest reachable only via HAS_HISTORY returns 404
- **WHEN** a Manifest is reachable only through an archived HAS_HISTORY Index
- **AND** the Manifest has no HAS_PUSHED edge from any current Slot
- **THEN** `GET /v2/<repo>/manifests/<digest>` SHALL return HTTP 404

### Requirement: handleBlob resolves blob pulls via reachability and UPLOADED fallback

The blob pull handler SHALL check blob reachability via the chain `(:Blob)<-[:HAS_CONFIG|HAS_LAYER]-(:Manifest)<-[:REFERENCES]-(:Index)<-[:POINTS_TO|HAS_PUSHED_INDEX]-(:Slot {repo, profile})`, falling back to `(:Profile {label})-[:UPLOADED {repo}]->(:Blob)` for pre-manifest upload windows. Either match SHALL grant access.

`HAS_HISTORY` edges SHALL NOT grant blob reachability.

#### Scenario: Blob reachable via current index path returns 200
- **WHEN** a Blob is a layer of a Manifest referenced by a POINTS_TO Index on the requesting Slot
- **THEN** `GET /v2/<repo>/blobs/<digest>` SHALL return HTTP 200 with the blob bytes

#### Scenario: Blob reachable only via UPLOADED attribution returns 200 (pre-manifest window)
- **WHEN** a blob upload completes but no Manifest has referenced it yet
- **AND** the uploading profile requests `GET /v2/<repo>/blobs/<digest>`
- **THEN** the response SHALL be HTTP 200 via the UPLOADED attribution fallback

#### Scenario: Blob reachable only via historical index returns 404
- **WHEN** a Blob is only reachable via a Manifest in a HAS_HISTORY archived Index
- **AND** no UPLOADED attribution exists for the requesting profile
- **THEN** `GET /v2/<repo>/blobs/<digest>` SHALL return HTTP 404

### Requirement: computeAfterManifestPush uses ComputeIndexForRepoTag under the hood

The `computeAfterManifestPush` helper method SHALL call `mag.ComputeIndexForRepoTag(ctx, repo, tag, pusherPrf)` (renamed from the previous `ComputeManifestlistsForRepoTag`). The helper's public signature does not change -- callers in `handleDockerManifestPut` and `handleOCIManifestPut` continue to invoke it identically.

#### Scenario: Docker v2 push path still invokes the rename-transparent helper
- **WHEN** `handleDockerManifestPut` reaches the compute step after a successful StoreManifestUpload
- **THEN** `computeAfterManifestPush` SHALL be invoked
- **AND** `computeAfterManifestPush` SHALL internally call `ComputeIndexForRepoTag`

#### Scenario: OCI v1 push path still invokes the rename-transparent helper
- **WHEN** `handleOCIManifestPut` reaches the compute step after a successful StoreManifestUpload
- **THEN** `computeAfterManifestPush` SHALL be invoked
- **AND** `computeAfterManifestPush` SHALL internally call `ComputeIndexForRepoTag`
