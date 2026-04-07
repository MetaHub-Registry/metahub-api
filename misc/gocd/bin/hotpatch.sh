#!/bin/bash
set -e

if [[ -f metahub_api/result.yaml ]];then
    NEW_IMAGE=$(yq -r .metahub.image.full metahub_api/result.yaml)
else 
    echo "> Expected results file to be downloaded: metahub_api/result.yaml"
fi

echo "Hotpatching metahub-api in metahub-dev to ${NEW_IMAGE}..."
mkdir -p ~/.kube
echo "${K8S_METAHUB_KUBECONFIG}" | base64 -d > ~/.kube/config
kubectl set image deployment/metahub-api metahub-api="${NEW_IMAGE}" -n metahub-dev
kubectl rollout status deployment/metahub-api -n metahub-dev --timeout=120s
echo "Hotpatch done: ${NEW_IMAGE}"
