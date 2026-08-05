#!/usr/bin/env bash
# Tear the estate down. Safe to run when nothing is up.
#
# The compose EE server applies a Business Edition licence exactly like the
# Kubernetes leg does (see k3d-down.sh), and it must give it back before its
# container is destroyed: once the container is gone the server is
# unreachable and the licence key would be stranded against a real account for
# good. Releasing it is attempted before the compose teardown below and is
# best-effort only — a run that fails to bring the estate down is worse than a
# stranded licence, so nothing here is allowed to abort the script.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."
repo_root=$(cd ../.. && pwd)
source ./scripts/lib.sh
source ./scripts/remote.sh

# Teardown takes no flag. It reads where `up` actually went, so
# `make e2e-up-remote` followed by a plain `make e2e-down` still tears down
# the remote estate instead of leaving it running on somebody else's machine.
ssh_dest=$(recorded_docker_host)
if [[ -n "$ssh_dest" ]]; then
    export DOCKER_HOST="ssh://$ssh_dest"
    echo "tearing down the estate on $ssh_dest" >&2
fi

# The GPU override interpolates PORTAINER_E2E_CDI_SPEC and fails without it,
# so teardown names the same file only when it can supply the variable. The
# path is fixed and known, so it does not have to survive from up.sh.
cdi_spec_path="/tmp/portainer-mcp-e2e-cdi-nvidia.yaml"
compose_files=(-f docker-compose.yml)
if on_docker_host "$ssh_dest" "test -f '$cdi_spec_path'" 2>/dev/null; then
    compose_files+=(-f docker-compose.gpu.yml)
    export PORTAINER_E2E_CDI_SPEC="$cdi_spec_path"
fi

estate_file="${PORTAINER_E2E_ESTATE:-$PWD/.estate.json}"
cluster="${E2E_K3D_CLUSTER:-portainer-mcp-e2e}"

# The documented order is `make e2e-k8s-down` before `make e2e-down` (see
# README.md), so that by the time this script deletes the estate file below,
# the Kubernetes leg has already had its own chance to release its licence
# through that file. Calling `make e2e-down` first, or on its own without
# `make e2e-k8s-down` ever running, would otherwise delete the one file
# k3d-down.sh needs to release a licence attached to a server it is about to
# destroy — and unlike this compose leg, once k3d-down.sh's `k3d cluster
# delete` runs, that server is gone and the licence is unrecoverable in
# place. Detecting a still-running cluster here and tearing it down first,
# regardless of which order the two `make` targets were invoked in, makes
# `make e2e-down` alone safe rather than relying on every caller to remember
# the documented sequence.
#
# This whole probe runs in a subshell with its own DOCKER_HOST, deliberately
# not the compose leg's exported above. k3d-up.sh records the Kubernetes
# leg's OWN destination under its own marker (record_docker_host "$dest"
# "kubernetes"), which can legitimately differ from the compose leg's — local
# Kubernetes alongside a remote compose estate, a different remote host for
# each, or either one alone — but the compose leg above may well have just
# exported DOCKER_HOST=ssh://truenas. Leaving that in place here would point
# `k3d cluster list`, and any k3d-down.sh it triggers, at the compose leg's
# daemon instead of the Kubernetes leg's own recorded one; when the two
# differ, the check would then silently find nothing and skip a real cluster
# still running elsewhere with a Business Edition licence attached to it. Do
# not "simplify" this back to a bare `if`: that silent skip is exactly the bug
# this subshell exists to prevent, and it does not reproduce on any host
# where the compose and Kubernetes legs happen to share the same destination,
# which is precisely why it would go unnoticed in ordinary use. The subshell
# also keeps the compose destination exported above intact for everything
# below this block, which still has to run against it.
#
# The subshell's own exit status is deliberately not left to abort this
# script under set -e: a future command inside k3d-down.sh that is not
# guarded the way its licence release and `k3d cluster delete` already are
# would otherwise take this whole script down with it, and everything below
# -- the compose teardown -- would simply never run, leaving the compose
# estate up on whatever host it lives on. That failure mode is exactly what
# this script's own header calls worse than a stranded licence, so a failed
# Kubernetes teardown is reported and swallowed here instead.
(
    k8s_ssh_dest=$(recorded_docker_host kubernetes)
    if [[ -n "$k8s_ssh_dest" ]]; then
        export DOCKER_HOST="ssh://$k8s_ssh_dest"
    else
        unset DOCKER_HOST
    fi
    if command -v k3d >/dev/null && k3d cluster list -o json 2>/dev/null | grep -q "\"name\":\"$cluster\""; then
        echo "kubernetes leg still up: tearing it down first so its licence can be released before this estate file is removed" >&2
        ./scripts/k3d-down.sh
    fi
) || echo "warning: the kubernetes teardown failed; continuing with the compose teardown" >&2

licence=$(read_licence "$repo_root")
if [[ -n "$licence" && -f "$estate_file" ]]; then
    echo "releasing compose estate's business edition licence" >&2
    PORTAINER_E2E_ESTATE="$estate_file" \
    PORTAINER_E2E_LICENCE="$licence" \
        go run ./harness/cmd/provision -release-licence \
        || echo "warning: could not release the business edition licence; continuing teardown" >&2
fi

# --profile edge: the edge agent only ever runs under that profile (up.sh
# starts it in a second pass, once EDGE_ID/EDGE_KEY exist), and without
# naming the profile here `down` leaves it running.
docker compose "${compose_files[@]}" --profile edge down -v --remove-orphans
rm -f "$estate_file"
rm -f "${PORTAINER_E2E_EDGE_ENV:-$PWD/.edge.env}"

# Everything this estate put on the Docker host goes back. The CDI
# specification is the only file it writes outside the compose project.
on_docker_host "$ssh_dest" "rm -f '$cdi_spec_path'" 2>/dev/null || true
tunnel_down "$ssh_dest"
# Last, so that everything above ran against the recorded destination.
record_docker_host ""
