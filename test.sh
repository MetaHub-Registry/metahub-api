#!/bin/bash
set -ex
echo "Running tests with coverage... -short -parallel=1"
go test -timeout 180s ./... -coverprofile coverage.txt $@
gocover-cobertura < coverage.txt > coverage.xml
gocoverstats -v -f coverage.txt --percent
