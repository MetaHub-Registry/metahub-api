#!/bin/bash
set -ex 

rm -f ./metahub 
go build -o metahub
exec ./metahub serve --config ./misc/config/compose-config.yaml --user-db ./misc/sqlite/metahub_20251209.db