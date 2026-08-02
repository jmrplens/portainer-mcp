#!/usr/bin/env bash
# Bring up the ephemeral Portainer estate and provision it.
#
# Idempotent: running it twice replaces the estate rather than accumulating
# one. Safe by construction: every resource is named under the compose project
# portainer-mcp-e2e, so nothing here can touch a Portainer the developer
# already runs.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_root=$(cd ../.. && pwd)

estate_file="${PORTAINER_E2E_ESTATE:-$PWD/.estate.json}"

# The Business Edition licence lives in a gitignored .env at the repository
# root. Its absence is not an error: the Community Edition legs still run, and
# the estate records that Business Edition is unavailable so suites skip.
licence=""
if [[ -f "$repo_root/.env" ]]; then
    licence=$(grep -E '^PORTAINER_LICENSE=' "$repo_root/.env" | cut -d= -f2- | tr -d '"'"'"'' || true)
fi
if [[ -z "$licence" ]]; then
    echo "no PORTAINER_LICENSE in .env: provisioning Community Edition only" >&2
fi

docker compose up -d --wait --wait-timeout 120

PORTAINER_E2E_ESTATE="$estate_file" \
PORTAINER_E2E_LICENCE="$licence" \
    go run ./harness/cmd/provision

echo "estate ready: $estate_file" >&2
