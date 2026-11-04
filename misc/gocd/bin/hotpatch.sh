#!/bin/bash
set -e

# Target namespace is parameterized so the same script serves dev (metahub-dev)
# and prod (metahub). Defaults to metahub-dev to keep existing behaviour.
K8S_NAMESPACE="${K8S_NAMESPACE:-metahub-dev}"

if [[ -f metahub_api/result.yaml ]];then
    NEW_IMAGE=$(yq -r .metahub.image.full metahub_api/result.yaml)
else 
    echo "> Expected results file to be downloaded: metahub_api/result.yaml"
fi

echo "Hotpatching metahub-api in ${K8S_NAMESPACE} to ${NEW_IMAGE}..."
mkdir -p ~/.kube
echo "${K8S_METAHUB_KUBECONFIG}" | base64 -d > ~/.kube/config
kubectl set image deployment/metahub-api metahub-api="${NEW_IMAGE}" -n "${K8S_NAMESPACE}"
kubectl rollout status deployment/metahub-api -n "${K8S_NAMESPACE}" --timeout=120s
echo "Hotpatch done: ${NEW_IMAGE}"
