#!/bin/bash

if [[ -z "$1" ]];then
    echo "Please provide a version tag"
    exit 1
fi

if [[ -d "ext/metalib" ]];then
    git -C ext/metalib pull
else 
    git clone git@github.com:MetaHub-Registry/metahub-core.git ext/metalib
fi

cp go.mod go.mod.orig
go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/ext/metalib 
go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/minio=/go/metahub-api/ext/metalib/pkg/backends/minio
go mod edit -replace github.com/metahub-registry/metahub-core/pkg/types=/go/metahub-api/ext/metalib/pkg/types
go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/sqlite=/go/metahub-api/ext/metalib/pkg/backends/sqlite
go mod edit -replace github.com/metahub-registry/metahub-core/pkg/backends/neo=/go/metahub-api/ext/metalib/pkg/backends/neo

ARCHS="aarch64,amd64"
for x in $(echo ${ARCHS}|tr , ' '); do 
    docker build --platform linux/${x} -t qnib/metahub-api:$1-${x} .
done
mv go.mod.orig go.mod