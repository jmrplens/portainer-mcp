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

# The edge agent (profile "edge", so the plain `up` above skipped it) needs
# EDGE_ID and EDGE_KEY, and neither exists until the provisioner has
# registered the environment and the server has issued them — there is no way
# to know them before this point. Present only when a licence was supplied:
# edge domains are Business Edition only.
#
# EDGE_ID is edge_agent_id (a UUID), not edge_endpoint_id (Portainer's
# ordinary numeric database id for the same environment): the two are easy to
# conflate and only one of them is what the agent's EDGE_ID variable means.
# Passing the numeric id here fails distinctively — the agent polls and the
# server answers "invalid Edge identifier" — not silently, but it still isn't
# what a reader skimming for "the id" would reach for.
endpoint_id=$(python3 -c "import json; print(json.load(open('$estate_file')).get('edge_endpoint_id') or '')")
edge_id=$(python3 -c "import json; print(json.load(open('$estate_file')).get('edge_agent_id') or '')")
edge_key=$(python3 -c "import json; print(json.load(open('$estate_file')).get('edge_key') or '')")
if [[ -n "$edge_id" && -n "$edge_key" ]]; then
    EDGE_ID="$edge_id" EDGE_KEY="$edge_key" docker compose --profile edge up -d edge
    echo "edge agent starting for endpoint $endpoint_id (edge id $edge_id)" >&2
else
    echo "no edge endpoint provisioned: skipping the edge agent" >&2
fi

echo "estate ready: $estate_file" >&2
