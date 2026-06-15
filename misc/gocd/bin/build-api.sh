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
fi
go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/exp/metalib

REGISTRY=""
if [[ ! -z ${ZOT_REGISTRY_URL} ]];then
   REGISTRY=${ZOT_REGISTRY_URL}
elif [[ ! -z ${HARBOR_REG} ]];then
   REGISTRY=${HARBOR_REG}
else
   echo "No ZOT_REGISTRY_URL defined"
   exit 1
fi

NEW_IMAGE_NAME=${REGISTRY}/metahub/api
GIT_TAG=$(git describe --tags --exact-match 2>/dev/null || true)
if [ -n "$GIT_TAG" ]; then
   TAG="$(echo ${GIT_TAG}-${GO_PIPELINE_GROUP_NAME}|tr '[:upper:]' '[:lower:]')"
else
   TAG="$(echo ${GO_PIPELINE_LABEL}|tr '[:upper:]' '[:lower:]')"
   NEW_IMAGE_NAME=$(echo ${NEW_IMAGE_NAME}/${GO_PIPELINE_GROUP_NAME}/${GO_PIPELINE_NAME}|tr '[:upper:]' '[:lower:]')
fi
echo "> REGISTRY: ${REGISTRY}"
echo "> NEW_IMAGE_NAME: ${NEW_IMAGE_NAME}"

# Harbor login is required whenever Harbor creds are present: the build may pull
# base images from Harbor (e.g. Dockerfile.result FROM harbor.example.com/...), and we
# may also push there. This is independent of the push target (Zot vs Harbor).
if [[ ! -z "${HARBOR_REG}" && ! -z "${HARBOR_TOKEN}" ]];then
   echo "> Login to ${HARBOR_REG} as 'robot$good'"
   echo "HARBOR_TOKEN length: $(echo ${HARBOR_TOKEN}|wc -c)"
   docker login ${HARBOR_REG} \
      -u 'robot$good' \
      -p "${HARBOR_TOKEN}"
else
   echo "> WARNING: HARBOR_REG/HARBOR_TOKEN not set; skipping Harbor login (base image pulls from Harbor may fail with 401)"
fi

docker login registry.gitlab.com \
   -u qnib \
   -p "${GITLAB_REGISTRY_TOKEN}"

NEW_IMAGE=$(echo ${NEW_IMAGE_NAME}:${TAG} |tr '[:upper:]' '[:lower:]')
set -x
if [[ ! -z ${METALIB_IMAGE} ]];then
   docker build --build-arg=METALIB_IMAGE=${METALIB_IMAGE} -f ${DOCKERFILE} -t ${NEW_IMAGE} .
else 
   docker build -f ${DOCKERFILE} -t ${NEW_IMAGE} .
fi
docker push ${NEW_IMAGE}
docker rmi ${NEW_IMAGE}
gdh patch -f result.yaml \
   --add=metahub.image.reg=${REGISTRY} \
   --add=metahub.image.name=${NEW_IMAGE_NAME} \
   --add=metahub.image.tag=${TAG} \
   --add=metahub.image.full=${NEW_IMAGE}
