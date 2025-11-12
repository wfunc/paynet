#!/usr/bin/env bash

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CERT_DIR="${ROOT}/certs"
mkdir -p "${CERT_DIR}"

# Usage: scripts/gen_certs.sh [server-cn] [default-device-cn]
SERVER_CN="${1:-paynet.local}"
DEVICE_CN="${2:-device-001}"
CA_SUBJ="${CA_SUBJ:-/CN=paynet-ca}"
VALID_DAYS="${VALID_DAYS:-3650}"
KEY_BITS="${KEY_BITS:-4096}"
FORCE="${FORCE:-0}"
SERIAL_FILE="${CERT_DIR}/ca.srl"

generate_ca() {
  if [[ "${FORCE}" != "1" && -f "${CERT_DIR}/ca.key" && -f "${CERT_DIR}/ca.crt" ]]; then
    echo "==> CA already exists, skipping (set FORCE=1 to regenerate)"
    return
  fi
  echo "==> Generating CA (${CA_SUBJ})"
  openssl req -x509 -new -nodes -newkey "rsa:${KEY_BITS}" \
    -days "${VALID_DAYS}" \
    -keyout "${CERT_DIR}/ca.key" \
    -out "${CERT_DIR}/ca.crt" \
    -subj "${CA_SUBJ}"
  rm -f "${SERIAL_FILE}"
}

generate_server() {
  if [[ "${FORCE}" != "1" && -f "${CERT_DIR}/server.key" && -f "${CERT_DIR}/server.crt" ]]; then
    echo "==> Server cert already exists, skipping (set FORCE=1 to regenerate)"
    return
  fi
  echo "==> Generating server certificate (${SERVER_CN})"
  openssl req -new -newkey "rsa:${KEY_BITS}" -nodes \
    -keyout "${CERT_DIR}/server.key" \
    -out "${CERT_DIR}/server.csr" \
    -subj "/CN=${SERVER_CN}"

  openssl x509 -req -in "${CERT_DIR}/server.csr" \
    -CA "${CERT_DIR}/ca.crt" -CAkey "${CERT_DIR}/ca.key" \
    -CAserial "${SERIAL_FILE}" -CAcreateserial \
    -out "${CERT_DIR}/server.crt" -days "${VALID_DAYS}" \
    -extfile <(printf "subjectAltName=DNS:%s\n" "${SERVER_CN}")
  rm -f "${CERT_DIR}/server.csr"
}

generate_default_device() {
  if [[ "${FORCE}" != "1" && -f "${CERT_DIR}/device.key" && -f "${CERT_DIR}/device.crt" ]]; then
    echo "==> Default device cert already exists, skipping (set FORCE=1 to regenerate)"
    return
  fi
  echo "==> Generating default device certificate (${DEVICE_CN})"
  openssl req -new -newkey "rsa:${KEY_BITS}" -nodes \
    -keyout "${CERT_DIR}/device.key" \
    -out "${CERT_DIR}/device.csr" \
    -subj "/CN=${DEVICE_CN}"

  openssl x509 -req -in "${CERT_DIR}/device.csr" \
    -CA "${CERT_DIR}/ca.crt" -CAkey "${CERT_DIR}/ca.key" \
    -CAserial "${SERIAL_FILE}" -CAcreateserial \
    -out "${CERT_DIR}/device.crt" -days "${VALID_DAYS}" \
    -extfile <(printf "subjectAltName=DNS:%s\n" "${DEVICE_CN}")
  rm -f "${CERT_DIR}/device.csr"
}

generate_ca
generate_server
generate_default_device

echo "==> Certificates available in ${CERT_DIR}"
