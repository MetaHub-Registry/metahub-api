# Alpine layer fixtures

The layer tarballs used by this Bruno collection are **not committed**: they are
third-party Alpine binaries (~26 MB total) and do not belong in the source
repository.

The requests in `singlePostAmd64/` and `singlePostArm64/` expect:

```text
singlePostAmd64/data/alpine-amd64.tar
singlePostArm64/data/alpine-arm64.tar
```

## Recreate the fixtures locally

Requires Docker. Pull the same Alpine release for each architecture and extract
the uncompressed rootfs layer:

```bash
# amd64
docker pull --platform linux/amd64 alpine:3.21
docker save alpine:3.21 -o /tmp/alpine-amd64-image.tar
mkdir -p /tmp/alpine-amd64 && tar -xf /tmp/alpine-amd64-image.tar -C /tmp/alpine-amd64
# the layer path is listed in /tmp/alpine-amd64/manifest.json
cp /tmp/alpine-amd64/<layer-from-manifest>/layer.tar \
   misc/bruno/metahub-api/alpine/singlePostAmd64/data/alpine-amd64.tar

# arm64
docker pull --platform linux/arm64 alpine:3.21
docker save alpine:3.21 -o /tmp/alpine-arm64-image.tar
mkdir -p /tmp/alpine-arm64 && tar -xf /tmp/alpine-arm64-image.tar -C /tmp/alpine-arm64
cp /tmp/alpine-arm64/<layer-from-manifest>/layer.tar \
   misc/bruno/metahub-api/alpine/singlePostArm64/data/alpine-arm64.tar
```

## Update the pinned values

The layer digests and sizes in `folder.bru` identify the exact tarballs. If your
generated layer differs (different Alpine build date, different tar options),
update these variables:

```text
singlePostAmd64/folder.bru: alpineAmd64LayerDigest, alpineAmd64LayerSize
singlePostArm64/folder.bru: alpineArm64LayerDigest, alpineArm64LayerSize
```

Compute them with:

```bash
sha256sum misc/bruno/metahub-api/alpine/singlePostAmd64/data/alpine-amd64.tar
stat -c%s misc/bruno/metahub-api/alpine/singlePostAmd64/data/alpine-amd64.tar
```

The `.tar` files are git-ignored under `alpine/*/data/`.
