#!/bin/bash
set -e

source ~/gocd_func.sh
dumpEnv


#if [[ -f /run/secrets/gocd-token ]];then
#    export GOCD_TOKEN=$(cat /run/secrets/gocd-token)
#else 
#    echo "ERROR: No GOCD token found (/run/secrets/gocd-token)"
#    exit 1
#fi
TOPDIR=$(git rev-parse --show-toplevel)

source ~/gocd.env
if [[ -z ${CI_JOB_TOKEN} ]];then
   CI_JOB_TOKEN=${GITLAB_TOKEN}
fi
cat go.mod|awk '!/^replace/' > go.mod.tmp
mv go.mod.tmp go.mod

if [[ -d exp/metalib ]];then
   echo ">> rm -rf exp/metalib"
   rm -rf exp/metalib
fi

if [[ "${DEPLOYMENT}" == "dev" ]];then
   echo "> git clone --branch=develop https://gitlab-ci-token:CI_JOB_TOKEN@github.com/metahub-registry/metahub-core exp/metalib"
   git clone --branch=develop https://gitlab-ci-token:${CI_JOB_TOKEN}@github.com/metahub-registry/metahub-core exp/metalib
elif [[ "${DEPLOYMENT}" == "prod" ]];then
   echo "> git clone --branch=main https://gitlab-ci-token:CI_JOB_TOKEN@github.com/metahub-registry/metahub-core exp/metalib"
   git clone --branch=main https://gitlab-ci-token:${CI_JOB_TOKEN}@github.com/metahub-registry/metahub-core exp/metalib
else 
   echo "ERROR: DEPLOYMENT '${DEPLOYMENT} not known or not set (dev|prod)"
   exit 1
fi
go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/exp/metalib  

rm -f metahub.db
## Fetch Artefact
curl -sH "Authorization: bearer ${GOCD_TOKEN}" \
     -o metahub.db "${GO_SERVER_URL}/files/${GO_DEPENDENCY_LOCATOR_METAHUB_ADMIN}/RunQnib/misc/gocd/bin/metahub.db"
NEW_IMAGE=metahub-api-test:${GO_PIPELINE_LABEL}
docker build -f Dockerfile.legacy -t ${NEW_IMAGE} .
docker save -o ${TOPDIR}/metahub-api.tar ${NEW_IMAGE}
docker rmi ${NEW_IMAGE}