#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "Usage: scripts/gen_device_cert.sh <device-cn> [output-prefix]"
  exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CERT_DIR="${ROOT}/certs"
mkdir -p "${CERT_DIR}"

DEVICE_CN="$1"
SAFE_CN="$(echo "${DEVICE_CN}" | tr -c 'a-zA-Z0-9._-' '_')"
OUT_PREFIX="${2:-device-${SAFE_CN}}"
KEY_BITS="${KEY_BITS:-4096}"
VALID_DAYS="${VALID_DAYS:-3650}"
FORCE="${FORCE:-0}"
SERIAL_FILE="${CERT_DIR}/ca.srl"

CA_KEY="${CERT_DIR}/ca.key"
CA_CRT="${CERT_DIR}/ca.crt"

if [[ ! -f "${CA_KEY}" || ! -f "${CA_CRT}" ]]; then
  echo "CA files not found in ${CERT_DIR}. Run scripts/gen_certs.sh first."
  exit 1
fi

KEY_FILE="${CERT_DIR}/${OUT_PREFIX}.key"
CRT_FILE="${CERT_DIR}/${OUT_PREFIX}.crt"
CSR_FILE="${CERT_DIR}/${OUT_PREFIX}.csr"

if [[ "${FORCE}" != "1" && -f "${KEY_FILE}" && -f "${CRT_FILE}" ]]; then
  echo "Certificate for ${OUT_PREFIX} already exists. Set FORCE=1 to overwrite."
  exit 0
fi

echo "==> Generating key/csr for ${DEVICE_CN} (${OUT_PREFIX})"
openssl req -new -newkey "rsa:${KEY_BITS}" -nodes \
  -keyout "${KEY_FILE}" \
  -out "${CSR_FILE}" \
  -subj "/CN=${DEVICE_CN}"

echo "==> Signing certificate using CA"
openssl x509 -req -in "${CSR_FILE}" \
  -CA "${CA_CRT}" -CAkey "${CA_KEY}" \
  -CAserial "${SERIAL_FILE}" -CAcreateserial \
  -out "${CRT_FILE}" -days "${VALID_DAYS}" \
  -extfile <(printf "subjectAltName=DNS:%s\n" "${DEVICE_CN}")

rm -f "${CSR_FILE}"

echo "==> Device certificate created:"
echo "    Key : ${KEY_FILE}"
echo "    Cert: ${CRT_FILE}"
