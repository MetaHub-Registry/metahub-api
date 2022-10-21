#!/bin/bash
set -e

if [[ ! -z ${GO_PIPELINE_LABEL} ]];then
    env |sort >> gocd.env 
fi

if [[ -f /run/secrets/gocd-token ]];then
    export GOCD_TOKEN=$(cat /run/secrets/gocd-token)
else 
    echo "ERROR: No AWS ECR SECRET KEY found (/run/secrets/aws-ecr-secret-access-key)"
    exit 1
fi


## Fetch Artefact
curl -sH "Authorization: bearer ccca261ed4b37169d257e55ec4cdcc891daf5b09" \
     -o metahub.db "${GO_SERVER_URL}/files/${GO_DEPENDENCY_LOCATOR_METAHUB_ADMIN}/Job/metahub.db"

$(aws --profile=ecr ecr get-login --no-include-email --region eu-west-1)
NEW_IMAGE=193198941246.dkr.ecr.eu-west-1.amazonaws.com/metahub-api-prod:gocd-${GO_PIPELINE_LABEL}
docker build -f Dockerfile.legacy -t ${NEW_IMAGE} .
docker push ${NEW_IMAGE}
