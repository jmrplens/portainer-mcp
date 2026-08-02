#!/usr/bin/env bash
# Tear down the k3d cluster and the Portainer deployed into it.
#
# The in-cluster server applies a Business Edition licence exactly like the
# compose legs do, and it must give it back before the cluster that hosts it
# is destroyed: once the cluster is gone the server is unreachable and the
# licence key would be stranded against a real account for good. Releasing it
# is attempted before the cluster delete below and is best-effort only — a
# cluster that fails to delete is worse than a stranded licence, so nothing
# here is allowed to abort the script.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_root=$(cd ../.. && pwd)
cluster="${E2E_K3D_CLUSTER:-portainer-mcp-e2e}"
estate_file="${PORTAINER_E2E_ESTATE:-$PWD/.estate.json}"

licence=""
if [[ -f "$repo_root/.env" ]]; then
    licence=$(grep -E '^PORTAINER_LICENSE=' "$repo_root/.env" | cut -d= -f2- | tr -d '"'"'"'' || true)
fi

if [[ -n "$licence" && -f "$estate_file" ]]; then
    echo "releasing kubernetes leg's business edition licence" >&2
    PORTAINER_E2E_ESTATE="$estate_file" \
    PORTAINER_E2E_LICENCE="$licence" \
        go run ./harness/cmd/provision -kubernetes -release-licence \
        || echo "warning: could not release the kubernetes licence; continuing teardown" >&2
fi

k3d cluster delete "$cluster" 2>/dev/null || true
