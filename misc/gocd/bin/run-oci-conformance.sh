#!/bin/bash

docker run --rm \
  -v $(pwd)/results:/results \
  -w /results \
  -e OCI_ROOT_URL="http://localhost:8080" \
  -e OCI_NAMESPACE="oci/conformance" \
  -e OCI_USERNAME="oci" \
  -e OCI_PASSWORD="${OCI_MH_PASS:-test}" \
  -e OCI_TEST_PULL=1 \
  -e OCI_TEST_PUSH=0 \
  -e OCI_TEST_CONTENT_DISCOVERY=1 \
  -e OCI_TEST_CONTENT_MANAGEMENT=0 \
  -e OCI_HIDE_SKIPPED_WORKFLOWS=0 \
  -e OCI_DEBUG=0 \
  -e OCI_DELETE_MANIFEST_BEFORE_BLOBS=0 \
  qnib/oci-conformance:2022-11-21