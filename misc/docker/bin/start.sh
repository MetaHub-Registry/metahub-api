#!/bin/bash
set -e

if [[ !-f ${CFG_FILE} ]];then
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

metahub-api serve --config=${CFG_FILE} \
    --port=${METAHUB_API_PORT} \
    --user-db=${METAHUB_DB}