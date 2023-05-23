#!/bin/bash

echo ">> echo MH_PASS | docker login MH_URL --password-stdin -u $1"
echo ${MH_PASS} | docker login ${MH_URL} --password-stdin -u $1 2>/dev/null
