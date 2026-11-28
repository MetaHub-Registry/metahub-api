#!/usr/bin/env bash
# Smoke test for a running quickstart stack: push an OCI image and pull it back.
#
#   ./.quickstart.env is read when present
#   ./misc/quickstart/smoke.sh [base-url]
#
# Exits non-zero on the first unexpected status, so it doubles as the CI check
# for the quickstart.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

base="${1:-http://localhost:8080}"
repo="demo/hello"
tag="smoke"

if [ -f .quickstart.env ]; then
  set -a; . ./.quickstart.env; set +a
fi
: "${GODMODE_USER:?set GODMODE_USER (or run ./quickstart.sh first)}"
: "${GODMODE_PASSWORD:?set GODMODE_PASSWORD (or run ./quickstart.sh first)}"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

printf '{"architecture":"amd64","os":"linux","rootfs":{"type":"layers","diff_ids":[]}}' > "$work/config.json"
printf 'hello from the quickstart smoke test\n' > "$work/layer.txt"

sha() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }
size() { wc -c < "$1" | tr -d ' '; }

token="$(curl -sf -u "$GODMODE_USER:$GODMODE_PASSWORD" \
  "$base/auth?scope=repository:$repo:pull,push&service=${base#http://}" | sed -E 's/.*"token":"([^"]+)".*/\1/')"
[ -n "$token" ] || { echo "smoke: no token from /auth" >&2; exit 1; }

put_blob() { # file digest
  local file="$1" digest="$2" location
  location="$(curl -sf -X POST -H "Authorization: Bearer $token" \
    "$base/v2/$repo/blobs/uploads/" -D - -o /dev/null \
    | awk 'tolower($1)=="location:"{print $2}' | tr -d '\r')"
  case "$location" in /*) location="$base$location" ;; esac
  code="$(curl -s -o /dev/null -w '%{http_code}' -X PUT -H "Authorization: Bearer $token" \
    -H "Content-Type: application/octet-stream" --data-binary "@$file" \
    "$location?digest=sha256:$digest")"
  [ "$code" = "201" ] || { echo "smoke: blob $digest -> $code" >&2; exit 1; }
}

config_digest="$(sha "$work/config.json")"
layer_digest="$(sha "$work/layer.txt")"
put_blob "$work/config.json" "$config_digest"
put_blob "$work/layer.txt" "$layer_digest"

printf '{"schemaVersion":2,"mediaType":"application/vnd.oci.image.manifest.v1+json","config":{"mediaType":"application/vnd.oci.image.config.v1+json","digest":"sha256:%s","size":%s},"layers":[{"mediaType":"application/vnd.oci.image.layer.v1.tar","digest":"sha256:%s","size":%s}]}' \
  "$config_digest" "$(size "$work/config.json")" "$layer_digest" "$(size "$work/layer.txt")" > "$work/manifest.json"

code="$(curl -s -o /dev/null -w '%{http_code}' -X PUT -H "Authorization: Bearer $token" \
  -H "Content-Type: application/vnd.oci.image.manifest.v1+json" \
  --data-binary "@$work/manifest.json" "$base/v2/$repo/manifests/$tag")"
[ "$code" = "201" ] || { echo "smoke: manifest -> $code" >&2; exit 1; }

pulled="$(curl -sf -H "Authorization: Bearer $token" "$base/v2/$repo/blobs/sha256:$layer_digest")"
[ "$pulled" = "$(cat "$work/layer.txt")" ] || { echo "smoke: pulled content differs" >&2; exit 1; }

echo "smoke ok: pushed $repo:$tag, pulled the layer back"
