#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
CONTAINER_TOOL=${CONTAINER_TOOL:-podman}
MICROCKS_CONTAINER_NAME=${MICROCKS_CONTAINER_NAME:-maintainerd-microcks}
MICROCKS_PORT=${MICROCKS_PORT:-8585}
MICROCKS_IMAGE=${MICROCKS_IMAGE:-quay.io/microcks/microcks-uber:latest-native}
MICROCKS_ARTIFACT=${MICROCKS_ARTIFACT:-$ROOT_DIR/testdata/microcks/fossa-api-mock.yaml}

if [ ! -r "${MICROCKS_ARTIFACT}" ]; then
  echo "Microcks artifact is not readable: ${MICROCKS_ARTIFACT}" >&2
  exit 1
fi

dump_logs() {
  "${CONTAINER_TOOL}" logs "${MICROCKS_CONTAINER_NAME}" 2>&1 || true
}

"${CONTAINER_TOOL}" rm -f "${MICROCKS_CONTAINER_NAME}" >/dev/null 2>&1 || true

"${CONTAINER_TOOL}" run -d \
  --name "${MICROCKS_CONTAINER_NAME}" \
  -p "${MICROCKS_PORT}:8080" \
  "${MICROCKS_IMAGE}" >/dev/null

ready=""
for _ in $(seq 1 90); do
  if curl -sf "http://localhost:${MICROCKS_PORT}/" >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done

if [ -z "${ready}" ]; then
  echo "Microcks did not become ready on port ${MICROCKS_PORT}" >&2
  dump_logs >&2
  exit 1
fi

if ! curl -fsS \
  --retry 10 \
  --retry-delay 2 \
  --retry-connrefused \
  --retry-all-errors \
  "http://localhost:${MICROCKS_PORT}/api/artifact/upload?mainArtifact=true" \
  -F "file=@${MICROCKS_ARTIFACT}" >/dev/null; then
  echo "Failed to upload Microcks artifact: ${MICROCKS_ARTIFACT}" >&2
  dump_logs >&2
  exit 1
fi

echo "MICROCKS_URL=http://localhost:${MICROCKS_PORT}"
echo "FOSSA_API_BASE=http://localhost:${MICROCKS_PORT}/rest/FOSSA%20API/4.32.3"
