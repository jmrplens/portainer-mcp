#!/usr/bin/env bash
# Shared helpers sourced by the e2e scripts. Not meant to be run directly.

# read_licence echoes the Business Edition licence key from the repository
# root's gitignored .env, or nothing if the file or the key is absent. Every
# script that needs the licence reads it through this one place rather than
# repeating the same grep|cut|tr pipeline with room to drift between copies.
read_licence() {
    local repo_root="$1"
    local licence=""
    if [[ -f "$repo_root/.env" ]]; then
        licence=$(grep -E '^PORTAINER_LICENSE=' "$repo_root/.env" | cut -d= -f2- | tr -d '"'"'"'' || true)
    fi
    echo "$licence"
}

# fetch_k8s_ca writes the Kubernetes leg's in-cluster Portainer certificate to
# stdout as PEM.
#
# The Helm chart creates no TLS secret for it: Portainer's own binary
# generates this self-signed certificate at startup and writes it only to its
# data volume (/data/certs/cert.pem inside the pod), never to anything the
# Kubernetes API itself exposes. The image is also distroless — no shell, no
# cat, confirmed by a plain `kubectl exec` failing with "executable file not
# found in $PATH" for every one of sh/ls/busybox — so reaching that file needs
# an ephemeral debug container that shares the target's process namespace,
# reading it through /proc/1/root rather than execing into the container
# itself.
#
# --attach=true is not optional: kubectl debug defaults it to false without
# -i/-t, and without it the command that reads the file still runs, but
# nothing streams its output back, so the caller would see an empty, silently
# wrong result rather than a failure. -q suppresses kubectl's own "Targeting
# container..." status lines, which otherwise land on the same stdout as the
# certificate. No --container name is passed, so kubectl mints a fresh one
# each call — this is invoked once by k3d-up.sh and again, against the same
# still-running pod, by k3d-down.sh, and a repeated fixed name would collide
# with the first debug container Kubernetes never lets a pod remove.
fetch_k8s_ca() {
    local context="$1" namespace="$2"
    local pod
    pod=$(kubectl --context "$context" -n "$namespace" get pod \
          -l app.kubernetes.io/name=portainer -o jsonpath='{.items[0].metadata.name}')
    if [[ -z "$pod" ]]; then
        echo "could not find the portainer pod in $namespace" >&2
        return 1
    fi
    kubectl --context "$context" -n "$namespace" debug -q --attach=true \
        --image=busybox:1.36 --target=portainer "$pod" \
        -- cat /proc/1/root/data/certs/cert.pem
}
