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

DOCKERFILE=""
BUILD_ARGS=""
if [[ -f metalib_result/result.yaml ]];then
   echo ">> found metalib_result.yml"
   echo "####"
   cat metalib_result/result.yaml
   echo "####"
   DOCKERFILE="Dockerfile.result"
   METALIB_IMAGE=$(yq -r .image metalib_result/result.yaml)
   cat >> result.yaml << EOF
metalib:
   image: ${METALIB_IMAGE}
EOF
   
else
   if [[ -f metalib.tar ]];then
      DOCKERFILE="Dockerfile.legacy"
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
fi
go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/exp/metalib

REGISTRY=""
if [[ ! -z ${ZOT_REGISTRY_URL} ]];then
   REGISTRY=${ZOT_REGISTRY_URL}
else
   echo "No ZOT_REGISTRY_URL defined"
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
if [[ ! -z "${ZOT_REGISTRY_URL}" ]];then
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
set -x
if [[ ! -z ${METALIB_IMAGE} ]];then
   docker build --build-arg=METALIB_IMAGE=${METALIB_IMAGE} -f ${DOCKERFILE} -t ${NEW_IMAGE} .
else 
   docker build -f ${DOCKERFILE} -t ${NEW_IMAGE} .
fi
docker push ${NEW_IMAGE}
docker rmi ${NEW_IMAGE}
