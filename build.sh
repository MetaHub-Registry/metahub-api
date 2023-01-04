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

if [[ -d "ext/metalib" ]];then
    git -C ext/metalib pull
else 
    git clone git@github.com:MetaHub-Registry/metahub-core.git ext/metalib
fi

go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/ext/metalib 
go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/minio=/go/metahub-api/ext/metalib/pkg/backends/minio
go mod edit -replace github.com/metahub-registry/metahub-core/pkg/types=/go/metahub-api/ext/metalib/pkg/types
go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/sqlite=/go/metahub-api/ext/metalib/pkg/backends/sqlite
go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/neo=/go/metahub-api/ext/metalib/pkg/backends/neo

ARCH=$(uname -m)
for x in $(echo ${ARCHS}|tr , ' '); do
    if [[ "${ARCH}" == "x86_64" ]] && [[ "$x" == "amd64" ]];then
        docker build --pull -t registry.github.com/metahub-registry/metahub-api:$1-${x} .
    else
        docker build --pull --platform linux/${x} -t registry.github.com/metahub-registry/metahub-api:$1-${x} .
    fi
done
git checkout go.mod