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

echo "licence recovery complete: safe to run make e2e-up again" >&2
