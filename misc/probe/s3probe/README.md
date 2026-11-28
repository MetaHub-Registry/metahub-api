# s3probe

Exercises every S3 operation the registry uses against a live object store, so a
backend swap is a test rather than a claim.

The registry touches storage through nine calls and nothing else — no bucket
notifications, object lock, versioning, lifecycle rules, tagging or
`SelectObjectContent`:

| operation | registry uses it for |
|---|---|
| `BucketExists` | startup check |
| `MakeBucket` | creating `blobs` / `manifests` |
| `PutObject` | layer blobs, manifests, multipart layer uploads |
| `StatObject` | existence and size checks |
| `GetObject` | serving blobs and manifests |
| `PresignedGetObject` | handing out time-limited URLs |
| `ListObjects` | listing a digest prefix |
| `RemoveObject` | single-object cleanup |
| `RemoveObjects` | bulk cleanup (garbage collection) |

## Run it

```bash
set -a; . misc/config/compose.env; set +a
go run ./misc/probe/s3probe -endpoint localhost:9000
```

Credentials default to `$MINIO_ROOT_USER` / `$MINIO_ROOT_PASSWORD`, which the
compose file maps to the backend's own variables (`RUSTFS_ACCESS_KEY` /
`RUSTFS_SECRET_KEY` for RustFS).

It creates its own bucket (`-bucket`, default `s3probe`), writes three objects
under `probe/`, verifies them — including a 20 MiB multipart upload and a
presigned GET — then removes them. It never writes to the registry's buckets.

Exit status is non-zero when any check fails; each check is printed with its
error. Adjust `-multipart-mib`, `-part-mib`, `-region` or `-secure` for other
stores.

## When to extend it

If a feature starts using an S3 call that is not in the table above, add it here
and to the backend matrix in `metahub-docs` in the same change. The probe is the
definition of "supported backend", so it must not lag behind the code.
