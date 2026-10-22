# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project uses
semantic versioning once it reaches 1.0.

## [Unreleased]

### Changed — BREAKING (pre-1.0)

Public payloads and request bodies no longer carry storage element IDs. Clients
must address a resource by its natural key:

| Endpoint | Removed | Use instead |
|---|---|---|
| `POST /api/v1/collections` | `next[].collection_id`, `next[].tag_id` | `next[].repo` + `next[].tag` |
| `POST /api/v1/collection` | request `collection_id`, `tag_id` | `repo_name` + `tag_name` |
| `GET /api/v1/profiles/{label}` | `id` on the profile, rules and manipulations | `label`, the rule tuple, the manipulation `label` |
| `GET /api/v1/oras/artifact` | `?id=` and the response `id` | `repo` + `artifact_type` (or `digest`) |

There is no dual-format window: the project is pre-1.0 and the repositories
have no public clients yet.
