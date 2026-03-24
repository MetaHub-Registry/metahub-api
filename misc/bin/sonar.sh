#!/bin/bash


if [[ "$1" == "local" ]];then
  SONAR_QUBE_URL=${SONAR_QUBE_URL_LOCAL}
  SONAR_QUBE_TOKEN=${SONAR_QUBE_TOKEN_LOCAL}
fi
if [[ ! -z "$1" ]];then
  TS=$(git show -s --format=%cI $1| tail -n1 |awk -F"T" '{print $1}')
  TAG_ARG="-Dsonar.projectVersion=$1 -Dsonar.branch.name=main -Dsonar.projectDate=${TS}"
else
  TAG_ARG="-Dsonar.branch.name=$(git rev-parse --abbrev-ref HEAD)"
fi

set -x 


sonar-scanner ${TAG_ARG} \
  -Dsonar.projectKey=${SONAR_QUBE_PROJECT} \
  -Dsonar.sources=. \
  -Dsonar.go.exclusions="**/*_test.go,**/test_*.go,**/vendor/**,**/test.go,**/testcases.go,**/*swagger*.go,**/coverage.*" \
  -Dsonar.go.coverage.reportPaths=coverage.txt \
  -Dsonar.go.tests.reportPaths=report.json \
  -Dsonar.host.url=${SONAR_QUBE_URL} \
  -Dsonar.token=${SONAR_QUBE_TOKEN}