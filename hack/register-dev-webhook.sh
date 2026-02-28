#!/usr/bin/env bash
# register-dev-webhook.sh — register the MutatingWebhookConfiguration pointing to
# the operator running locally on the dev machine (outside the cluster).
#
# Requires: gen-dev-certs.sh to have run first.
#
# Usage:
#   HOST_IP=192.168.5.2 ./hack/register-dev-webhook.sh
#   ./hack/register-dev-webhook.sh   # auto-detects host IP

set -euo pipefail

CERT_DIR="${CERT_DIR:-/tmp/bootchain-webhook-certs}"
HOST_IP="${HOST_IP:-}"
# Port 9444 is the socat IPv4 proxy that forwards to the IPv6 webhook socket on macOS.
# The webhook server itself listens on ::1:9443 (IPv6), but Colima can only reach the
# Mac host via IPv4 (192.168.64.1). socat bridges the two.
WEBHOOK_PORT="${WEBHOOK_PORT:-9444}"

if [[ -z "${HOST_IP}" ]]; then
  HOST_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || true)
  if [[ -z "${HOST_IP}" ]]; then
    echo "ERROR: Could not detect host IP. Set HOST_IP env var manually." >&2
    exit 1
  fi
fi

if [[ ! -f "${CERT_DIR}/ca.crt" ]]; then
  echo "ERROR: CA cert not found at ${CERT_DIR}/ca.crt. Run hack/gen-dev-certs.sh first." >&2
  exit 1
fi

CA_BUNDLE=$(base64 < "${CERT_DIR}/ca.crt" | tr -d '\n')

echo "Registering webhook: host=${HOST_IP} port=${WEBHOOK_PORT}"

kubectl apply -f - <<EOF
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: bootchain-dev-webhook
webhooks:
  - name: mdeployment-v1.kb.io
    admissionReviewVersions: ["v1"]
    clientConfig:
      url: "https://${HOST_IP}:${WEBHOOK_PORT}/mutate-apps-v1-deployment"
      caBundle: "${CA_BUNDLE}"
    rules:
      - apiGroups: ["apps"]
        apiVersions: ["v1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["deployments"]
    failurePolicy: Fail
    sideEffects: None
    namespaceSelector:
      matchLabels:
        bootchain-webhook: enabled
EOF

kubectl apply -f - <<EOF
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: bootchain-dev-validating-webhook
webhooks:
  - name: vbootdependency-v1alpha1.kb.io
    admissionReviewVersions: ["v1"]
    clientConfig:
      url: "https://${HOST_IP}:${WEBHOOK_PORT}/validate-core-bootchain-operator-ruicoelho-dev-v1alpha1-bootdependency"
      caBundle: "${CA_BUNDLE}"
    rules:
      - apiGroups: ["core.bootchain-operator.ruicoelho.dev"]
        apiVersions: ["v1alpha1"]
        operations: ["CREATE", "UPDATE"]
        resources: ["bootdependencies"]
    failurePolicy: Fail
    sideEffects: None
    namespaceSelector:
      matchLabels:
        bootchain-webhook: enabled
EOF

echo ""
echo "Done. Label a namespace to activate the webhooks:"
echo "  kubectl label namespace default bootchain-webhook=enabled"
