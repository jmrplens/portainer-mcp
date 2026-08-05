#!/usr/bin/env bash
# Shared helpers sourced by the e2e scripts. Not meant to be run directly.

# read_env_var echoes one key's value from the repository root's gitignored
# .env, or nothing when the file or the key is absent. The key is anchored on
# both sides ("^KEY=") so that asking for a prefix of a real key — the kind of
# typo that silently returns the wrong secret — matches nothing instead.
read_env_var() {
    local repo_root="$1" key="$2"
    [[ -f "$repo_root/.env" ]] || return 0
    grep -E "^${key}=" "$repo_root/.env" | head -n1 | cut -d= -f2- | tr -d '"'"'"'' || true
}

# read_licence echoes the Business Edition licence key. Every script that needs
# it reads it through this one place rather than repeating the pipeline.
read_licence() {
    read_env_var "$1" PORTAINER_LICENSE
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

# docker_ssh_dest echoes the SSH destination of the Docker host the estate
# should run on, or nothing for "this machine".
#
# The .env key alone is NOT enough. Remote execution requires
# PORTAINER_E2E_REMOTE=1, which only the *-remote make targets set, because a
# key sitting in .env would otherwise make a plain `make e2e-up` — typed
# without thinking about it — reach out to somebody's real machine. Two
# independent things must line up: the intent (the flag) and the address (the
# key).
#
# When the flag is set and the key is missing this dies rather than falling
# back to local. A silent fallback would mean asking for a remote GPU run and
# getting a local GPU-less one, whose suites skip — a green run that tested
# nothing.
docker_ssh_dest() {
    local repo_root="$1"
    [[ "${PORTAINER_E2E_REMOTE:-0}" == "1" ]] || return 0
    local dest
    dest=$(read_env_var "$repo_root" PORTAINER_E2E_DOCKER_SSH)
    if [[ -z "$dest" ]]; then
        echo "PORTAINER_E2E_REMOTE=1 but no PORTAINER_E2E_DOCKER_SSH in $repo_root/.env" >&2
        return 1
    fi
    echo "$dest"
}

# record_docker_host and recorded_docker_host carry the destination from `up`
# to `down`, so teardown takes no flag at all.
#
# Without this, `make e2e-up-remote` followed by the ordinary `make e2e-down`
# would tear down a local estate that does not exist and leave the remote one
# running on somebody else's machine — the single worst failure mode this
# feature has. The marker is written after the estate is up and removed by
# teardown; an empty or absent file means local.
docker_host_marker() {
    echo "${PORTAINER_E2E_DOCKER_HOST_FILE:-$PWD/.docker-host}"
}

record_docker_host() {
    local dest="$1" marker
    marker=$(docker_host_marker)
    if [[ -z "$dest" ]]; then
        rm -f "$marker"
    else
        printf '%s\n' "$dest" > "$marker"
    fi
}

recorded_docker_host() {
    local marker
    marker=$(docker_host_marker)
    [[ -f "$marker" ]] || return 0
    head -n1 "$marker"
}

# on_docker_host runs a command on whichever machine the Docker daemon lives
# on. With an empty destination that is this machine; otherwise it is over
# SSH. BatchMode refuses to prompt: a script that blocks on a passphrase in
# CI hangs until the job times out rather than failing with a usable message.
on_docker_host() {
    local dest="$1"; shift
    if [[ -z "$dest" ]]; then
        bash -c "$*"
    else
        ssh -o BatchMode=yes -o ConnectTimeout=10 "$dest" "$*"
    fi
}

# write_to_docker_host reads stdin and writes it to a path on the Docker host.
# Used for the CDI specification, which has to be readable by the daemon that
# bind-mounts it — i.e. on the daemon's own filesystem, not this one.
write_to_docker_host() {
    local dest="$1" path="$2"
    if [[ -z "$dest" ]]; then
        cat > "$path"
    else
        ssh -o BatchMode=yes -o ConnectTimeout=10 "$dest" "cat > '$path'"
    fi
}

# strip_cdi_hooks removes every hooks: block from a CDI specification on
# stdin.
#
# This is not a simplification: it is required. The hooks generated by
# nvidia-ctk all invoke /usr/bin/nvidia-cdi-hook, a glibc binary, and the
# estate's dind image is Alpine (musl). Leaving them in makes every nested
# GPU container fail at creation with "fork/exec /usr/bin/nvidia-cdi-hook: no
# such file or directory"; installing the binary is not possible either
# (measured: gcompat resolves the loader but not NVML's symbols). The device
# nodes and library mounts the hooks would have decorated are declared in the
# specification itself, so stripping them costs only the SONAME symlinks and
# the ldcache refresh — which any workload recovers with a plain `ldconfig`.
# See docs/api-divergences.md for the full account.
#
# The filter is indentation-based rather than a YAML parse because no YAML
# tool is guaranteed present on the Docker host: a hooks: key opens a block,
# and every following line that is blank or more deeply indented belongs to
# it.
strip_cdi_hooks() {
    awk '
        /^[[:space:]]*hooks:[[:space:]]*$/ { hook_indent = match($0, /[^ ]/); skipping = 1; next }
        skipping {
            if ($0 ~ /^[[:space:]]*$/) { next }
            if (match($0, /[^ ]/) > hook_indent) { next }
            skipping = 0
        }
        { print }
    '
}

# cdi_device_id echoes the CDI device the estate asks for. "all" rather than a
# specific index because the estate does not know how many GPUs the Docker
# host has and does not need to: one device request that means "whatever is
# there" is what a test asserting the GPU is reachable actually wants.
cdi_device_id() {
    echo "nvidia.com/gpu=all"
}

# detect_gpu_name echoes the model name of the Docker host's first NVIDIA GPU,
# or nothing when it has none. Absence is never an error — a contributor
# without a GPU must still be able to bring the estate up, with the GPU suites
# skipping the way they skip without a Business Edition licence.
#
# The result is captured into a variable first and only emitted once
# on_docker_host has actually succeeded, rather than letting its stdout reach
# the caller directly with a trailing "|| true": that would forward whatever
# nvidia-smi had already written even when it went on to fail, which is lower
# stakes here than for gpu_cdi_spec (a truncated display name is still just a
# name) but is still not a distinction a reader of this file should have to
# make between the two functions. "if ! raw=$(...); then" is exempt from
# set -e — the failing command sits in an if's condition — so no "|| true" is
# needed at all.
#
# The command string sets pipefail itself: without it, "nvidia-smi ... |
# head -n1" inside the fresh bash -c (or remote shell) reports only head's
# exit status, so a nvidia-smi that printed a line and then failed would
# still read as success and its output would still be captured. Verified
# directly with a stub that does exactly that.
detect_gpu_name() {
    local dest="$1" raw
    if ! raw=$(on_docker_host "$dest" \
        'set -o pipefail; command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null | head -n1' \
        2>/dev/null); then
        return 0
    fi
    printf '%s\n' "$raw"
}

# gpu_cdi_spec echoes a hookless CDI specification for the Docker host's GPUs,
# or nothing when nvidia-ctk is unavailable or fails. See strip_cdi_hooks for
# why the hooks cannot survive: the estate's dind is Alpine and every
# generated hook invokes a glibc binary.
#
# The generator's output is captured into a variable first and only piped
# through strip_cdi_hooks once on_docker_host has actually succeeded. An
# earlier version of this function ran the generator directly into the pipe
# with a trailing "|| true": that discards the exit *status* of a failing
# nvidia-ctk but not whatever partial, well-formed-looking YAML it had
# already written to stdout before dying — which strip_cdi_hooks would then
# happily forward as if it were a complete specification. Task 4 mounts this
# output into the dind and only checks that the file is non-empty, so a
# truncated document would pass that check and break every nested GPU
# container, silently, on exactly the hosts this feature exists for.
# Verified directly: a stub nvidia-ctk that prints a well-formed but
# truncated document and then exits 1 made the earlier version return that
# truncated document instead of nothing.
#
# This shape also sidesteps the grouping the earlier version needed: an
# on_docker_host call piped straight into strip_cdi_hooks with "A || true"
# appended, rather than grouped as "{ A || true; }", parses as
# "A || (true | strip_cdi_hooks)" — on a successful A the whole right-hand
# side, strip_cdi_hooks included, would never run, and the hooks would reach
# the caller unstripped. Capturing into a variable first removes the pipe
# from this function entirely, so that trap cannot recur here even by
# accident.
gpu_cdi_spec() {
    local dest="$1" raw
    if ! raw=$(on_docker_host "$dest" \
        'command -v nvidia-ctk >/dev/null 2>&1 && nvidia-ctk cdi generate --format=yaml 2>/dev/null' \
        2>/dev/null); then
        return 0
    fi
    [[ -n "$raw" ]] || return 0
    printf '%s\n' "$raw" | strip_cdi_hooks
}
