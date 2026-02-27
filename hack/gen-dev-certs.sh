#!/usr/bin/env bash
# gen-dev-certs.sh — generate self-signed TLS certs for local webhook development.
# The cert includes the host IP (passed as HOST_IP env var) and localhost in the SAN
# so the Kubernetes API server can reach the webhook running on the dev machine.
#
# Usage:
#   HOST_IP=192.168.5.2 ./hack/gen-dev-certs.sh
#   ./hack/gen-dev-certs.sh   # auto-detects host IP

set -euo pipefail

CERT_DIR="${CERT_DIR:-/tmp/bootchain-webhook-certs}"
HOST_IP="${HOST_IP:-}"

if [[ -z "${HOST_IP}" ]]; then
  # Try to detect the host IP that the cluster can reach.
  # Works for most local setups (Colima, minikube, k3s on Lima).
  HOST_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || true)
  if [[ -z "${HOST_IP}" ]]; then
    echo "ERROR: Could not detect host IP. Set HOST_IP env var manually." >&2
    exit 1
  fi
fi

echo "Generating dev certs for HOST_IP=${HOST_IP} in ${CERT_DIR}"
mkdir -p "${CERT_DIR}"

# Generate CA
openssl genrsa -out "${CERT_DIR}/ca.key" 2048 2>/dev/null
openssl req -new -x509 -days 365 -key "${CERT_DIR}/ca.key" \
  -subj "/CN=bootchain-dev-ca" \
  -out "${CERT_DIR}/ca.crt" 2>/dev/null

# Generate server key + CSR
openssl genrsa -out "${CERT_DIR}/tls.key" 2048 2>/dev/null
openssl req -new -key "${CERT_DIR}/tls.key" \
  -subj "/CN=bootchain-webhook" \
  -out "${CERT_DIR}/tls.csr" 2>/dev/null

# Sign with SAN for localhost, 127.0.0.1, and the detected host IP
openssl x509 -req -days 365 \
  -in "${CERT_DIR}/tls.csr" \
  -CA "${CERT_DIR}/ca.crt" \
  -CAkey "${CERT_DIR}/ca.key" \
  -CAcreateserial \
  -extfile <(printf "subjectAltName=IP:127.0.0.1,IP:%s,DNS:localhost" "${HOST_IP}") \
  -out "${CERT_DIR}/tls.crt" 2>/dev/null

echo "Certs written to ${CERT_DIR}"
echo "  CA bundle (base64): $(base64 < "${CERT_DIR}/ca.crt" | tr -d '\n')"
echo ""
echo "HOST_IP=${HOST_IP}"
