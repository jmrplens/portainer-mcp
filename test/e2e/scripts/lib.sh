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

# licence_lock_path echoes the path to the licence lock: the artefact that
# records which leg (compose or kubernetes) currently holds the single-use
# Business Edition licence, so a second leg refuses to activate the same key
# instead of colliding with the first. Rooted at repo_root, like
# read_licence's own .env, rather than $PWD -- take_licence_lock and
# release_licence_lock are called from both up.sh (cwd test/e2e) and
# k3d-up.sh/k3d-down.sh (also cwd test/e2e today, but nothing here should
# depend on that staying true), and a path anchored on the one thing every
# caller already resolved identically avoids yet another place the two
# legs could quietly disagree.
licence_lock_path() {
    local repo_root="$1"
    echo "$repo_root/test/e2e/.licence.lock"
}

# licence_lock_holder_running reports, via a THREE-way exit status, whether
# the leg named in an existing lock is actually still running right now --
# separate from anything the lock file itself claims:
#
#   0  running       the check ran cleanly and found the leg up
#   1  not running   the check ran cleanly and found nothing
#   2  unknown       the check itself could not run at all
#
# The distinction between 1 and 2 is the whole point, and callers MUST NOT
# collapse it back into a plain zero/non-zero test. "Unknown" is genuinely
# reachable, not a theoretical edge case: a Docker daemon that is briefly
# unreachable, `DOCKER_HOST` pointed at a different machine than the one the
# lock's holder is actually running on (see take_licence_lock's own callers
# for exactly this scenario on the remote path), or `k3d` simply not being on
# PATH, all make it impossible to tell "genuinely gone" apart from "cannot
# see it from here" -- and those two must never read the same to a caller
# deciding whether to delete something. Reported this way rather than, say,
# writing to a global or a second output stream, because bash's own exit
# status is already a first-class three-or-more-way channel every caller in
# this file already knows how to read without a second mechanism to keep in
# sync.
#
# take_licence_lock uses this only to choose which of three refusal messages
# to print -- it never deletes anything regardless of the answer. But
# licence-check.sh's cleanup (see its own comment) uses it to gate an actual
# `rm -f`, so a caller that quietly treated 2 as "confirmed absent" would
# turn "the check is unreliable right now" into "the check said clear it".
#
# The compose leg is matched on the compose project label rather than a
# container name substring: `docker ps --filter name=portainer-mcp-e2e`
# (what docs/domain-wave-checklist.md uses for a human-read report) also
# matches the Kubernetes leg's own node containers, which k3d names
# k3d-portainer-mcp-e2e-server-0 -- a substring match here would report the
# compose leg as running because the OTHER leg is up, which is exactly
# backwards for a check whose only job is telling the two apart.
licence_lock_holder_running() {
    local leg="$1"
    case "$leg" in
        compose)
            local out
            # docker's own exit status is read directly, not folded into the
            # grep below: a daemon that cannot be reached at all ("Cannot
            # connect to the Docker daemon...") must report unknown (2), not
            # be indistinguishable from a daemon that answered cleanly with
            # an empty list (not running, 1).
            if ! out=$(docker ps --filter "label=com.docker.compose.project=portainer-mcp-e2e" \
                    --format '{{.Names}}' 2>/dev/null); then
                return 2
            fi
            [[ -n "$out" ]] && return 0
            return 1
            ;;
        kubernetes)
            local cluster="${E2E_K3D_CLUSTER:-portainer-mcp-e2e}" out
            # k3d simply missing from PATH is unknown, not "not running": it
            # says nothing about whether a cluster exists, only that this
            # machine cannot ask. The original `command -v k3d && ...` chain
            # folded that into the same false as an empty cluster list,
            # which is exactly the collapse this rewrite exists to undo.
            command -v k3d >/dev/null 2>&1 || return 2
            if ! out=$(k3d cluster list -o json 2>/dev/null); then
                return 2
            fi
            printf '%s\n' "$out" | grep -q "\"name\":\"$cluster\"" && return 0
            return 1
            ;;
        *)
            # An unrecognised leg (a blank or garbled HOLDER, reachable only
            # via an interrupted write -- see take_licence_lock's own "|| true"
            # comment) is not "unknown": no leg by that name is ever a
            # candidate to be running, checked or not, so this can be
            # answered with confidence rather than punting to 2.
            return 1
            ;;
    esac
}

# licence_lock_resolve_command echoes the exact command that frees the named
# leg's licence, so a refusal can tell the operator precisely what to run
# instead of just what is wrong.
licence_lock_resolve_command() {
    local leg="$1"
    case "$leg" in
        kubernetes) echo "make e2e-k8s-down" ;;
        *)          echo "make e2e-down" ;;
    esac
}

