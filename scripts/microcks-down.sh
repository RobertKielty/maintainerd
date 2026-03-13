#!/usr/bin/env bash
set -euo pipefail

CONTAINER_TOOL=${CONTAINER_TOOL:-podman}
MICROCKS_CONTAINER_NAME=${MICROCKS_CONTAINER_NAME:-maintainerd-microcks}

"${CONTAINER_TOOL}" rm -f "${MICROCKS_CONTAINER_NAME}" >/dev/null 2>&1 || true
