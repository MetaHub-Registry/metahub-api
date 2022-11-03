#!/bin/bash
set -e

# bash /go/metahub/misc/docker/bin/test-push.sh
docker pull alpine:3.16.1
docker tag alpine:3.16.1 ${MH_URL}/qnib/alpine:3.16.1
docker login ${MH_URL} -p mypassword -u qnib
docker push ${MH_URL}/qnib/alpine:3.16.1