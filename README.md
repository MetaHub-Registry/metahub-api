<div align="center">

# MetaHub Registry

**Stop shipping the wrong binary to the right tag.**

A profile-aware OCI registry: the same tag can resolve to hardware-appropriate
content, without changing the client.

[Documentation](https://metahub-registry.org) ·
[Profiles](https://metahub-registry.org/profiles) ·
[Roadmap](ROADMAP.md) ·
[Contributing](CONTRIBUTING.md)

</div>

---

## The problem

One image tag has to serve an increasingly fragmented fleet: `amd64` and
`arm64`, different CPU microarchitectures, different GPU vendors, different
model runtimes. The usual answers are tag sprawl (`app:cuda12`, `app:rocm`,
`app:zen4`, `app:a100`, …) or per-node configuration that only the cluster
admin understands.

Both push the decision onto the person running the workload. The wrong choice
is discovered at runtime, as an illegal instruction, a missing driver, or a
container that starts and then cannot use the hardware.

## What MetaHub does

MetaHub sits in front of an OCI registry and resolves `repo:tag` against a
**profile** — a small set of facts about the client (CPU microarchitecture, GPU,
OS, runtime). The profile determines which manifest and which layers the client
receives.

- **Profile-aware resolution** — the same tag can return different, valid
  content per profile.
- **Manifest composition** — images are assembled from collections, profiles,
  and rules rather than duplicated per target.
- **Named manipulations** — layer selection, defaults, and variants are
  explicit, recorded operations, not silent rewrites.
- **Pull-through cache** — upstream registries are mirrored on demand, with the
  same profile logic applied.
- **Standard OCI distribution** — clients use `docker`, `containerd`, `oras`,
  or any conformant tool. No custom client.

## How it works

```text
  docker / containerd / oras
            │
            │  GET /v2/  ──►  401 (auth challenge)
            ▼
         /auth  ── login suffix "user/profile"  ──┐
            │      or IP → profile (optional      │
            │      inventory service)             │
            ▼                                     │
       metahub-api                                │
            │                                     │
            ├── profile ──► Slot {repo, tag, profile}
            │                   │
            │                   ├── manifest (+ named manipulations)
            │                   └── provenance annotations
            │
            ├── Neo4j (metadata: collections, profiles, rules, manifests)
            ├── MinIO / S3 (blobs)
            └── pull-through ──► upstream registry (optional)
```

The profile comes from one of three places:

1. **Explicit** — the registry login username carries a suffix:
   `docker login -u alice/zen2`.
2. **Automatic** — when the optional inventory service is configured,
   metahub-api asks it which profile belongs to the client IP.
3. **Fallback** — anonymous pulls resolve to `generic`.

## Integrity model

Profile-aware resolution means `repo:tag` is **not** immutable across profiles:
the same tag may resolve to different digests. This is intentional, and it is
documented rather than hidden.

- **Pull by digest** bypasses resolution and returns exactly that manifest. It
  is the strict-immutability mode for clients that need it.
- **Derived artefacts carry provenance annotations** naming the source digest
  and the manipulation set that produced them.
- **Registry-side signing of derived artefacts is not part of 1.0.** Publisher
  signatures over the original digest do not describe the transformed output.
  Signing is on the [roadmap](ROADMAP.md).

If you need reproducible, signed, profile-independent content, pin digests.

## Quickstart

A self-contained, offline quickstart ships with the 1.0 release. It starts the
registry, UI, Neo4j, and MinIO with one command, builds two tiny demo images
locally, and pulls the same tag under two profiles to show different content.
No internet, no identity provider, no credentials beyond generated defaults.

```bash
# available with 1.0
docker compose up
```

Until then, the development setup is described in
[CONTRIBUTING.md](CONTRIBUTING.md).

## Components

| Repository | Role |
|---|---|
| [`metahub-api`](https://github.com/metahub-registry/metahub-api) | Registry API: OCI distribution, authentication, profile resolution, pull-through |
| [`metahub-core`](https://github.com/metahub-registry/metahub-core) | Shared Go library: graph model, profiles, rules, manipulations, storage backends |
| [`metahub-ui`](https://github.com/metahub-registry/metahub-ui) | End-user web UI: collections, profiles, models, artefacts |
| [`metahub-admin`](https://github.com/metahub-registry/metahub-admin) | CLI: bootstrap, parse, repackage |
| [`metahub-admin-ui`](https://github.com/metahub-registry/metahub-admin-ui) | **Experimental** inventory and profile-resolution service (Go + SPA) |
| [`metahub-community-edition`](https://github.com/metahub-registry/metahub-community-edition) | Curated collections, profiles, and manipulations |
| [`metahub-docs`](https://github.com/metahub-registry/metahub-docs) | Documentation site |

> The `metalib` repository is renamed to `metahub-core` as part of the 1.0
> release. Module paths move to `github.com/metahub-registry/`.

## Project status

**Pre-1.0.** The 1.0 release is planned for **January 2027**.

Stable surfaces at 1.0: the OCI distribution endpoints, `/api/v1`, the profile
YAML schema, and the CLI commands. Experimental: model repackaging, the Waldur
billing adapter, inventory-based resolution, and alternative storage backends.

MetaHub makes **no outbound connections** except to upstream registries,
inventory, and identity providers you configure. There is no telemetry.

## Documentation

- [Quickstart](https://metahub-registry.org/quickstart) — first run, offline
- [Profiles and resolution](https://metahub-registry.org/profiles) — the model
- [Manipulations](https://metahub-registry.org/manipulations) — layer composition
- [Pull-through cache](https://metahub-registry.org/pull-through-cache)
- [API reference](https://metahub-registry.org/api) — `/api/v1`
- [Operations](https://metahub-registry.org/operations) — Neo4j, object store, upgrades

## Contributing

Contributions are welcome. Every commit needs a Developer Certificate of Origin
sign-off (`git commit -s`); there is no CLA. See
[CONTRIBUTING.md](CONTRIBUTING.md), [GOVERNANCE.md](GOVERNANCE.md), and the
[code of conduct](CODE_OF_CONDUCT.md).

Security issues: please follow [SECURITY.md](SECURITY.md) — do not open a public
issue.

## Licence

Source code and curated data: **Apache-2.0**. Documentation: **CC-BY-4.0**.

The name and logo are governed by [TRADEMARK.md](TRADEMARK.md); the source
licence grants no trademark rights.

MetaHub uses Neo4j Community Edition (GPLv3) as an external metadata store,
reached over the Bolt protocol. It is a separate service, not linked into
MetaHub's source; see the operations documentation for the licensing details.
