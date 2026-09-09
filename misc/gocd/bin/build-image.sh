#!/bin/bash
# Build + push ONE ARCH of the metahub-api PRODUCT image to Harbor.
#
# The pipeline fans out: two parallel Image jobs (ARCH=amd64 / ARCH=arm64),
# each pinned to an arch-matched agent (k8s-dind-amd64 / k8s-dind-arm64), push
# ${NAME}:${TAG}-${ARCH}; a downstream Manifest stage assembles them into a
# multi-arch manifest list via `docker manifest create`. This isolates
# per-arch failures, parallelizes the legs, and runs everything natively —
# no QEMU/binfmt, no buildx sidecar.
#
# Mirrors metahub-admin's build-image.sh, with the metahub-api specifics:
#   * builds Dockerfile.result (api server on the metalib base image passed as
#     METALIB_IMAGE; the Go build cross-compiles via zig cc inside the build),
#   * keeps build-api.sh's naming: tagged commits go to metahub/api:${GIT_TAG}-
#     <group>-${ARCH}, branch builds to metahub/api/<group>/<pipeline> — the
#     multi-arch manifest is assembled downstream under the plain tag.
set -e

if [[ -z "${ARCH}" ]]; then
   echo "ERROR: ARCH required (amd64|arm64) — the pipeline sets it per job"
   exit 1
fi
if [[ "${ARCH}" != "amd64" && "${ARCH}" != "arm64" ]]; then
   echo "ERROR: unsupported ARCH '${ARCH}' (amd64|arm64)"
   exit 1
fi

if [[ -f ~/gocd_func.sh ]]; then
   FUNC_SRC=~/gocd_func.sh
elif [[ -f /opt/qnib/gocd/src/gocd_func.sh ]]; then
   FUNC_SRC=/opt/qnib/gocd/src/gocd_func.sh
else
    echo "ERROR: No gocd_func.sh found"
    exit 1
fi
source ${FUNC_SRC}
dumpEnv

# strip any local `replace` directive, then re-point metalib at the directory
# the Docker build populates from METALIB_IMAGE (same two-step as build-api.sh):
# Dockerfile.result copies the workspace go.mod into WORKDIR /go/metahub-api and
# fills exp/metalib/ from the metalib image, so the replace must target that
# container path — otherwise `go mod tidy` tries to fetch the placeholder
# pseudo-version from the private gitlab repo and fails.
cat go.mod | awk '!/^replace/' > go.mod.tmp
mv go.mod.tmp go.mod
go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/exp/metalib

# metalib base image (FROM $METALIB_IMAGE) from the upstream metalib pipeline.
if [[ -f metalib_result/result.yaml ]]; then
   echo ">> found metalib_result/result.yaml"
   cat metalib_result/result.yaml
   METALIB_IMAGE=$(yq -r .image metalib_result/result.yaml)
   cat >> result.yaml << EOF
metalib:
   image: ${METALIB_IMAGE}
EOF
else
   echo "ERROR: no metalib_result/result.yaml — need METALIB_IMAGE for the build"
   exit 1
fi

if [[ -z "${HARBOR_REG}" || -z "${HARBOR_TOKEN}" ]]; then
   echo "ERROR: HARBOR_REG/HARBOR_TOKEN required to push the api image"
   exit 1
fi

NEW_IMAGE_NAME=${HARBOR_REG}/metahub/api
GIT_TAG=$(git describe --tags --exact-match 2>/dev/null || true)
if [ -n "$GIT_TAG" ]; then
   TAG="$(echo ${GIT_TAG}-${GO_PIPELINE_GROUP_NAME}|tr '[:upper:]' '[:lower:]')"
else
   TAG="$(echo ${GO_PIPELINE_LABEL}|tr '[:upper:]' '[:lower:]')"
   NEW_IMAGE_NAME=$(echo ${NEW_IMAGE_NAME}/${GO_PIPELINE_GROUP_NAME}/${GO_PIPELINE_NAME}|tr '[:upper:]' '[:lower:]')
fi
# Per-arch tag: the multi-arch manifest is assembled downstream (manifest-image.sh).
NEW_IMAGE=$(echo ${NEW_IMAGE_NAME}:${TAG}-${ARCH} | tr '[:upper:]' '[:lower:]')
echo "> NEW_IMAGE: ${NEW_IMAGE}"

echo "> Login to ${HARBOR_REG} as 'robot\$good'"
docker login ${HARBOR_REG} -u 'robot$good' -p "${HARBOR_TOKEN}"

set -x
# --- single-arch build on the arch-pinned agent ------------------------------
# Plain `docker build` is enough here: the job runs on an arch-matched agent
# (k8s-dind-amd64 / k8s-dind-arm64), so the build AND the final-stage RUNs
# (mkdir) execute natively — no QEMU/binfmt, no buildx sidecar. The Go build
# itself cross-compiles with zig cc ($BUILDPLATFORM pattern in the
# Dockerfile). The default (docker) buildkit driver still honors the
# Dockerfile's RUN --mount=type=cache mounts and emits a plain
# single-platform manifest — no attestation index — which is exactly what
# `docker manifest create` needs as a source downstream. The push goes through
# dockerd, which has harbor in INSECURE_REGISTRIES (plain HTTP), so no
# buildkitd registry config is needed either.
export DOCKER_BUILDKIT=1
docker build --platform linux/${ARCH} \
   -f Dockerfile.result \
   --build-arg=METALIB_IMAGE=${METALIB_IMAGE} \
   -t ${NEW_IMAGE} .
docker push ${NEW_IMAGE}
# .full is the per-arch image; the Manifest stage publishes the manifest-list
# ref (no suffix) that downstream consumers (Deploy/AwaitRollout) run.
gdh patch -f result.yaml \
   --add=metahub.image.reg=${HARBOR_REG} \
   --add=metahub.image.name=${NEW_IMAGE_NAME} \
   --add=metahub.image.tag=${TAG} \
   --add=metahub.image.arch=${ARCH} \
   --add=metahub.image.full=${NEW_IMAGE}
