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
# or a second `make e2e-down`. It needs no special handling for a forward
# tunnel_add_forward added after tunnel_up ran: `-O exit` terminates the
# whole master process, which drops every forward it was carrying, however
# many were added and whenever — measured directly.
tunnel_down() {
    local dest="$1"
    [[ -n "$dest" ]] || return 0
    local sock
    sock=$(tunnel_socket_path)
    ssh -S "$sock" -O exit "$dest" 2>/dev/null || true
    rm -f "$sock"
}

# tunnel_add_forward requests one additional local forward on the master
# tunnel_up already opened, and confirms it is actually live before
# returning. A no-op when the destination is empty, matching tunnel_up and
# tunnel_down.
#
# Its remote-side target is not necessarily the destination's own loopback,
# unlike tunnel_up's own forwards: it exists specifically to reach an
# address only the destination itself can route to (for example, a
# container's address on the Docker host's own bridge network, which the
# Docker host reaches directly and this machine cannot) — a shape
# tunnel_up's "mirror this port to the remote's 127.0.0.1" forwards cannot
# express.
#
# The confirmation is not optional. `ssh -O forward` does not behave like
# `ssh -M ... -o ExitOnForwardFailure=yes`: measured directly, binding the
# wanted local port out from under a running master first, `ssh -S sock -O
# forward -L <that port>:...` for the same port still exits 0. Trusting
# that exit code would reintroduce, one layer up, exactly the silent
# failure ExitOnForwardFailure exists to close for the master's own initial
# ports — the caller would carry on believing a forward exists that does
# not, and whatever tries to use it next would fail somewhere else,
# unrecognisably. So this function polls the local port itself with a
# plain bash TCP connect (`/dev/tcp`, no external dependency) and only
# returns success once something actually answers there.
#
# PORTAINER_E2E_TUNNEL_FORWARD_RETRIES overrides how many 0.2s polls are
# attempted (default 20, ~4s) so a test asserting the failure path is not
# made to wait the full real-world budget for it.
tunnel_add_forward() {
    local dest="$1" local_port="$2" remote_host="$3" remote_port="$4"
    [[ -n "$dest" ]] || return 0
    local sock retries tries=0
    sock=$(tunnel_socket_path)
    retries="${PORTAINER_E2E_TUNNEL_FORWARD_RETRIES:-20}"
    ssh -S "$sock" -O forward -L "${local_port}:${remote_host}:${remote_port}" "$dest" >/dev/null 2>&1
    while (( tries < retries )); do
        if (exec 3<>"/dev/tcp/127.0.0.1/${local_port}") 2>/dev/null; then
            echo "tunnel forward added to $dest: 127.0.0.1:${local_port} -> ${remote_host}:${remote_port}" >&2
            return 0
        fi
        sleep 0.2
        tries=$((tries + 1))
    done
    echo "could not confirm the forward from 127.0.0.1:${local_port} to ${remote_host}:${remote_port} on $dest -- the local port may already be in use, or $dest cannot reach ${remote_host}:${remote_port}" >&2
    return 1
}