# take_licence_lock refuses (returns 1, prints why to stderr) when the lock
# already names a holder, otherwise records this leg as the new holder and
# returns 0. Callers take it only when a licence is actually in play -- a
# Community-only run reads none and must neither be blocked by a lock nor
# create one -- and take it BEFORE activating anything, so a refusal costs
# nothing.
#
# A held lock is never auto-cleared here, whether or not
# licence_lock_holder_running says the holder looks gone: it only reports the
# distinction, so the operator decides, and `make e2e-licence-release`
# (licence-check.sh) is the one path allowed to actually remove a lock it did
# not itself just take, because that path first confirms -- via a live
# attach-then-release round trip, AND its own check of whether the recorded
# holder still looks running -- that nothing genuinely holds the licence any
# more, rather than inferring it from a process list alone.
#
# The write is attempted FIRST, under `set -o noclobber`, rather than tested
# with a separate `[[ -f ]]` and written afterward: noclobber's ">" opens
# with O_EXCL, so the existence check and the write are one atomic kernel
# call. Two runs racing to take the lock at the same instant cannot both see
# "absent" and both proceed to write -- the loser's redirection fails
# outright instead of silently overwriting the winner's file, which a
# test-then-write pair, however narrow the window, cannot promise. noclobber
# is scoped to the subshell below so it never leaks into the caller's own
# shell options.
take_licence_lock() {
    local repo_root="$1" leg="$2" lock_path estate_file
    lock_path=$(licence_lock_path "$repo_root")
    estate_file="${PORTAINER_E2E_ESTATE:-$PWD/.estate.json}"

    if (
        set -o noclobber
        {
            echo "HOLDER=$leg"
            echo "TAKEN_AT=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
            echo "ESTATE=$estate_file"
        } > "$lock_path"
    ) 2>/dev/null; then
        return 0
    fi

    # The write above failed. Overwhelmingly that means the lock already
    # exists -- the ordinary refusal below -- though the identical failure
    # would also occur if $lock_path's directory were unwritable for some
    # other reason; this function cannot tell the two apart, and neither
    # could the `[[ -f ]]` test this replaces (the file existing right up
    # until immediately before the write is exactly the read that test
    # performed). The three fields are read with "|| true": a lock file that
    # exists but is missing or blank on one line (reachable only via a write
    # interrupted mid-way) must not make this whole function -- and the
    # caller's `set -e` script -- abort on a grep that legitimately found no
    # match, the same reason read_env_var's own pipeline ends the same way.
    local holder taken_at holder_estate resolve_cmd
    holder=$(grep -E '^HOLDER=' "$lock_path" 2>/dev/null | head -n1 | cut -d= -f2-) || true
    taken_at=$(grep -E '^TAKEN_AT=' "$lock_path" 2>/dev/null | head -n1 | cut -d= -f2-) || true
    holder_estate=$(grep -E '^ESTATE=' "$lock_path" 2>/dev/null | head -n1 | cut -d= -f2-) || true
    resolve_cmd=$(licence_lock_resolve_command "$holder")

    # Three-way, not if/else: licence_lock_holder_running's own doc names the
    # exact accident collapsing 1 (confirmed not running) and 2 (could not
    # tell) back into a plain truthy test would cause here -- this function
    # never deletes anything either way, but it must not ASSERT staleness,
    # or point at `make e2e-licence-release` as though the lock were known
    # stale, on an answer that may mean nothing of the kind.
    local holder_status=0
    licence_lock_holder_running "$holder" || holder_status=$?
    case "$holder_status" in
        0)
            echo "refusing to activate the business edition licence for '$leg': $lock_path is already held by '$holder' (taken $taken_at, estate $holder_estate). The licence permits exactly one instance at a time. Run '$resolve_cmd' first, then retry." >&2
            ;;
        1)
            echo "refusing to activate the business edition licence for '$leg': $lock_path names '$holder' (taken $taken_at, estate $holder_estate), but '$holder' does not appear to be running right now. This lock is reported as stale, not removed automatically -- the running check above can itself be wrong, and silently clearing it is how two live instances happen again. If you are certain nothing is really using the licence, run 'make e2e-licence-release': it clears the stranded licence and this stale lock together." >&2
            ;;
        *)
            echo "refusing to activate the business edition licence for '$leg': $lock_path names '$holder' (taken $taken_at, estate $holder_estate); whether '$holder' is still running could not be determined right now (the docker/k3d check itself failed -- check DOCKER_HOST and that docker/k3d are reachable from here). An unreliable check is not evidence the holder is gone, so this is treated as still held, not as stale: 'make e2e-licence-release' would delete this lock outright, so do not run it on this basis alone. Fix the check and retry, or confirm by hand that '$holder' is genuinely gone before clearing the lock yourself." >&2
            ;;
    esac
    return 1
}

