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

### Update ECS task definition
if [[ -f /run/secrets/task-def-name ]];then
    export AWS_TASK_DEF_NAME=$(cat /run/secrets/task-def-name)
fi
if [[ -z ${AWS_TASK_DEF_NAME} ]];then
  echo "SKIP: No AWS_TASK_DEF_NAME found"
  exit 0
fi
TASK_DEFINITION=$(aws ecs describe-task-definition --task-definition=${AWS_TASK_DEF_NAME})
NEW_TASK_DEFINITION=$(echo $TASK_DEFINITION | jq --arg IMAGE "$NEW_IMAGE" '.taskDefinition | .containerDefinitions[0].image = $IMAGE | del(.taskDefinitionArn) | del(.revision) | del(.status) | del(.requiresAttributes) | del(.compatibilities)')
aws ecs register-task-definition --cli-input-json "$NEW_TASK_DEFINTIION"