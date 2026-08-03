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

k3d cluster create "$cluster" --network "$network" --agents 1 --wait

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
server_ip=$(docker inspect "k3d-$cluster-server-0" \
            --format "{{(index .NetworkSettings.Networks \"$network\").IPAddress}}")
if [[ -z "$server_ip" ]]; then
    echo "could not resolve k3d-$cluster-server-0's address on $network" >&2
    exit 1
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
PORTAINER_E2E_K8S_URL="https://${server_ip}:${nodeport}" \
PORTAINER_E2E_K8S_SETUP_TOKEN="$token" \
PORTAINER_E2E_K8S_CA_FILE="$ca_file" \
PORTAINER_E2E_LICENCE="$licence" \
    go run ./harness/cmd/provision -kubernetes

echo "kubernetes leg ready: k3d-$cluster" >&2
