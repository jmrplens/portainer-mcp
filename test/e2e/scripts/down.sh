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
cluster="${E2E_K3D_CLUSTER:-portainer-mcp-e2e}"

# The documented order is `make e2e-k8s-down` before `make e2e-down` (see
# README.md), so that by the time this script deletes the estate file below,
# the Kubernetes leg has already had its own chance to release its licence
# through that file. Calling `make e2e-down` first, or on its own without
# `make e2e-k8s-down` ever running, would otherwise delete the one file
# k3d-down.sh needs to release a licence attached to a server it is about to
# destroy — and unlike this compose leg, once k3d-down.sh's `k3d cluster
# delete` runs, that server is gone and the licence is unrecoverable in
# place. Detecting a still-running cluster here and tearing it down first,
# regardless of which order the two `make` targets were invoked in, makes
# `make e2e-down` alone safe rather than relying on every caller to remember
# the documented sequence.
if command -v k3d >/dev/null && k3d cluster list -o json 2>/dev/null | grep -q "\"name\":\"$cluster\""; then
    echo "kubernetes leg still up: tearing it down first so its licence can be released before this estate file is removed" >&2
    ./scripts/k3d-down.sh
fi

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
