#!/usr/bin/env bash
# Tear the estate down. Safe to run when nothing is up.
#
# The compose EE server applies a Business Edition licence exactly like the
# Kubernetes leg does (see k3d-down.sh), and it must give it back before its
# container is destroyed: once the container is gone the server is
# unreachable and the licence key would be stranded against a real account for
# good. Releasing it is attempted before the compose teardown below and is
# best-effort only — a run that fails to bring the estate down is worse than a
# stranded licence, so nothing here is allowed to abort the script.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_root=$(cd ../.. && pwd)
source ./scripts/lib.sh

estate_file="${PORTAINER_E2E_ESTATE:-$PWD/.estate.json}"

licence=$(read_licence "$repo_root")
if [[ -n "$licence" && -f "$estate_file" ]]; then
    echo "releasing compose estate's business edition licence" >&2
    PORTAINER_E2E_ESTATE="$estate_file" \
    PORTAINER_E2E_LICENCE="$licence" \
        go run ./harness/cmd/provision -release-licence \
        || echo "warning: could not release the business edition licence; continuing teardown" >&2
fi

# --profile edge: the edge agent only ever runs under that profile (up.sh
# starts it in a second pass, once EDGE_ID/EDGE_KEY exist), and without
# naming the profile here `down` leaves it running.
docker compose --profile edge down -v --remove-orphans
rm -f "$estate_file"
rm -f "${PORTAINER_E2E_EDGE_ENV:-$PWD/.edge.env}"
