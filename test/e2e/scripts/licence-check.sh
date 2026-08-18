#!/usr/bin/env bash
# Recover a Business Edition licence stranded by a run that crashed before its
# own teardown could release it (down.sh and k3d-down.sh release on every
# clean path; nothing releases one after a hard kill mid-suite).
#
# Starts a throwaway Business Edition server — never the shared estate — on
# its own container and port, attaches the licence to it, then immediately
# releases it and confirms the release with a GET /licenses that must now
# come back empty. Safe to run whether or not anything is actually stranded:
# an already-clean licence attaches to, and detaches from, the throwaway
# server exactly the same way. The throwaway container is destroyed on every
# exit path, success or failure.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_root=$(cd ../.. && pwd)
source ./scripts/lib.sh

licence=$(read_licence "$repo_root")
if [[ -z "$licence" ]]; then
    echo "no PORTAINER_LICENSE in $repo_root/.env: nothing to recover" >&2
    exit 1
fi

port="${E2E_RECOVER_PORT:-19098}"
name="portainer-mcp-e2e-licence-recovery"

# In case a previous, interrupted recovery attempt left this container behind.
docker rm -f "$name" >/dev/null 2>&1 || true

cleanup() {
    docker stop "$name" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run -d --rm --name "$name" -p "127.0.0.1:${port}:9000" \
    portainer/portainer-ee:2.44.0 --no-setup-token >/dev/null

# The licence never touches an argument or an echo: it travels from
# read_licence straight into the environment of the one process that needs
# it, exactly like up.sh and down.sh.
PORTAINER_E2E_RECOVER_URL="http://127.0.0.1:${port}" \
PORTAINER_E2E_LICENCE="$licence" \
    go run ./harness/cmd/provision -recover-licence

# A stale lock is the other half of the same accident this script already
# recovers from: a run that crashed mid-suite can leave both a stranded
# licence AND a lock naming a leg that is no longer running. Clearing only
# the licence would leave the next `make e2e-up` refusing for a licence that
# is now free.
#
# The round trip above does NOT, by itself, prove that: harness.ApplyLicence
# returns a conflicting activation as DATA (a warning logged by
# logConflictingKeys), never as a failure, so attaching the licence to this
# throwaway server would succeed even while some other server -- including
# the real estate this lock is supposed to be protecting -- still holds it
# too. And the post-release GET /licenses check afterward only reads back
# THIS throwaway server; it says nothing about any other. So the lock is
# still only cleared once its own recorded holder does not actually look
# like it is running -- the identical check take_licence_lock's own refusal
# path uses, in the direction where being wrong is harmless (leaving a lock
# in place costs an extra manual look; deleting a live leg's lock is the
# accident this whole task exists to prevent). This also protects against
# DOCKER_HOST pointing at the wrong daemon when this script happens to run
# with a stale export from an unrelated remote session: if that made the
# holder look absent when it is not, this still only warns rather than
# deletes.
lock_path=$(licence_lock_path "$repo_root")
if [[ -f "$lock_path" ]]; then
    lock_holder=$(grep -E '^HOLDER=' "$lock_path" 2>/dev/null | head -n1 | cut -d= -f2-) || true
    if licence_lock_holder_running "$lock_holder"; then
        echo "warning: $lock_path still names '$lock_holder', and '$lock_holder' appears to be running right now; leaving the lock in place rather than risk clearing a leg that is genuinely still using the licence. If you are certain it is stale, remove it by hand: rm -f $lock_path" >&2
    else
        rm -f "$lock_path"
        echo "cleared the stale licence lock at $lock_path" >&2
    fi
fi

echo "licence recovery complete: safe to run make e2e-up again" >&2
