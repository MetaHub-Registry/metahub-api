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

aws ecr get-login-password --region eu-west-1 | docker login --username AWS --password-stdin 193198941246.dkr.ecr.eu-west-1.amazonaws.com

docker build -t 193198941246.dkr.ecr.eu-west-1.amazonaws.com/qnib-golang/metahub-api:${GO_PIPELINE_COUNTER} .
docker push 193198941246.dkr.ecr.eu-west-1.amazonaws.com/qnib-golang/metahub-api:${GO_PIPELINE_COUNTER}