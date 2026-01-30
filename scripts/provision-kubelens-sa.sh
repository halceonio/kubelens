#!/usr/bin/env bash
set -euo pipefail

if [[ ${1:-} == "" || ${2:-} == "" ]]; then
  echo "Usage: $0 <namespace> <serviceaccount> [kubeconfig-output]" >&2
  echo "Example: $0 apps kubelens-test ./kubelens-test.kubeconfig" >&2
  exit 1
fi

NAMESPACE="$1"
SERVICEACCOUNT="$2"
OUTPUT="${3:-./kubelens-${NAMESPACE}.kubeconfig}"
ROLE_NAME="${SERVICEACCOUNT}-${NAMESPACE}-reader"

kubectl get namespace "${NAMESPACE}" >/dev/null

kubectl -n "${NAMESPACE}" apply -f - <<EOF_MANIFEST
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${SERVICEACCOUNT}
  namespace: ${NAMESPACE}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: ${ROLE_NAME}
rules:
  - apiGroups: [""]
    resources:
      - pods
      - pods/log
      - secrets
      - configmaps
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ${ROLE_NAME}
subjects:
  - kind: ServiceAccount
    name: ${SERVICEACCOUNT}
    namespace: ${NAMESPACE}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ${ROLE_NAME}
EOF_MANIFEST

CONTEXT=$(kubectl config current-context)
CLUSTER=$(kubectl config view -o jsonpath='{.contexts[?(@.name=="'"${CONTEXT}"'")].context.cluster}')
SERVER=$(kubectl config view -o jsonpath='{.clusters[?(@.name=="'"${CLUSTER}"'")].cluster.server}')
CA_DATA=$(kubectl config view --raw -o jsonpath='{.clusters[?(@.name=="'"${CLUSTER}"'")].cluster.certificate-authority-data}')

if [[ -z "${CA_DATA}" ]]; then
  CA_FILE=$(kubectl config view --raw -o jsonpath='{.clusters[?(@.name=="'"${CLUSTER}"'")].cluster.certificate-authority}')
  if [[ -n "${CA_FILE}" && -f "${CA_FILE}" ]]; then
    CA_DATA=$(base64 < "${CA_FILE}" | tr -d '\n')
  fi
fi

TOKEN=""
if kubectl -n "${NAMESPACE}" create token "${SERVICEACCOUNT}" >/dev/null 2>&1; then
  TOKEN=$(kubectl -n "${NAMESPACE}" create token "${SERVICEACCOUNT}")
else
  SECRET=$(kubectl -n "${NAMESPACE}" get sa "${SERVICEACCOUNT}" -o jsonpath='{.secrets[0].name}')
  if [[ -z "${SECRET}" ]]; then
    echo "Could not find a token secret for the service account." >&2
    exit 1
  fi
  TOKEN=$(kubectl -n "${NAMESPACE}" get secret "${SECRET}" -o jsonpath='{.data.token}' | base64 --decode)
fi

OUT_DIR=$(dirname "${OUTPUT}")
mkdir -p "${OUT_DIR}"

cat > "${OUTPUT}" <<EOF_KUBECONFIG
apiVersion: v1
kind: Config
clusters:
- name: ${CLUSTER}
  cluster:
    server: ${SERVER}
    certificate-authority-data: ${CA_DATA}
users:
- name: ${SERVICEACCOUNT}-${NAMESPACE}
  user:
    token: ${TOKEN}
contexts:
- name: ${SERVICEACCOUNT}-${NAMESPACE}
  context:
    cluster: ${CLUSTER}
    namespace: ${NAMESPACE}
    user: ${SERVICEACCOUNT}-${NAMESPACE}
current-context: ${SERVICEACCOUNT}-${NAMESPACE}
EOF_KUBECONFIG

chmod 600 "${OUTPUT}"

echo "Kubeconfig written to ${OUTPUT}"
