## Why

Public responses and request bodies expose database-assigned IDs: Neo4j element
IDs (`collection_id`, `tag_id`, profile/rule/manipulation `id`, ORAS artifact
`id`) and SQLite rowids (`www.User.ID`, `www.Secret.ID`). They are storage
implementation details: a rebuilt graph assigns different values, and a client
that stores one breaks after a restore.

The repositories are private and pre-1.0, so this is the last cheap moment to
remove them. A quickstart written against numeric IDs would need a rewrite
after this change.

## What Changes

- **Responses** stop emitting storage IDs. Collections are identified by
  `repo` + `tag`; profiles, rules and manipulations by `label` and the rule
  tuple; ORAS artifacts by `repo` + `digest`.
- **Requests** stop accepting storage IDs. `POST /api/v1/collection` requires
  `repo_name` + `tag_name`; `GET /api/v1/oras/artifact?id=` is removed.
- **SQL IDs**: `www.User.ID` and `www.Secret.ID` are hidden from JSON;
  secrets are addressed by `name`, users by `username`; the redundant
  `/users/{id}` routes are removed and `DELETE /user/secret/{name}` replaces
  the ID route.
- **metahub-core** gains natural-key lookups; this repo pins the session commit
  as a pseudo-version and switches its second hops to those methods. The
  release lane replaces the pseudo-version with the tagged release.
- A restart round-trip test proves the same resource resolves by natural key
  after an API restart.

## Impact

- Breaking pre-1.0 API change. `metahub-ui` is updated in its own change;
  `metahub-admin` only pins the same core version and does not call the new
  lookups; `metahub-admin-ui` serves its own API and is unaffected.
- No database migration: the IDs were never part of the contract.
- `CHANGELOG.md` records the removed fields and their replacements.
