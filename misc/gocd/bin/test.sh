#!/bin/bash
# Run the metahub-api test suite in CI (GoCD 'Test' stage) and produce
# coverage.txt for the downstream 'Sonar' stage.
#
# metahub-api's go.mod carries `replace github.com/metahub-registry/metahub-core => ../metahub-core`,
# which does not exist in a fresh CI clone. Re-wire metalib the same way build-api.sh
# does: drop the local replace and re-point it at the metalib image the upstream
# metalib pipeline produced (fetched into metalib_result/result.yaml), then run
# ./test.sh (which writes coverage.txt).
set -ex

# Drop the local '../metahub-core' replace directive.
cat go.mod | awk '!/^replace/' > go.mod.tmp
mv go.mod.tmp go.mod

if [[ ! -f metalib_result/result.yaml ]];then
   echo "ERROR: metalib_result/result.yaml not found (expected fetch from the metalib pipeline)"
   exit 1
fi
METALIB_IMAGE=$(yq -r .image metalib_result/result.yaml)
echo "> metalib image: ${METALIB_IMAGE}"

# Export the metalib source from its built image and replace the module locally.
mkdir -p ext/metalib
(
  cd ext/metalib
  docker run -d --name target-test "${METALIB_IMAGE}" sleep 30
  docker export target-test | tar xf - pkg go.mod go.sum
  docker rm -f target-test
)
go mod edit -replace github.com/metahub-registry/metahub-core=${PWD}/ext/metalib
go mod tidy

# Run the suite → coverage.txt (consumed by the Sonar stage via a GoCD artifact).
./test.sh
