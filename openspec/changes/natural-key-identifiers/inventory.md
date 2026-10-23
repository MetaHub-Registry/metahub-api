# Inventory — storage IDs in public payloads (metahub-api)

Scope: the API surface and the Go types that are serialised to clients. Neo4j
element IDs are the primary target; SQLite rowids have the same failure mode
and were handled in the same change.

## Response leaks (all removed)

| Endpoint | DTO | Field(s) | Natural key |
|---|---|---|---|
| `POST /api/v1/collections` | `Next` | `collection_id`, `tag_id` | `repo` + `tag` |
| `GET /api/v1/profiles/{label}` | `ProfileDetailResponse` | `id` | `label` |
| `GET /api/v1/profiles/{label}` | `ProfileRuleResp` | `id` | rule tuple |
| `GET /api/v1/profiles/{label}` | `UIManipulation` | `id` | manipulation `label` |
| `GET /api/v1/oras/artifact` | `types.OrasArtifact` | `id` | `repo` + `digest` |
| `GET /api/v1/protected/user` | `www.User` | `id` | `username` |
| `GET /api/v1/protected/user/secret` | `www.Secret` | `id` | `name` |

## Request leaks (all removed)

| Endpoint | Payload | Field(s) | Replacement |
|---|---|---|---|
| `POST /api/v1/collection` | `repoGetOnePayload` | `collection_id`, `tag_id` | `repo_name` + `tag_name` |
| `GET /api/v1/oras/artifact` | query | `id` | `repo` + `digest` / `artifact_type` |

## Second hops

| Handler | Before | After |
|---|---|---|
| `repos.go` `hydrateManifestLayers` | `GetLayersForManifestID(ctx, id)` | `GetLayersForManifestDigest(ctx, digest)` |
| `profiles.go` `ProfileGetOne` | `ManipulationsWithConfigByProfileID(ctx, id)` | `ManipulationsWithConfigByProfileLabel(ctx, label)` |
| `oras.go` `OrasGetOne` | `OrasArtifactGetByID(ctx, id)` | `OrasArtifactGetByDigest` / `OrasArtifactGetByName` |

Internal types (`Manifest.ID`, `Config.ID`, `OrasArtifact.ID`,
`Manipulation.ID`, …) stay internal and are not serialised. `id(n)` remains an
internal Cypher detail; no exported lookup reached from the public API takes an
element ID.
