#!/bin/bash
set -e

if [[ -z "${METAHUB_SHA256_DIR}" ]]; then
    METAHUB_SHA256_DIR=/var/metahub/run/
fi
if [[ -z "${METAHUB_SHA256_FNAME}" ]]; then
    METAHUB_SHA256_FNAME=metahub-api.yaml.sha256
fi

if [[ -z "${CFG_FILE}" ]]; then
	echo "ERROR: No CFG_FILE set"
	exit 1
fi
if [[ ! -f ${CFG_FILE} ]];then
	: "${MINIO_PASSWD:?set MINIO_PASSWD when rendering the config}"
	: "${NEO4J_PASSWD:?set NEO4J_PASSWD when rendering the config}"
	if [[ -n "${GODMODE_USER:-}" ]]; then
		: "${GODMODE_PASS:?set GODMODE_PASS when GODMODE_USER is set}"
		: "${GODMODE_SIGNING_KEY:?set GODMODE_SIGNING_KEY when GODMODE_USER is set}"
	fi
	gonja-cli /opt/metahub-api/metahub-api.yaml.tpl ${CFG_FILE}
fi
  
echo "[INFO] sha256sum ${CFG_FILE} |awk '{print \$1}' > ${METAHUB_SHA256_DIR}/${METAHUB_SHA256_FNAME}"
sha256sum ${CFG_FILE} |awk '{print $1}' > ${METAHUB_SHA256_DIR}/${METAHUB_SHA256_FNAME}

metahub-api serve --config=${CFG_FILE} \
    --port=${METAHUB_API_PORT} \
    --user-db=${METAHUB_DB}