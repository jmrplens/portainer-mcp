#!/usr/bin/env bash
# Create a k3d cluster on the estate's network and deploy Portainer into it.
#
# k3d rather than kind because its nodes are Docker containers, so --network
# places them on the same network as the compose estate. Measured: 41s for the
# cluster, 79s for the chart.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_root=$(cd ../.. && pwd)
source ./scripts/lib.sh
source ./scripts/remote.sh

api_port="${E2E_K3D_API_PORT:-16443}"

# The Kubernetes leg's tunnel needs its own control socket, distinct from the
# compose leg's default (.ssh-tunnel.sock — see remote.sh). Both legs can
# legitimately be remote at the same time: `make e2e-up-remote` followed by
# `make e2e-k8s-up-remote` is exactly the combination record_docker_host's own
# per-leg marker exists for, and tunnel_up's first act is to close whatever
# already holds ITS socket. Sharing the compose leg's socket would make this
# leg's tunnel_up silently tear down the compose leg's already-open tunnel the
# moment both run remotely together — invisible until the compose suite's
# next request against http://localhost:19000 fails with connection refused.
# See test/e2e/.gitignore's .k8s-tunnel.sock entry, which exists for exactly
# this path.
export PORTAINER_E2E_TUNNEL_SOCK="$PWD/.k8s-tunnel.sock"

ssh_dest=$(docker_ssh_dest "$repo_root")
if [[ -n "$ssh_dest" ]]; then
    export DOCKER_HOST="ssh://$ssh_dest"
    echo "running the kubernetes leg on $ssh_dest via $DOCKER_HOST" >&2
fi
# The Kubernetes leg records its OWN marker, never the compose one. The two
# legs are brought up by separate targets and can legitimately live in
# different places — `make e2e-k8s-up` (local) alongside `make e2e-up-remote`
# is a combination a user can type. Writing a single shared marker would make
# whichever ran second silently redirect the other's teardown; Task 4 split
# the marker per leg for exactly this reason.
record_docker_host "$ssh_dest" kubernetes

cluster="${E2E_K3D_CLUSTER:-portainer-mcp-e2e}"
network="${E2E_NETWORK:-portainer-mcp-e2e_default}"
namespace=portainer

for tool in k3d kubectl helm; do
    command -v "$tool" >/dev/null || { echo "$tool is required but not installed" >&2; exit 1; }
done

# Same gitignored .env the compose legs (up.sh) read. Its absence is not an
# error: the Kubernetes leg still comes up, just as Community Edition, and the
# estate records that so suites skip the Business Edition assertions.
licence=$(read_licence "$repo_root")
if [[ -z "$licence" ]]; then
    echo "no PORTAINER_LICENSE in .env: Kubernetes leg will be Community Edition only" >&2
fi

# --api-port is pinned rather than left to k3d because a remote cluster is
# only reachable through an SSH tunnel (the k3s serving certificate covers
# 127.0.0.1 and not the host's LAN address), and a tunnel cannot forward a
# port nobody recorded.
#
# --gpus all is NOT unconditional, despite the plan's own description of it
# as "harmless on a host without one". Measured directly on a host with no
# NVIDIA Container Toolkit installed at all (the ordinary case for a
# contributor's laptop or a CI runner, as opposed to a host that has the
# toolkit but simply has no card): `docker run --gpus all` — and therefore
# `k3d cluster create --gpus all`, which passes the flag straight through —
# fails outright with "failed to discover GPU vendor from CDI: no known GPU
# vendor found", rolling back the entire cluster. That is a hard failure of
# `make e2e-k8s-up` on exactly the machines this task's own verification step
# requires to keep working, which is a stricter and more common case than
# "a GPU-aware host with zero devices" — the docker-compose.gpu.yml override
# for the compose leg already avoids this same trap by gating on detection
# rather than passing the flag unconditionally; k3d cluster create gets the
# same treatment here for the same reason. See detect_gpu_name in lib.sh.
gpu_flags=()
gpu_name=$(detect_gpu_name "$ssh_dest")
if [[ -n "$gpu_name" ]]; then
    gpu_flags=(--gpus all)
    echo "gpu detected on the docker host: $gpu_name (passing --gpus all to the kubernetes node)" >&2
else
    echo "no GPU on the docker host: the kubernetes leg's GPU suites will skip" >&2
fi

k3d cluster create "$cluster" --network "$network" --agents 1 --wait \
    --api-port "127.0.0.1:${api_port}" "${gpu_flags[@]}"

# When the cluster is remote, k3d writes a kubeconfig naming a host this
# machine cannot use: the SSH alias does not resolve, and the k3s serving
# certificate does not cover the LAN address either. Forward the API port and
# point the context at the one address the certificate does cover.
if [[ -n "$ssh_dest" ]]; then
    tunnel_up "$ssh_dest" "$api_port"
    kubectl config set-cluster "k3d-$cluster" --server "https://127.0.0.1:${api_port}" >/dev/null
fi

# The published chart's own app version is ee-2.39.5, five minor versions
# behind the product. The image tag is overridden so the Kubernetes leg tests
# the same 2.44.0 our vendored specs describe; without this the API surface
# under test would silently differ from the API surface we generated against.
helm repo add portainer https://portainer.github.io/k8s/ >/dev/null
helm repo update >/dev/null
kubectl --context "k3d-$cluster" create namespace "$namespace" --dry-run=client -o yaml \
    | kubectl --context "k3d-$cluster" apply -f -
helm install --kube-context "k3d-$cluster" -n "$namespace" portainer portainer/portainer \
    --set enterpriseEdition.enabled=true \
    --set enterpriseEdition.image.tag=2.44.0 \
    --set service.type=NodePort \
    --set tls.force=true \
    --wait --timeout 5m

