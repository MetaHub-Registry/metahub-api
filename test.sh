#!/bin/bash
set -ex

go test -short -timeout 180s ./... -coverprofile coverage.txt
gocover-cobertura < coverage.txt > coverage.xml
gocoverstats -v -f coverage.txt --percent
