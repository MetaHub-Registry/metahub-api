#!/usr/bin/env bash
# Asserts that every healthcheck probes the endpoint its image actually serves.
#
#   ./misc/bin/check-compose-healthcheck.sh [compose-file]
#
# A backend whose health path differs from the previous one (RustFS serves
# /health, MinIO served /minio/health/live) breaks `compose up --wait` silently:
# the container runs, the gate never turns green, and the quickstart looks
# broken. This catches the mismatch at review time.
set -euo pipefail

exec python3 - "$@" <<'PY'
import json
import subprocess
import sys

compose = sys.argv[1] if len(sys.argv) > 1 else "docker-compose-data.yaml"

# health path each known backend serves
EXPECTED = {
    "rustfs": "/health",
    "minio": "/minio/health/live",
}

proc = subprocess.run(
    ["docker", "compose", "-f", compose, "config", "--no-interpolate", "--format", "json"],
    capture_output=True, text=True,
)
if proc.returncode != 0:
    sys.exit(f"check-compose-healthcheck: docker compose config failed:\n{proc.stderr}")

services = (json.loads(proc.stdout).get("services") or {})
failures = []
checked = 0

for name, svc in services.items():
    test = ((svc.get("healthcheck") or {}).get("test") or [])
    urls = [t for t in test if isinstance(t, str) and t.startswith(("http://", "https://"))]
    if not urls:
        continue
    image = svc.get("image", "")
    backend = next((k for k in EXPECTED if k in image), None)
    if backend is None:
        print(f"warn  {name}: unknown backend image '{image}', healthcheck not verified")
        continue
    checked += 1
    want = EXPECTED[backend]
    if urls[0].rstrip("/").endswith(want):
        print(f"ok    {name}: {backend} serves {want}")
    else:
        failures.append(f"{name}: image is {image} (serves {want}) but the healthcheck probes {urls[0]}")

for f in failures:
    print(f"FAIL  {f}", file=sys.stderr)
if not failures:
    print(f"{checked} healthcheck(s) verified")
sys.exit(1 if failures else 0)
PY
