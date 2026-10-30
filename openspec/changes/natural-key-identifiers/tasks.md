# Tasks — natural-key identifiers (metahub-api)

## 1. Collections

- [x] 1.1 Remove `collection_id` / `tag_id` from `Next` and
      `repoGetOnePayload`; resolve by `repo_name` + `tag_name` only.
- [x] 1.2 Reject an ID-only `POST /api/v1/collection` with a clear 400.
- [x] 1.3 Update the listing and request-validation tests.

## 2. Profiles

- [x] 2.1 Remove `id` from `ProfileDetailResponse`, `ProfileRuleResp` and
      `UIManipulation`; key rules by the rule tuple and manipulations by
      `label`.
- [x] 2.2 Hydrate manipulations through
      `ManipulationsWithConfigByProfileLabel(ctx, label)`.

## 3. ORAS

- [x] 3.1 Remove `?id=`; resolve by `repo` + `digest`, keep `repo` +
      `artifact_type` for tree navigation.
- [x] 3.2 Drop `id` from the returned artifact (the core type carries
      `json:"-"`).

## 4. Core pin

- [x] 4.1 Pin `metahub-core` at the session commit
      (`v0.3.11-0.20261023073118-725938dd0b1e`) in `go.mod`/`go.sum`.
      Replaced by the released tag `v0.3.12` (current `go.mod`).
- [x] 4.2 Switch `hydrateManifestLayers` to
      `GetLayersForManifestDigest(ctx, digest)`.
- [x] 4.3 Switch the profile detail hop to the label lookup.

## 5. SQL-backed resources

- [x] 5.1 Make `www.User.ID` and `www.Secret.ID` `json:"-"`.
- [x] 5.2 Address secrets by `name` and the user by `username`; remove the
      `/users/{id}` routes.
- [x] 5.3 Replace the secret delete route with
      `DELETE /user/secret/{name}`.
- [x] 5.4 Update the handler and router tests.

## 6. Regression and documentation

- [x] 6.1 Add a response-shape test that walks the affected endpoints and
      fails on any `id` or `*_id` key.
- [x] 6.2 Add the restart round-trip test (collection by `repo`+`tag`,
      manifest by `digest`, payload equal apart from time fields).
- [x] 6.3 Record the breaking change in `CHANGELOG.md`.
- [x] 6.4 Run the full suites and the fast lane; both green.
