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
#
# AddressFamily=inet closes a gap in that same guarantee that ExitOnForward-
# Failure alone does not: when the wanted 127.0.0.1:<port> is already taken,
# ssh's default behaviour is not to fail, but to silently retry the bind on
# ::1 instead and report success once THAT succeeds — measured directly,
# with something else already listening on 127.0.0.1, both a bare `ssh -M`
# and one carrying ExitOnForwardFailure=yes bound [::1]:<port> and exited 0.
# Restricting the connection to IPv4 removes that fallback entirely, so a
# taken v4 port has nowhere left to go and ExitOnForwardFailure's exit
# actually fires. This has to live on the master's own connection, not on
# a later per-forward request: measured separately, passing AddressFamily=
# inet to `ssh -O forward` itself changes nothing, because by that point the
# connection (and the address family it negotiated) already exists — see
# tunnel_add_forward, which depends on this master-level setting to make
# ITS OWN forward requests fail loudly instead of falling back the same way.
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
        -o AddressFamily=inet \
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
# What this function actually guarantees, and what it does not:
#
# GUARANTEED: the local port was not already answering immediately before
# the request (the preflight probe below), and something answers it
# immediately after (the poll below), on 127.0.0.1 specifically — not on
# ::1, because tunnel_up's master is opened with AddressFamily=inet, so
# there is no address family left for the forward to silently land on
# instead. Between those two facts, a successful return is good evidence
# that IT WAS THIS CALL that made the port answer, not a coincidence.
#
# NOT GUARANTEED: that remote_host:remote_port, as seen from $dest, is
# actually what answers through it end to end. A TCP accept on the local
# side happens before ssh has necessarily finished (or even started)
# opening the channel to the remote target, so this cannot distinguish "the
# forward is fully working" from "the forward exists but the remote side
# will fail a moment later" — a distinction the provisioner's own first real
# request against the forwarded URL will surface regardless, the same way
# it would for any other network failure.
#
# Both checks below exist because of two DIFFERENT ways `ssh -O forward`'s
# own exit code lies, both measured directly rather than assumed:
#   1. Without AddressFamily=inet on the master, a taken 127.0.0.1:<port>
#      makes ssh silently retry on ::1 and exit 0 once that succeeds — the
#      previous version of this function trusted that exit code and a poll
#      of 127.0.0.1 would then connect to whatever ELSE was already using
#      that port, reporting success for a forward that does not exist there.
#      AddressFamily=inet on the master (see tunnel_up) closes this: passing
#      it to `-O forward` itself does nothing, since the address family is
#      negotiated once, when the master's own connection is established.
#   2. Even with that fixed, `-O forward` against an ALREADY-OCCUPIED port
#      still exits non-zero — but a plain poll of that port afterwards
#      would still connect to the pre-existing occupant and report success,
#      because ssh never touched it. AddressFamily=inet cannot help here:
#      there is nothing wrong with the connection's address family, the
#      port is just legitimately in use by something unrelated. Only
#      checking BEFORE asking ssh for anything closes this, which is what
#      the preflight probe is for.
#
# PORTAINER_E2E_TUNNEL_FORWARD_RETRIES overrides how many 0.2s polls are
# attempted after the request (default 20, ~4s) so a test asserting the
# failure path is not made to wait the full real-world budget for it.
tunnel_add_forward() {
    local dest="$1" local_port="$2" remote_host="$3" remote_port="$4"
    [[ -n "$dest" ]] || return 0
    local sock retries tries=0
    sock=$(tunnel_socket_path)
    retries="${PORTAINER_E2E_TUNNEL_FORWARD_RETRIES:-20}"

    if (exec 3<>"/dev/tcp/127.0.0.1/${local_port}") 2>/dev/null; then
        echo "cannot forward to 127.0.0.1:${local_port}: something else is already listening there" >&2
        return 1
    fi

    if ! ssh -S "$sock" -O forward -L "${local_port}:${remote_host}:${remote_port}" "$dest" >/dev/null 2>&1; then
        echo "ssh refused to add the forward 127.0.0.1:${local_port} -> ${remote_host}:${remote_port} on $dest (port likely in use, or the master is not running)" >&2
        return 1
    fi

    while (( tries < retries )); do
        if (exec 3<>"/dev/tcp/127.0.0.1/${local_port}") 2>/dev/null; then
            echo "tunnel forward added to $dest: 127.0.0.1:${local_port} -> ${remote_host}:${remote_port}" >&2
            return 0
        fi
        sleep 0.2
        tries=$((tries + 1))
    done
    echo "could not confirm the forward from 127.0.0.1:${local_port} to ${remote_host}:${remote_port} on $dest -- ssh accepted the request but nothing answers yet; $dest may not be able to reach ${remote_host}:${remote_port}" >&2
    return 1
}
