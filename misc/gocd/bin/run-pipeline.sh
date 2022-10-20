#!/bin/bash

if [[ -f /run/secrets/aws-ecr-access-key ]];then
    export AWS_ACCESS_KEY=$(cat /run/secrets/aws-ecr-access-key)
else 
    echo "ERROR: No AWS ECR ACCESS KEY found (/run/secrets/aws-ecr-access-key)"
    exit 1

fi
if [[ -f /run/secrets/aws-ecr-secret-access-key ]];then
    export AWS_SECRET_ACCESS_KEY=$(cat /run/secrets/aws-ecr-secret-access-key)
else 
    echo "ERROR: No AWS ECR SECRET KEY found (/run/secrets/aws-ecr-secret-access-key)"
    exit 1
fi

if [[ -f /run/secrets/gocd-token]];then
    export GOCD_TOKEN=$(cat /run/secrets/gocd-token)
#else 
    #echo "ERROR: No AWS ECR SECRET KEY found (/run/secrets/aws-ecr-secret-access-key)"
    #exit 1
fi


## Fetch Artefact
curl -H "Authorization: bearer ccca261ed4b37169d257e55ec4cdcc891daf5b09" \
     -o metahub.db "${GO_SERVER_URL}/files/${GO_DEPENDENCY_LOCATOR_METAHUB_ADMIN}/Job/metahub.db"


aws ecr get-login-password --region eu-west-1 | docker login --username AWS --password-stdin 193198941246.dkr.ecr.eu-west-1.amazonaws.com

docker build -f Dockerfile.legacy -t 193198941246.dkr.ecr.eu-west-1.amazonaws.com/qnib-golang/metahub-api:${GO_PIPELINE_LABEL} .
docker push 193198941246.dkr.ecr.eu-west-1.amazonaws.com/qnib-golang/metahub-api:${GO_PIPELINE_LABEL}