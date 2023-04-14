#!/bin/bash

export DOCKER_BUILDKIT=1

if [[ -z "$1" ]];then
    echo "Please provide a version tag"
    exit 1
fi

if [[ -z $2 ]];then
    ARCHS="amd64,arm64"
else 
    ARCHS="$2"
fi
if [[  ! -z $3 ]];then
    TARGET_OPTS="--target=$3"
fi

ARCH=$(uname -m)
for x in $(echo ${ARCHS}|tr , ' '); do
    if [[ "${ARCH}" == "x86_64" ]] && [[ "$x" == "amd64" ]];then
        IMG_NAME="registry.github.com/metahub-registry/metahub-api:$1-${x}"
        if [[ ! -x "${TARGET_OPTS}" ]];then
            IMG_NAME="${IMG_NAME}-$3"
        fi
        set -x
        docker build --pull ${TARGET_OPTS} -t ${IMG_NAME} .
        set +x
    else
        IMG_NAME="registry.github.com/metahub-registry/metahub-api:$1-${x}"
        if [[ ! -x ${TARGET_OPTS} ]];then
            IMG_NAME="${IMG_NAME}-$3"
        fi
        set -x
        docker build --pull ${TARGET_OPTS} --ssh default=$SSH_AUTH_SOCK --platform linux/${x} -t ${IMG_NAME} .
        set +x
    fi
done