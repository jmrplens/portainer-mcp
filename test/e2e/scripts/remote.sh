#!/usr/bin/env bash
# SSH tunnel lifecycle for an estate whose Docker daemon runs on another host.
# Sourced by up.sh and down.sh; not meant to be run directly.
#
# docker-compose.yml publishes Portainer on 127.0.0.1 of whichever machine
# runs the daemon. Run remotely that is the remote loopback, so the Go suite
# cannot reach it. Rather than widen the bind address — which would put a
# newly initialised Portainer on the owner's LAN for the length of every run —
# these helpers forward the same ports back here, leaving every URL the suite
# uses (http://localhost:19000) exactly as it is when running locally.

# tunnel_socket_path echoes the OpenSSH ControlMaster socket both directions
# address. Overridable through PORTAINER_E2E_TUNNEL_SOCK so tests can point it
# at a temporary directory.
tunnel_socket_path() {
    echo "${PORTAINER_E2E_TUNNEL_SOCK:-$PWD/.ssh-tunnel.sock}"
}

# tunnel_up opens one multiplexed SSH connection forwarding each named port
# from the Docker host's loopback to this machine's. A no-op when the
# destination is empty, which is what makes local runs unchanged.
#
# ExitOnForwardFailure is load-bearing: without it ssh reports success when a
# local port is already taken, and the suite then runs against whatever else
# is listening on 19000 — a failure that looks like a Portainer bug.
tunnel_up() {
    local dest="$1"; shift
    [[ -n "$dest" ]] || return 0
    local sock forwards=()
    sock=$(tunnel_socket_path)
    tunnel_down "$dest"
    local port
    for port in "$@"; do
        forwards+=(-L "${port}:127.0.0.1:${port}")
    done
    ssh -M -S "$sock" -f -N \
        -o BatchMode=yes -o ConnectTimeout=10 -o ExitOnForwardFailure=yes \
        "${forwards[@]}" "$dest"
    echo "tunnel open to $dest for ports $*" >&2
}

# tunnel_down closes the multiplexed connection. Succeeds when nothing is
# open: teardown must never be the thing that fails a run, and `-O exit`
# against a missing socket is an ordinary, expected outcome after a local run
# or a second `make e2e-down`.
tunnel_down() {
    local dest="$1"
    [[ -n "$dest" ]] || return 0
    local sock
    sock=$(tunnel_socket_path)
    ssh -S "$sock" -O exit "$dest" 2>/dev/null || true
    rm -f "$sock"
}
