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
source ./scripts/lib.sh
cluster="${E2E_K3D_CLUSTER:-portainer-mcp-e2e}"
namespace=portainer
estate_file="${PORTAINER_E2E_ESTATE:-$PWD/.estate.json}"

licence=$(read_licence "$repo_root")

if [[ -n "$licence" && -f "$estate_file" ]]; then
    # The provisioner's release call verifies this server's certificate just
    # like provisioning did, so it needs the certificate again — the cluster
    # is still up at this point, so it can still be read out of the pod.
    ca_file="$(mktemp)"
    trap 'rm -f "$ca_file"' EXIT
    if fetch_k8s_ca "k3d-$cluster" "$namespace" > "$ca_file" && grep -q "BEGIN CERTIFICATE" "$ca_file"; then
        echo "releasing kubernetes leg's business edition licence" >&2
        PORTAINER_E2E_ESTATE="$estate_file" \
        PORTAINER_E2E_LICENCE="$licence" \
        PORTAINER_E2E_K8S_CA_FILE="$ca_file" \
            go run ./harness/cmd/provision -kubernetes -release-licence \
            || echo "warning: could not release the kubernetes licence; continuing teardown" >&2
    else
        echo "warning: could not read the portainer certificate to release the licence; continuing teardown" >&2
    fi
fi

k3d cluster delete "$cluster" 2>/dev/null || true
