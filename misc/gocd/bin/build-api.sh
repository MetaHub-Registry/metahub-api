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

echo "TOKEN length: $(echo ${GITLAB_REGISTRY_TOKEN}|wc -c)"
#if [[ -f /run/secrets/gocd-token ]];then
#    export GOCD_TOKEN=$(cat /run/secrets/gocd-token)
#else 
#    echo "ERROR: No GOCD token found (/run/secrets/gocd-token)"
#    exit 1
#fi
TOPDIR=$(git rev-parse --show-toplevel)
cat go.mod|awk '!/^replace/' > go.mod.tmp
mv go.mod.tmp go.mod

if [[ -d exp/metalib ]];then
   if [[ "${DEPLOYMENT}" == "dev" ]];then
      echo ">> it -C exp/metalib pull origin develop"
      git -C exp/metalib pull origin develop
   fi
fi

if [[ -f metalib.tar ]];then
   echo ">> tar -xf metalib.tar -C exp/"
   mkdir -p exp
   tar -xf metalib.tar -C exp/
   echo ">> pwd: $(pwd)"
   echo ">> ls -la exp/metalib"
   ls -la exp/metalib
elif [[ "${DEPLOYMENT}" == "dev" ]];then
   echo "> git clone --branch=develop https://gitlab-ci-token:GITLAB_REPO_RO@github.com/metahub-registry/metahub-core.git exp/metalib"
   git clone --branch=develop https://gitlab-ci-token:${GITLAB_REPO_RO}@github.com/metahub-registry/metahub-core.git exp/metalib
elif [[ "${DEPLOYMENT}" == "prod" ]];then
   echo "> git clone --branch=main https://gitlab-ci-token:GITLAB_REPO_RO@github.com/metahub-registry/metahub-core.git exp/metalib"
   git clone --branch=main https://gitlab-ci-token:${GITLAB_REPO_RO}@github.com/metahub-registry/metahub-core.git exp/metalib
else 
   echo "ERROR: DEPLOYMENT '${DEPLOYMENT} not known or not set (dev|prod)"
   exit 1
fi
go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/exp/metalib  

GIT_TAG=$(git describe --tags --exact-match 2>/dev/null || true)
if [ -n "$GIT_TAG" ]; then
   TAG="{{ GOCD_GROUP }}-${GIT_TAG}"
else
   TAG="{{ GOCD_GROUP }}.$(date +%F-%H)"
fi
if [[ ! -z "${ZOT_REGISTRY_URL}" ]];then
   REGISTRY=${ZOT_REGISTRY_URL}
else 
   REGISTRY=registry.gitlab.com
   echo "${GITLAB_REGISTRY_TOKEN}" | docker login registry.gitlab.com \
      -u qnib \
      --password-stdin
fi

NEW_IMAGE_NAME=${REGISTRY}/qnib-golang/metahub-api
NEW_IMAGE=${NEW_IMAGE_NAME}:${TAG}
cat > result.yaml << EOF
image:
  name: ${NEW_IMAGE_NAME}
  tag: ${TAG}
EOF
docker build -f Dockerfile.legacy -t ${NEW_IMAGE} .
docker save -o ${TOPDIR}/metahub-api.tar ${NEW_IMAGE}
docker push ${NEW_IMAGE}
docker rmi ${NEW_IMAGE}
