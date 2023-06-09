#!/bin/bash

: "${DEST_REG:="localhost:5050"}"

skopeo copy --dest-tls-verify=false --dest-precompute-digests \
	 docker://registry.gitlab.com/${1} \
	 docker://${DEST_REG}/${1}
