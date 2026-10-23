# Design — natural-key identifiers (metahub-api)

The before/after mapping is in [`inventory.md`](inventory.md).

## Decisions

### D1. The natural key is the public contract

A response field is either a natural key or a derived value. A storage element
ID (`id(n)`, SQL rowid) never appears in a public payload. The mapping:

| Resource | Natural key |
|---|---|
| Collection (repo+tag) | `repo` + `tag` |
| Profile | `label` |
| Profile rule | (`type`, `op`, `key`, `value`) scoped to the profile |
| Manipulation | `label` scoped to the profile |
| Manifest / blob | `digest` |
| ORAS artifact | `repo` + `digest` (tree: `repo` + `artifact_type`) |
| User / secret (SQL) | `username` / secret `name` |

### D2. No dual format

The repositories are private and pre-1.0. Requests that carry only
`collection_id`/`tag_id` are rejected; responses drop the fields outright.
Keeping both forms would mean two code paths and a migration window for clients
that do not exist.

### D3. The second hop uses the core's natural-key methods

`hydration` calls `GetLayersForManifestDigest`, and the profile detail calls
`ManipulationsWithConfigByProfileLabel`. The core resolves the key and may use
`id(n)` internally. No element ID crosses the handler boundary.

### D4. The core pin is a pseudo-version

`go.mod` pins the session `metahub-core` commit
(`v0.3.11-0.20261023073118-725938dd0b1e`). The hosted fast lane fetches the
private module with the read-only `METAHUB_CORE_RO_TOKEN`; fork PRs skip the
module-dependent jobs until the visibility flip. The release lane replaces the
pseudo-version with the tag.

### D5. SQL IDs follow the same rule

`www.User.ID` and `www.Secret.ID` are `json:"-"`; the routes use `username`
and `name`. There is no compatibility shim for the old `/users/{id}` routes.

### D6. Restart is the regression test

A write → restart → read cycle is the test that proves the natural keys are
sufficient. It compares the payload apart from time-dependent fields.
