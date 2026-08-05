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
source ./scripts/lib.sh
source ./scripts/remote.sh

estate_file="${PORTAINER_E2E_ESTATE:-$PWD/.estate.json}"
edge_env="${PORTAINER_E2E_EDGE_ENV:-$PWD/.edge.env}"

# Where the estate's Docker daemon lives. Empty means this machine, which is
# what `make e2e-up` always gets: docker_ssh_dest answers non-empty only when
# PORTAINER_E2E_REMOTE=1, which only `make e2e-up-remote` sets.
ssh_dest=$(docker_ssh_dest "$repo_root")
if [[ -n "$ssh_dest" ]]; then
    export DOCKER_HOST="ssh://$ssh_dest"
    echo "running the estate on $ssh_dest via $DOCKER_HOST" >&2
fi
# Refuses rather than silently orphaning a still-running estate this run would
# otherwise stop being able to find: see refuse_docker_host_switch's own doc
# for the plain-`make e2e-up`-after-`make e2e-up-remote` scenario this exists
# to catch. Must run before record_docker_host, which unconditionally deletes
# the marker on an empty destination.
refuse_docker_host_switch "$ssh_dest"
# Recorded before anything is created, not after: a run that dies half way
# through still has to be tearable down against the right daemon.
record_docker_host "$ssh_dest"

# The Business Edition licence lives in a gitignored .env at the repository
# root. Its absence is not an error: the Community Edition legs still run, and
# the estate records that Business Edition is unavailable so suites skip.
licence=$(read_licence "$repo_root")
if [[ -z "$licence" ]]; then
    echo "no PORTAINER_LICENSE in .env: provisioning Community Edition only" >&2
fi

# A GPU on the Docker host is an optional capability, discovered the same way
# the licence is: absent means the GPU suites skip, never that the estate
# fails. When present, the dind needs two things — the card itself, and a CDI
# specification the daemon *inside* it can use to pass that card on to a
# nested container. See docker-compose.gpu.yml.
compose_files=(-f docker-compose.yml)
gpu_name=$(detect_gpu_name "$ssh_dest")
gpu_cdi_device=""
cdi_spec_path=""
if [[ -n "$gpu_name" ]]; then
    cdi_spec_path="/tmp/portainer-mcp-e2e-cdi-nvidia.yaml"
    # `test -s` is not enough on its own: a generator that dies part-way
    # through leaves a non-empty file that passes it. Check the document is
    # actually shaped like a CDI specification — both keys, at the top level —
    # so a truncated one is rejected here rather than mounted into the dind
    # and breaking every nested GPU container silently.
    if gpu_cdi_spec "$ssh_dest" | write_to_docker_host "$ssh_dest" "$cdi_spec_path" \
       && on_docker_host "$ssh_dest" "test -s '$cdi_spec_path' && grep -q '^cdiVersion:' '$cdi_spec_path' && grep -q '^kind:' '$cdi_spec_path'"; then
        gpu_cdi_device=$(cdi_device_id)
        compose_files+=(-f docker-compose.gpu.yml)
        export PORTAINER_E2E_CDI_SPEC="$cdi_spec_path"
        echo "gpu detected on the docker host: $gpu_name (offering $gpu_cdi_device)" >&2
    else
        # A GPU with no usable CDI specification is not a GPU the estate can
        # offer. Say so and carry on GPU-less rather than starting a dind that
        # would fail every nested GPU container at creation time.
        echo "warning: $gpu_name present but no usable CDI specification could be generated (absent, empty or not shaped like one); continuing without GPU" >&2
        on_docker_host "$ssh_dest" "rm -f '$cdi_spec_path'" 2>/dev/null || true
        gpu_name=""
        cdi_spec_path=""
    fi
else
    echo "no GPU on the docker host: GPU suites will skip" >&2
fi

# --wait-timeout is 300 rather than 120 because a remote Docker host pulling
# docker:28-dind, both Portainer images and the agent for the first time
# routinely needs more than two minutes, and the failure mode is an
# indistinguishable "unhealthy" timeout.
docker compose "${compose_files[@]}" up -d --wait --wait-timeout 300

# Portainer is published on the *daemon host's* loopback (see
# docker-compose.yml's 127.0.0.1 bindings). When that host is remote, forward
# both ports here so the suite's URLs stay http://localhost:19000 and
# http://localhost:19001 exactly as they are locally.
tunnel_up "$ssh_dest" "${E2E_CE_PORT:-19000}" "${E2E_EE_PORT:-19001}"

PORTAINER_E2E_ESTATE="$estate_file" \
PORTAINER_E2E_EDGE_ENV="$edge_env" \
PORTAINER_E2E_LICENCE="$licence" \
PORTAINER_E2E_GPU_NAME="$gpu_name" \
PORTAINER_E2E_GPU_CDI_DEVICE="$gpu_cdi_device" \
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
#
# The provisioner writes this file itself, alongside the estate, once it
# knows these values; compose consumes KEY=value natively via --env-file, so
# nothing here has to parse the estate's JSON to reach three strings out of
# it. The estate file stays the canonical record of what was provisioned —
# this is a derived artefact for the shell, and the provisioner removes it
# when no edge environment exists, so a stale file from an earlier run cannot
# enrol an agent against a server that is gone.
if [[ -f "$edge_env" ]]; then
    docker compose "${compose_files[@]}" --env-file "$edge_env" --profile edge up -d edge
    endpoint_id=$(grep '^EDGE_ENDPOINT_ID=' "$edge_env" | cut -d= -f2-)
    echo "edge agent starting for endpoint $endpoint_id" >&2
else
    echo "no edge endpoint provisioned: skipping the edge agent" >&2
fi

echo "estate ready: $estate_file" >&2
