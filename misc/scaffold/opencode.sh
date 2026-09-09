#!/usr/bin/env bash
export TMPDIR=/tmp/metahub
export SAFEHOUSE_TRUST_WORKDIR_CONFIG=1
cd "$(dirname "$0")"
safehouse opencode
