#!/bin/bash

docker pull ${1}
docker tag ${1} ${MH_URL}/oci/${1}
docker push ${MH_URL}/oci/${1}
