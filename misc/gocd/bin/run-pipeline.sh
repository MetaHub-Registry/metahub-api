#!/bin/bash
set -e

if [[ ! -z ${GO_PIPELINE_LABEL} ]];then
    env |sort >> gocd.env 
fi

if [[ -f /run/secrets/gocd-token ]];then
    export GOCD_TOKEN=$(cat /run/secrets/gocd-token)
else 
    echo "ERROR: No GOCD token found (/run/secrets/gocd-token)"
    exit 1
fi
if [[ -z ${CI_JOB_TOKEN} ]];then
   CI_JOB_TOKEN=${GITLAB_TOKEN}
fi
cat go.mod|awk '!/^replace/'
git clone https://gitlab-ci-token:${CI_JOB_TOKEN}@github.com/metahub-registry/metahub-core exp/metalib
go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/exp/metalib  

## Fetch Artefact
curl -sH "Authorization: bearer ${GOCD_TOKEN}" \
     -o metahub.db "${GO_SERVER_URL}/files/${GO_DEPENDENCY_LOCATOR_METAHUB_ADMIN}/Job/metahub.db"

$(aws --profile=ecr ecr get-login --no-include-email --region eu-west-1)
NEW_IMAGE=193198941246.dkr.ecr.eu-west-1.amazonaws.com/metahub-api-prod:gocd-${GO_PIPELINE_LABEL}
docker build -f Dockerfile.legacy -t ${NEW_IMAGE} .
docker push ${NEW_IMAGE}
