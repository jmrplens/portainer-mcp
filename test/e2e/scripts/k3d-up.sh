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

cluster="${E2E_K3D_CLUSTER:-portainer-mcp-e2e}"
network="${E2E_NETWORK:-portainer-mcp-e2e_default}"
namespace=portainer

# Checked BEFORE anything below touches the marker: on a machine missing one
# of these, record_docker_host used to run first and wipe whatever this leg's
# marker already recorded, then exit 1 having done nothing else -- a run that
# fails immediately still left a still-running remote cluster's own marker
# gone. Failing here first means a missing tool never touches state at all.
for tool in k3d kubectl helm; do
    command -v "$tool" >/dev/null || { echo "$tool is required but not installed" >&2; exit 1; }
done

# The Kubernetes leg records its OWN marker, never the compose one. The two
# legs are brought up by separate targets and can legitimately live in
# different places — `make e2e-k8s-up` (local) alongside `make e2e-up-remote`
# is a combination a user can type. Writing a single shared marker would make
# whichever ran second silently redirect the other's teardown; Task 4 split
# the marker per leg for exactly this reason.
#
# refuse_docker_host_switch runs first for the identical reason it does in
# up.sh: record_docker_host with an empty destination deletes this leg's
# marker unconditionally, and a plain `make e2e-k8s-up` typed after
# `make e2e-k8s-up-remote` must not silently orphan the still-running remote
# cluster that marker is the only record of.
refuse_docker_host_switch "$ssh_dest" kubernetes
record_docker_host "$ssh_dest" kubernetes

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
# "a GPU-aware host with zero devices".
#
# The gate is NOT detect_gpu_name alone. An earlier version of this script
# gated on it directly, which is wrong: detect_gpu_name only proves
# nvidia-smi answers, and nvidia-smi ships with the DRIVER — a host can have a
# working driver and no NVIDIA Container Toolkit at all, which is exactly the
# "failed to discover GPU vendor from CDI" host the paragraph above describes.
# On such a host the old gate opened anyway and reintroduced the very
# regression this comment exists to prevent. gpu_cdi_spec is the stronger
# probe: it shells out to `nvidia-ctk cdi generate`, part of the toolkit
# itself, so a non-empty, correctly-shaped result means the toolkit --gpus all
# actually needs is present, not merely that a card exists. up.sh already
# requires the same thing (a generated and validated CDI specification) before
# it will offer the compose leg's dind a GPU at all, and degrades to GPU-less
# with a warning otherwise; this mirrors that rule for the Kubernetes leg's
# --gpus all, on the same host, for the same reason.
gpu_flags=()
gpu_name=$(detect_gpu_name "$ssh_dest")
if [[ -n "$gpu_name" ]]; then
    gpu_cdi_probe=$(gpu_cdi_spec "$ssh_dest")
    if [[ -n "$gpu_cdi_probe" ]] && printf '%s\n' "$gpu_cdi_probe" | grep -q '^cdiVersion:'; then
        gpu_flags=(--gpus all)
        echo "gpu detected on the docker host: $gpu_name (nvidia container toolkit present; passing --gpus all to the kubernetes node)" >&2
    else
        echo "warning: $gpu_name present but no usable nvidia container toolkit found (nvidia-ctk cdi generate produced nothing usable); continuing without --gpus all, the kubernetes leg's GPU suites will skip" >&2
        gpu_name=""
    fi
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

