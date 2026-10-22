#!/bin/bash

export DOCKER_BUILDKIT=1

: "${REPO_NAME:="registry.github.com/metahub-registry/metahub-api"}"
: "${USERDB_FILE:="misc/sqlite/user_2023-05-31.sqlite3"}"

if [[ -z "$1" ]];then
    echo "Please provide a version tag"
    exit 1
fi

if [[ -z $2 ]];then
    ARCHS="amd64,arm64"
else 
    ARCHS="$2"
fi
for x in $(echo ${ARCHS}|tr , ' '); do
    IMG_NAME="${REPO_NAME}:$1-${x}"
    set -x
    docker buildx build --platform=linux/${x} \
        --load -f Dockerfile.dev \
        --build-arg=USERDB_FILE=${USERDB_FILE} \
        --build-context metahub-core=../metahub-core \
        -t ${IMG_NAME} .
    docker push ${IMG_NAME}
    set +x
done