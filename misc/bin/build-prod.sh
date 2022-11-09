#!/bin/bash

if [[ ! -f metahub.db ]];then
    echo "Please place metahub.db in the current directory"
    exit 0
else 
    echo -n "Are you sure the metahub.db file is ready for production (with all data)? [n/Y] "
    read REPLY
    if [[ $REPLY =~ ^[Nn]$ ]];then
        echo "Aborting..."
        exit 0
    fi
fi

cat go.mod|awk '!/^replace/' > go.mod.tmp
mv go.mod.tmp go.mod
git clone git@github.com:MetaHub-Registry/metahub-core.git exp/metalib
go mod edit -replace github.com/metahub-registry/metahub-core=/go/metahub-api/exp/metalib  


aws ecr get-login-password --region eu-west-1 | docker login --username AWS --password-stdin 193198941246.dkr.ecr.eu-west-1.amazonaws.com
NEW_IMAGE=193198941246.dkr.ecr.eu-west-1.amazonaws.com/metahub-api-prod:$(date +"%F-%H%M")
docker build --platform=linux/amd64 -f Dockerfile.legacy -t ${NEW_IMAGE} .
docker push ${NEW_IMAGE}

git checkout go.mod
rm -rf exp