# Unlike the containers, this server cannot be given --no-setup-token, so the
# token is read from its logs. It is printed once at startup as
# "setup_token=<64 hex>".
token=$(kubectl --context "k3d-$cluster" -n "$namespace" logs deploy/portainer \
        | grep -oE 'setup_token=[0-9a-f]{64}' | head -1 | cut -d= -f2)
if [[ -z "$token" ]]; then
    echo "could not read the setup token from the portainer pod logs" >&2
    exit 1
fi

# Selected by name, not position: .spec.ports[0] depends on the Service
# template's own field order, which --set tls.force=true does not pin down.
# If the rendered Service ever carries both an http and an https port,
# indexing by position risks picking the http NodePort and building an https
# URL that targets the wrong port. Falling back to the sole port only when no
# port is named "https" keeps this working against a Service that exposes
# only one.
nodeport=$(kubectl --context "k3d-$cluster" -n "$namespace" \
           get svc portainer -o jsonpath='{.spec.ports[?(@.name=="https")].nodePort}')
if [[ -z "$nodeport" ]]; then
    nodeport=$(kubectl --context "k3d-$cluster" -n "$namespace" \
               get svc portainer -o jsonpath='{.spec.ports[0].nodePort}')
fi

# k3d publishes only the API server port to the host (via the load balancer,
# on 127.0.0.1); a NodePort service is not published anywhere unless the
# cluster is created with an explicit --port mapping, which this one is not.
# Measured directly: a request to 127.0.0.1:<nodePort> is refused. The node
# container is reachable by its own address on the estate's network instead —
# Docker routes a user-defined bridge network's subnet from the host without
# any port publish, which is the same property --network was chosen for in
# the first place, just used from the host side instead of a sibling
# container's.
#
# That routing property belongs to whichever machine runs the Docker daemon.
# When $ssh_dest is non-empty this script (and the `go run ./harness/cmd/
# provision -kubernetes` below it) run on a DIFFERENT machine, which has no
# route onto the Docker host's bridge network at all — the same reason
# up.sh's estate is reached through a tunnel rather than a container address.
# k8s_url, built below, is what actually gets handed to the provisioner;
# server_ip:nodeport survives here only because it is also what the forward
# just below has to name as ITS remote-side target.
server_ip=$(docker inspect "k3d-$cluster-server-0" \
            --format "{{(index .NetworkSettings.Networks \"$network\").IPAddress}}")
if [[ -z "$server_ip" ]]; then
    echo "could not resolve k3d-$cluster-server-0's address on $network" >&2
    exit 1
fi

# Locally, server_ip:nodeport is exactly what the provisioner (running on
# this same machine as the Docker daemon) reaches directly, unchanged from
# before this task. Remotely, this process is NOT the Docker host, so it has
# no route onto server_ip at all -- the same reason the compose leg's ports
# are tunnelled rather than dialled by container address. The API port's
# tunnel (opened above, before Helm ran) cannot carry this forward too: the
# NodePort is not known until the Service above exists, which is after that
# tunnel already had to be live for kubectl. tunnel_add_forward asks the
# SAME already-open master for one more forward instead of closing and
# reopening it (which would have to remember to re-request the API port
# forward too, or silently drop it) -- see its own comment in remote.sh for
# why its success is polled rather than trusted from ssh's exit code alone.
#
# The forwarded local port is the NodePort's own number: nothing outside
# this script's own next few lines refers to it, so there is no reason to
# invent a second one, and k8s NodePorts (30000-32767 by default) do not
# collide with this leg's other fixed ports (16443, 19000, 19001).
#
# TLS verification still holds after the address changes: ClientWithCA
# (test/e2e/harness/tls.go) sets the handshake's ServerName from the pinned
# certificate's own SAN, not from whatever address was dialled -- the same
# property `kubectl config set-cluster --server https://127.0.0.1:...`
# already relies on above. Reaching the same server through 127.0.0.1
# instead of server_ip changes nothing it checks.
k8s_url="https://${server_ip}:${nodeport}"
if [[ -n "$ssh_dest" ]]; then
    if ! tunnel_add_forward "$ssh_dest" "$nodeport" "$server_ip" "$nodeport"; then
        echo "could not forward the kubernetes leg's NodePort: the provisioner would not be able to reach it" >&2
        exit 1
    fi
    k8s_url="https://127.0.0.1:${nodeport}"
fi

# The provisioner verifies this server's certificate rather than skipping
# verification, so it needs the certificate itself first. See fetch_k8s_ca in
# lib.sh for why an ephemeral debug container is what reads it out.
ca_file="$(mktemp)"
trap 'rm -f "$ca_file"' EXIT
if ! fetch_k8s_ca "k3d-$cluster" "$namespace" > "$ca_file" || ! grep -q "BEGIN CERTIFICATE" "$ca_file"; then
    echo "could not read the portainer server's certificate out of the running pod: cannot verify TLS for the Kubernetes leg" >&2
    exit 1
fi

PORTAINER_E2E_ESTATE="${PORTAINER_E2E_ESTATE:-$PWD/.estate.json}" \
PORTAINER_E2E_K8S_URL="$k8s_url" \
PORTAINER_E2E_K8S_SETUP_TOKEN="$token" \
PORTAINER_E2E_K8S_CA_FILE="$ca_file" \
PORTAINER_E2E_LICENCE="$licence" \
    go run ./harness/cmd/provision -kubernetes

echo "kubernetes leg ready: k3d-$cluster" >&2
