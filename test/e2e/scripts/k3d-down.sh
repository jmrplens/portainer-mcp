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
source ./scripts/remote.sh

api_port="${E2E_K3D_API_PORT:-16443}"

# Same fixed control socket k3d-up.sh's tunnel bound (see its own comment on
# .k8s-tunnel.sock). tunnel_down has to name the exact socket tunnel_up used,
# or -O exit addresses a stale default: at best it finds nothing and leaks
# this leg's forward silently (tunnel_down suppresses ssh's own errors so a
# legitimate no-op never fails a run), at worst — if the compose leg happens
# to share that default path — it closes the COMPOSE leg's tunnel instead.
export PORTAINER_E2E_TUNNEL_SOCK="$PWD/.k8s-tunnel.sock"

ssh_dest=$(recorded_docker_host kubernetes)
if [[ -n "$ssh_dest" ]]; then
    export DOCKER_HOST="ssh://$ssh_dest"
    echo "tearing down the kubernetes leg on $ssh_dest (api port $api_port)" >&2
fi

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
# Released unconditionally, OUTSIDE the block above, not merely on a path
# that reaches it: k3d-up.sh takes this leg's lock BEFORE `k3d cluster
# create` ever runs, roughly two minutes plus a Helm install before the
# estate file this block also gates on exists. A run that dies anywhere in
# that window -- cluster creation, Helm, reading the setup token -- leaves a
# lock this leg genuinely took but no estate file for
# "$licence && -f $estate_file" to ever see again, and the block above would
# then never run at all. release_licence_lock is already holder-gated and
# warn-only (see its own doc), so calling it unconditionally here can
# neither remove a lock the compose leg holds nor fail this teardown --
# including a Community-only run, where it simply warns that there was never
# a lock to release.
release_licence_lock "$repo_root" kubernetes

k3d cluster delete "$cluster" 2>/dev/null || true

# Last, mirroring down.sh: everything above ran against the recorded
# destination and the tunnel it needed to reach the API server through.
# Clearing the Kubernetes marker here is correct because it is this leg's OWN
# marker — down.sh (the compose leg's teardown) calls this script when it
# finds a live cluster and deliberately leaves the compose marker for itself
# to clear afterwards; touching that one here would misdirect the compose
# teardown that follows.
tunnel_down "$ssh_dest"
record_docker_host "" kubernetes
