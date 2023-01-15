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
	cat /opt/metahub-api/metahub-api.yaml \
		|wildq -M -i yaml '.minio.user = "AWSUSER"' |sed -e "s/AWSUSER/${AWS_ACCESS_KEY}/"  \
		|wildq -M -i yaml '.minio.password = "PASSWD"' |sed -e "s#PASSWD#${AWS_SECRET_ACCESS_KEY}#"  \
		|wildq -M -i yaml '.minio.region = "eu-west-1"' \
		|wildq -M -i yaml '.minio.url = "s3.amazonaws.com"' \
		|wildq -M -i yaml '.minio.configs.bucket = "CONFIGS_BUCKET"' | sed -e "s/CONFIGS_BUCKET/${CONFIGS_BUCKET}/" \
		|wildq -M -i yaml '.minio.layers.bucket = "LAYERS_BUCKET"' | sed -e "s/LAYERS_BUCKET/${LAYERS_BUCKET}/" \
		|wildq -M -i yaml '.minio.layers.redirect = "true"' \
		|wildq -M -i yaml '.minio.manifests.bucket = "MANIFESTS_BUCKET"' | sed -e "s/MANIFESTS_BUCKET/${MANIFESTS_BUCKET}/" \
		> ${CFG_FILE}
fi
echo "[INFO] sha256sum ${CFG_FILE} |awk '{print \$1}' > ${METAHUB_SHA256_DIR}/${METAHUB_SHA256_FNAME}"
sha256sum ${CFG_FILE} |awk '{print $1}' > ${METAHUB_SHA256_DIR}/${METAHUB_SHA256_FNAME}

metahub-api serve --config=${CFG_FILE} \
    --port=${METAHUB_API_PORT} \
    --user-db=${METAHUB_DB}