# GPU support for the Kubernetes leg, when the node has a card. Absent means
# the GPU suites skip, exactly as they do without a Business Edition licence.
#
# This checks the SERVER node itself (docker exec ... ls /dev/nvidia0), not
# merely whether --gpus all was passed to `k3d cluster create` above: the
# node container is where the device actually has to show up for the device
# plugin below to find anything, and it is the same signal the device plugin
# itself depends on. --gpus all is a whole-cluster flag applied identically to
# every node k3d creates, so checking the server node alone is enough to
# decide whether to install the plugin at all.
k8s_gpu=""
if kubectl --context "k3d-$cluster" get nodes -o jsonpath='{.items[*].metadata.name}' >/dev/null 2>&1 \
   && docker exec "k3d-${cluster}-server-0" sh -c 'ls /dev/nvidia0' >/dev/null 2>&1; then
    # test/e2e/k8s/nvidia-device-plugin.yaml hardcodes /usr/lib/x86_64-linux-gnu
    # as the node's driver library directory (its host-libs volume) — a
    # Debian/amd64 assumption. On a node whose driver libraries live
    # elsewhere (arm64, a non-Debian base image) the plugin's pods crash-loop
    # on a missing LD_LIBRARY_PATH target, and the rollout wait below would
    # burn the full 180s before failing and aborting this script under
    # set -euo pipefail — with the cluster (and, if remote, the SSH tunnel to
    # it) still running, since `k3d cluster create` installs no cleanup trap.
    # Probing for the directory first turns that into an immediate, named
    # skip instead.
    if ! docker exec "k3d-${cluster}-server-0" sh -c 'ls /usr/lib/x86_64-linux-gnu/libcuda.so.*' >/dev/null 2>&1; then
        echo "warning: gpu detected on the kubernetes node but /usr/lib/x86_64-linux-gnu (the driver library path test/e2e/k8s/nvidia-device-plugin.yaml hardcodes) is missing there; continuing without the kubernetes leg's GPU" >&2
    else
        # The device plugin generates its own CDI specification, and that
        # specification carries hooks invoking /usr/bin/nvidia-ctk. The k3s
        # node image has no standard libc layout — neither /lib/ld-musl* nor
        # /lib64/ld-linux-x86-64.so.2 — so a real nvidia-ctk cannot run there,
        # and container creation fails with "fork/exec /usr/bin/nvidia-ctk: no
        # such file or directory".
        #
        # The hook only has to exist and exit zero: the device nodes and the
        # library mounts come from the specification itself, not from the
        # hook. On the single-node cluster an earlier reconnaissance used,
        # this alone was enough for a pod requesting nvidia.com/gpu:1 to run
        # and report the real card. That result did NOT reproduce on the
        # two-node (--agents 1) cluster this script actually creates: see
        # docs/api-divergences.md §10.3, an open item this shim does not
        # resolve by itself. Node capacity (what the DaemonSet below
        # advertises, and what GetKubernetesGPUInfo will read) is reliable; a
        # scheduled GPU workload is not, yet.
        #
        # Installed on EVERY node of the cluster, not just the server. The
        # DaemonSet's own tolerations (`operator: Exists`, see
        # test/e2e/k8s/nvidia-device-plugin.yaml) schedule it onto both nodes
        # of this --agents 1 cluster, and a plugin pod on a node with no shim
        # fails exactly like §10.3's open item the moment it tries to create a
        # container. An earlier version of this script wrote the shim only
        # onto server-0; kubectl's own node listing is not guaranteed to sort
        # the server node first, and in practice
        # test/e2e/suite/fixtures_test.go's own `.items[0]` read sorts to
        # agent-0 — the node that version left without a shim.
        #
        # This loop, the apply just below, and the rollout wait after that are
        # all guarded the same way the driver-library probe just above is, and
        # for the identical reason: under set -euo pipefail, `docker exec`
        # failing on either node, `kubectl apply` failing outright (a
        # malformed manifest, an API server hiccup, the context briefly
        # unreachable), or `rollout status` never converging within its 180s
        # budget, would otherwise abort the whole script with the cluster
        # (and, if remote, its SSH tunnel) still running — `k3d cluster
        # create` installs no cleanup trap, so recovery would need a manual
        # `make e2e-k8s-down` against a host the operator may not even be
        # watching. The device plugin cannot do anything useful without the
        # shim on every node it might land on, so a failed write means
        # skipping the plugin entirely rather than applying a manifest that
        # can only fail later; a failed apply means there is nothing to wait
        # on, so the rollout wait is skipped too; a rollout that never
        # converges means the DaemonSet stays applied (removing it now would
        # race whatever kubectl apply already started) but the leg is still
        # reported GPU-less so the suites skip instead of trusting an
        # unconfirmed plugin.
        shim_ok=1
        for node in "k3d-${cluster}-server-0" "k3d-${cluster}-agent-0"; do
            if ! docker exec "$node" sh -c \
                'printf "#!/bin/sh\nexit 0\n" > /usr/bin/nvidia-ctk && chmod 0755 /usr/bin/nvidia-ctk'; then
                echo "warning: could not write the nvidia-ctk shim onto $node; the device plugin cannot work without it on every node, continuing without the kubernetes leg's GPU" >&2
                shim_ok=""
                break
            fi
        done
        if [[ -n "$shim_ok" ]]; then
            if kubectl --context "k3d-$cluster" apply -f ./k8s/nvidia-device-plugin.yaml; then
                if kubectl --context "k3d-$cluster" -n kube-system rollout status \
                    daemonset/nvidia-device-plugin --timeout=180s; then
                    k8s_gpu="1"
                    echo "gpu advertised to the kubernetes leg" >&2
                else
                    echo "warning: nvidia-device-plugin daemonset did not roll out within 180s; continuing without the kubernetes leg's GPU" >&2
                fi
            else
                echo "warning: kubectl apply -f ./k8s/nvidia-device-plugin.yaml failed; continuing without the kubernetes leg's GPU" >&2
            fi
        fi
    fi
else
    echo "no GPU on the kubernetes node: GPU suites will skip" >&2
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

# PORTAINER_E2E_K8S_GPU records THIS leg's own GPU capability -- distinct
# from PORTAINER_E2E_GPU_NAME/PORTAINER_E2E_GPU_CDI_DEVICE, which only up.sh
# ever sets and which describe the compose leg's dind. estate.HasGPU() reads
# those two, and the Kubernetes leg deliberately does not touch them (see
# runKubernetes's own comment in cmd/provision/main.go); a split-host estate
# (a different Docker host for each leg, which README.md calls a legitimate
# combination) can have a GPU on one leg and not the other, and a test
# gating on the wrong leg's field would skip a passing run or fail a
# GPU-less one. See harness.Estate.HasKubernetesGPU.
PORTAINER_E2E_ESTATE="${PORTAINER_E2E_ESTATE:-$PWD/.estate.json}" \
PORTAINER_E2E_K8S_URL="$k8s_url" \
PORTAINER_E2E_K8S_SETUP_TOKEN="$token" \
PORTAINER_E2E_K8S_CA_FILE="$ca_file" \
PORTAINER_E2E_LICENCE="$licence" \
PORTAINER_E2E_K8S_GPU="$k8s_gpu" \
    go run ./harness/cmd/provision -kubernetes

echo "kubernetes leg ready: k3d-$cluster" >&2
