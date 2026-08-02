#!/usr/bin/env bash
# Tear the estate down. Safe to run when nothing is up.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
# --profile edge: the edge agent only ever runs under that profile (up.sh
# starts it in a second pass, once EDGE_ID/EDGE_KEY exist), and without
# naming the profile here `down` leaves it running.
docker compose --profile edge down -v --remove-orphans
rm -f "${PORTAINER_E2E_ESTATE:-$PWD/.estate.json}"
rm -f "${PORTAINER_E2E_EDGE_ENV:-$PWD/.edge.env}"
