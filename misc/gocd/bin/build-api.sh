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

if [[ "${DEPLOYMENT}" == "dev" ]];then
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

NEW_IMAGE=metahub-api-test:${GO_PIPELINE_LABEL}
docker build -f Dockerfile.legacy -t ${NEW_IMAGE} .
docker save -o ${TOPDIR}/metahub-api.tar ${NEW_IMAGE}
docker rmi ${NEW_IMAGE}