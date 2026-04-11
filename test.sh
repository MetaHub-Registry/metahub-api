#!/bin/bash
set -ex
export CGO_CFLAGS="-Wno-gnu-folding-constant"

echo "Running tests with coverage... -short -parallel=1"
go test -timeout 240s ./... -coverprofile coverage.txt $@
gocover-cobertura < coverage.txt > coverage.xml
gocoverstats -v -f coverage.txt --percent
