#!/bin/bash
set -e

if [[ -f ~/gocd_func.sh ]];then
   FUNC_SRC=~/gocd_func.sh
elif [[ -f /opt/qnib/gocd/src/gocd_func.sh ]];then
   FUNC_SRC=/opt/qnib/gocd/src/gocd_func.sh
else 
    echo "ERROR: No gocd_func.sh found"
    exit 1
fi
source ${FUNC_SRC}
dumpEnv

TOPDIR=$(git rev-parse --show-toplevel)
cat go.mod|awk '!/^replace/' > go.mod.tmp
mv go.mod.tmp go.mod

REGISTRY=""
if [[ ! -z ${HARBOR_REG} ]];then
   REGISTRY=${HARBOR_REG}
elif [[ ! -z ${ZOT_REGISTRY_URL} ]];then
   # DEPRECATED zot fallback — only prod-stack pipelines still set
   # ZOT_REGISTRY_URL; dev pipelines are harbor-first since the zot→harbor
   # migration. Remove once prod-stack is migrated.
   REGISTRY=${ZOT_REGISTRY_URL}
else
   echo "No HARBOR_REG (or legacy ZOT_REGISTRY_URL) defined"
   exit 1
fi

NEW_IMAGE_NAME=${REGISTRY}/metahub-api
GIT_TAG=$(git describe --tags --exact-match 2>/dev/null || true)
if [ -n "$GIT_TAG" ]; then
   TAG="$(echo ${GIT_TAG}-${GO_PIPELINE_GROUP_NAME}|tr '[:upper:]' '[:lower:]')"
else
   TAG="$(echo ${GO_PIPELINE_LABEL}|tr '[:upper:]' '[:lower:]')"
   NEW_IMAGE_NAME=$(echo ${NEW_IMAGE_NAME}/${GO_PIPELINE_GROUP_NAME}/${GO_PIPELINE_NAME}|tr '[:upper:]' '[:lower:]')
fi
if [[ ! -z "${HARBOR_REG}" ]];then
   REGISTRY=${HARBOR_REG}
elif [[ ! -z "${ZOT_REGISTRY_URL}" ]];then
   # DEPRECATED zot fallback (see above) — remove once prod-stack is migrated.
   REGISTRY=${ZOT_REGISTRY_URL}
else 
   REGISTRY=registry.gitlab.com
   echo "${GITLAB_REGISTRY_TOKEN}" | docker login registry.gitlab.com \
      -u qnib \
      --password-stdin
fi


NEW_IMAGE=$(echo ${NEW_IMAGE_NAME}:${TAG} |tr '[:upper:]' '[:lower:]')
cat >> result.yaml << EOF
metahub:
   image:
      name: ${NEW_IMAGE_NAME}
      tag: ${TAG}
EOF
echo "Deploying ${NEW_IMAGE} to metahub-dev namespace..."
git clone https://qnib:${FLUX_METAHUB_TOKEN}@gitlab.com/qnib-solutions/flux/metahub.git
yq -yi ".spec.template.spec.containers[0].image = \"${NEW_IMAGE}\"" \
   metahub/clusters/k3s.metahub/dev/metahub-api/deployment.yaml
git -C metahub config user.email "gocd@qnib.hvk"
git -C metahub config user.name "GoCD"
git -C metahub commit -am "chore(dev): update metahub-api to ${NEW_IMAGE}"
git -C metahub push
echo "Deployment triggered for ${NEW_IMAGE}"