# release_licence_lock removes the lock this leg took, and is itself
# tolerant of the two ways there can be nothing sensible to remove: no lock
# file at all (nothing ever took it, or a run that never reached the point
# of taking one -- e.g. a Community-only run, or a crash before the licence
# was even read), or a lock recorded for a DIFFERENT leg (this leg's own
# teardown must never remove a lock it does not own). Both are warned, never
# failed: down.sh and k3d-down.sh call this on every path that reaches their
# own licence release, including the path where that release itself failed,
# and a teardown that aborts on a missing or foreign lock would leave the
# estate it was tearing down still up.
release_licence_lock() {
    local repo_root="$1" leg="$2" lock_path
    lock_path=$(licence_lock_path "$repo_root")

    if [[ ! -f "$lock_path" ]]; then
        echo "warning: no licence lock at $lock_path to release for '$leg'; continuing" >&2
        return 0
    fi

    # "|| true": see take_licence_lock's own doc -- a lock file that exists
    # but has a missing or blank HOLDER line (an interrupted write) must read
    # as holder="" here, never abort this function outright.
    local holder
    holder=$(grep -E '^HOLDER=' "$lock_path" 2>/dev/null | head -n1 | cut -d= -f2-) || true
    if [[ "$holder" != "$leg" ]]; then
        echo "warning: licence lock at $lock_path is held by '$holder', not '$leg'; leaving it in place" >&2
        return 0
    fi

    rm -f "$lock_path"
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
#
# All three functions take an optional leg argument. Each leg — the compose
# estate, the Kubernetes cluster k3d-up.sh brings up — can end up on a
# different machine (today only the compose leg has remote wiring at all;
# Task 7 adds it to the Kubernetes one), so one shared marker cannot describe
# both without one leg's record clobbering the other's the moment they
# differ. The default (no argument, or an empty string) is today's compose
# marker, at today's path, so every existing caller and test is unaffected.
# A named leg gets its own file instead — docker_host_marker "kubernetes" ->
# ".docker-host-kubernetes" — never the default ".docker-host".
#
# PORTAINER_E2E_DOCKER_HOST_FILE overrides only the default marker's path,
# deliberately not a named leg's. It exists so a test can point the compose
# marker at a scratch file without touching the real one under $PWD; the
# same override applying to every leg would collapse all of them back onto
# the single file this split exists to get away from, which defeats the
# point for exactly the multi-leg case it would be invoked for. A leg-scoped
# override can be added if a real need for one ever shows up; none has yet.
docker_host_marker() {
    local leg="${1:-}"
    if [[ -z "$leg" ]]; then
        echo "${PORTAINER_E2E_DOCKER_HOST_FILE:-$PWD/.docker-host}"
    else
        echo "$PWD/.docker-host-$leg"
    fi
}

record_docker_host() {
    local dest="$1" leg="${2:-}" marker
    marker=$(docker_host_marker "$leg")
    if [[ -z "$dest" ]]; then
        rm -f "$marker"
    else
        printf '%s\n' "$dest" > "$marker"
    fi
}

recorded_docker_host() {
    local leg="${1:-}" marker
    marker=$(docker_host_marker "$leg")
    [[ -f "$marker" ]] || return 0
    head -n1 "$marker"
}

# refuse_docker_host_switch dies with a clear message when leg's marker
# already names a destination different from dest, the one this run is about
# to use. Absence of an existing marker is never a mismatch -- it means no
# earlier run recorded anything for this leg, so the first run (local or
# remote) is always free to proceed and record. Re-running against the SAME
# destination an existing marker already names is also not a mismatch: this
# guard's own job is telling a host SWITCH apart from a same-destination
# re-run, and a second `make e2e-up-remote` against the same host is the
# latter, not the former -- regardless of whether that re-run goes on to
# succeed. up.sh's own header now qualifies its "idempotent" claim to
# Community Edition only: a licensed re-run can still refuse a moment later,
# at take_licence_lock, for an entirely different reason (the licence, not
# the host) than anything this function checks.
#
# Call this before record_docker_host, never after: record_docker_host with an
# empty destination DELETES the marker (see its own doc), and up.sh/k3d-up.sh
# both call it unconditionally on every run. Without this guard, a plain
# `make e2e-up` typed after `make e2e-up-remote` would resolve ssh_dest to
# empty (make e2e-up never sets PORTAINER_E2E_REMOTE) and record_docker_host
# would delete the ONLY record of where the earlier remote estate, its
# Business Edition licence and its open ssh master actually are. `make
# e2e-down` then reads no marker, tears down locally -- where nothing exists
# -- and reports success while the real estate, licence and tunnel are all
# still running, unreachable, on somebody else's machine.
refuse_docker_host_switch() {
    local dest="$1" leg="${2:-}" existing
    existing=$(recorded_docker_host "$leg")
    [[ -n "$existing" ]] || return 0
    [[ "$existing" == "$dest" ]] && return 0
    local legname="${leg:-compose}" marker
    marker=$(docker_host_marker "$leg")
    echo "refusing to continue: $marker already names '$existing' from an earlier, still-recorded $legname run, but this run would use '${dest:-the local docker daemon}'. Tear that estate down first with its own matching teardown target, or remove the marker by hand if you are certain it is stale and nothing is really running there." >&2
    return 1
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

# cdi_spec_path echoes the fixed location on the Docker host where the CDI
# specification for the GPU is written. up.sh writes it there, mounts it into
# the compose GPU override (PORTAINER_E2E_CDI_SPEC), and down.sh both looks
# for it (to decide whether the GPU compose override was ever in play) and
# removes it during teardown.
#
# It is the only file this estate ever writes on the Docker host outside the
# compose project itself, so up.sh and down.sh disagreeing on its path — even
# briefly, during a refactor — would leave a stale specification behind on
# whatever host runs the estate, undetected by anything that only looks
# inside the compose project. Before this function existed the same literal
# was duplicated in both scripts (and, for documentation purposes only, in
# docs/domain-wave-checklist.md's manual verification snippet, which cannot
# source this file and must be kept matching by hand).
cdi_spec_path() {
    echo "/tmp/portainer-mcp-e2e-cdi-nvidia.yaml"
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
# There is deliberately no "| head -n1" in the command string, and therefore
# no "pipefail" either — an earlier version had both, and it was wrong. On a
# multi-GPU host nvidia-smi writes one line per card; "head -n1" reads its
# first line and exits, closing the pipe, and if nvidia-smi is still writing
# its *next* line at that moment it gets SIGPIPE and dies with 141. With
# pipefail that 141 becomes the pipeline's exit status, "if !" reads it as
# failure, and a host with a GPU is reported as having none. Measured: a
# two-line stub with a few milliseconds between writes (roughly what
# separate GPU query rows cost for real) failed 200/200 sequential runs with
# pipefail set and 0/200 with it unset — this is not a rare race, it is the
# common case for anything but a single-GPU host. Fixed by letting the whole
# remote command finish and taking the first line locally instead, where
# there is no consumer to close the pipe out from under a still-writing
# producer.
detect_gpu_name() {
    local dest="$1" raw
    if ! raw=$(on_docker_host "$dest" \
        'command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi --query-gpu=name --format=csv,noheader 2>/dev/null' \
        2>/dev/null); then
        return 0
    fi
    [[ -n "$raw" ]] || return 0
    printf '%s\n' "${raw%%$'\n'*}"
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
#
# The local pipe into strip_cdi_hooks is captured the same way, for the same
# reason: "printf ... | strip_cdi_hooks" run bare would let a broken awk (the
# filter's only dependency) escape as this function's own exit status, and
# under a caller's set -euo pipefail that kills the whole script — the one
# failure mode every other path in both GPU functions was already closed
# against. Verified directly: shadowing awk with a stub that exits 127, with
# nvidia-ctk otherwise succeeding, made the bare pipe form propagate 127 and
# a statement placed right after the call never ran.
gpu_cdi_spec() {
    local dest="$1" raw stripped
    if ! raw=$(on_docker_host "$dest" \
        'command -v nvidia-ctk >/dev/null 2>&1 && nvidia-ctk cdi generate --format=yaml 2>/dev/null' \
        2>/dev/null); then
        return 0
    fi
    [[ -n "$raw" ]] || return 0
    if ! stripped=$(printf '%s\n' "$raw" | strip_cdi_hooks); then
        return 0
    fi
    printf '%s\n' "$stripped"
}

# swarm_fixture_service_name is the fixed name of the Swarm service up.sh
# creates as a fixture, so Swarm-dependent catalog actions
# (docker.service_image_status today; nodes/tasks/services in later waves)
# have something real to exercise instead of the CE dind's ordinary
# standalone-engine failure. Prefixed like the estate's own compose project
# (docker-compose.yml's "name: portainer-mcp-e2e"), so a human scanning
# `docker service ls` inside the dind, or a future orphan sweep, can
# recognise it as belonging to this estate rather than to whatever a test
# created on top of it.
swarm_fixture_service_name() {
    echo "portainer-mcp-e2e-swarm-probe"
}

# swarm_init makes dind_id -- the container id of the estate's own
# Docker-in-Docker daemon -- a one-node Swarm. Unlike the GPU functions above
# this never needs on_docker_host/ssh: `docker exec` is a Docker Engine API
# call against a container the compose project already started, and the
# `docker` CLI already routes it to the right daemon through whatever
# DOCKER_HOST up.sh has exported (empty for local, ssh://... for remote) --
# there is no separate physical-host shell command to reach the way
# nvidia-smi/nvidia-ctk need.
#
# Idempotent: a node that already belongs to a swarm is treated as success,
# matched on Docker's own fixed error text ("already part of a swarm")
# rather than probed for first with a second command that could itself fail.
# This matters because the dind keeps its Swarm state across compose's own
# idempotent `up -d --wait` -- nothing about that state lives outside the
# dind container's own writable layer, so it survives a second `make e2e-up`
# run with no intervening `make e2e-down` -- and without this check that
# second run would abort the whole estate on "this node is already part of a
# swarm", exactly the failure mode the brief calls out by name.
#
# --advertise-addr 127.0.0.1 is safe specifically because this Swarm has,
# and will only ever have, this one node: the dind is reachable solely on
# the compose network (docker-compose.yml publishes no port for it), so
# there is no second node that could ever dial that address to join.
#
# Any OTHER failure -- Swarm mode unsupported on this daemon, or refused for
# a reason this script cannot anticipate -- is reported to stderr and
# returned as a plain (non-fatal) failure, the same degrade-and-continue
# shape detect_gpu_name already uses: a host where Swarm cannot be enabled
# must still let the rest of the estate come up, with the Swarm-dependent
# suites simply skipping instead of the whole `make e2e-up` aborting over
# one optional leg.
swarm_init() {
    local dind_id="$1" out
    if out=$(docker exec "$dind_id" docker swarm init --advertise-addr 127.0.0.1 2>&1); then
        return 0
    fi
    if [[ "$out" == *"already part of a swarm"* ]]; then
        return 0
    fi
    echo "warning: could not initialise docker swarm on the estate's docker daemon; continuing without it: $out" >&2
    return 1
}

# swarm_fixture_service_id ensures swarm_fixture_service_name exists as a
# service on dind_id (already made a Swarm manager by swarm_init) and echoes
# its id -- Swarm's own alphanumeric service id, read back from Docker
# itself rather than assigned by this script (see docs/api-divergences.md's
# "The cheat this is written down to forbid": a hand-labelled small integer
# would make docker.service_image_status look correct without actually being
# correct). Echoes nothing and returns non-zero if the service could not be
# created or its id could not be confirmed.
#
# Idempotent like swarm_init: a service that already exists (a second
# `make e2e-up` run with no intervening `make e2e-down`) has its existing id
# read back and reused, rather than `docker service create` failing outright
# on Docker's own "name conflicts with an existing object".
#
# `busybox sleep 3600` is deliberately long-lived and does nothing: a
# Swarm-dependent action needs a service that is still there, with its one
# task still running, whenever a test calls it later -- not one whose task
# ran to completion and left the service converged at zero running
# replicas.
#
# --detach skips waiting for the new service to converge, which this
# function does not need: the e2e suite calls docker.service_image_status
# with refresh:true, which forces a live `docker service inspect` and
# succeeds once the service is registered with Swarm, whether or not its
# replica has finished converging (see docs/api-divergences.md section 2.4:
# the live check fails with "service ... not found" only once the service
# record itself is gone, never on an unconverged replica) -- so waiting here
# would only slow every `make e2e-up` down for a property nothing checks.
swarm_fixture_service_id() {
    local dind_id="$1" name id out
    name=$(swarm_fixture_service_name)

    if id=$(docker exec "$dind_id" docker service inspect "$name" --format '{{.ID}}' 2>/dev/null) && [[ -n "$id" ]]; then
        echo "$id"
        return 0
    fi

    if ! out=$(docker exec "$dind_id" docker service create --detach --name "$name" --replicas 1 busybox sleep 3600 2>&1); then
        echo "warning: could not create the swarm fixture service $name; continuing without it: $out" >&2
        return 1
    fi

    if ! id=$(docker exec "$dind_id" docker service inspect "$name" --format '{{.ID}}' 2>/dev/null) || [[ -z "$id" ]]; then
        echo "warning: swarm fixture service $name was created but its id could not be read back" >&2
        return 1
    fi
    echo "$id"
}
