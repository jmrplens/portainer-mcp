#!/usr/bin/env bash
# Tear the estate down. Safe to run when nothing is up.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
docker compose down -v --remove-orphans
rm -f "${PORTAINER_E2E_ESTATE:-$PWD/.estate.json}"
