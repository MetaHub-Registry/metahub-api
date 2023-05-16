#!/bin/bash


if [[ "$1" == "local" ]];then
  SONAR_QUBE_URL=${SONAR_QUBE_URL_LOCAL}
  SONAR_QUBE_TOKEN=${SONAR_QUBE_TOKEN_LOCAL}
fi

set -x 

sonar-scanner \
  -Dsonar.branch.name=$(git rev-parse --abbrev-ref HEAD) \
  -Dsonar.projectKey=${SONAR_QUBE_PROJECT} \
  -Dsonar.sources=. \
  -Dsonar.host.url=${SONAR_QUBE_URL} \
  -Dsonar.login=${SONAR_QUBE_TOKEN}
