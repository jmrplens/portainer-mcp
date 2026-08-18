<div align="center">

# Portainer MCP Server

**Manage Portainer through AI assistants, over the Model Context Protocol**

![Go Version](https://img.shields.io/github/go-mod/go-version/jmrplens/portainer-mcp)
![License](https://img.shields.io/github/license/jmrplens/portainer-mcp)
![Portainer](https://img.shields.io/badge/Portainer-2.44.0-blue)
![Status](https://img.shields.io/badge/status-in%20development-orange)

[Quickstart](#quickstart) · [Configuration](#configuration) · [Tool surfaces](#tool-surfaces) · [Development](#development)

</div>

---

A [Model Context Protocol](https://modelcontextprotocol.io/introduction) server that connects AI assistants to [Portainer](https://www.portainer.io/). Every action is generated from Portainer's own OpenAPI documents and verified against a real server, so what the tool publishes is what the API actually accepts — not what its documentation claims.

## Status

**In development.** The server runs, but it is being built domain by domain and does not yet cover the whole API.

| | Covered | Total |
|---|---|---|
| Business Edition operations | **35** | 442 |
| Community Edition operations | **27** | 252 |

The totals were 441 and 251 until 2026-08-18, when `cmd/audit_1to1` stopped
skipping routes the vendored documents leave without an `operationId`; see
[§6.2 of the divergence notes](docs/api-divergences.md). The **Covered**
column is stale and understates the real figures — run `make audit-1to1` for
the current ones.

Five domains are live: `system`, `tags`, `registries`, `docker` and `custom_templates` — 36 catalog actions in all. Every one of them is exercised against a disposable Portainer estate on both editions before it ships; see [End-to-end testing](#end-to-end-e2e-testing).

There are no releases, no published container image and no pre-built binaries yet. Build from source.

## Quickstart

### 1. Build

```bash
git clone https://github.com/jmrplens/portainer-mcp.git
cd portainer-mcp
make build          # → dist/portainer-mcp
```

Go 1.26.6 or newer.

### 2. Get a Portainer API token

In Portainer: **My account → Access tokens → Add access token**. The token carries the permissions of the user that created it, so create it as a user with only the access the assistant should have.

### 3. Configure your assistant

Claude Desktop (`claude_desktop_config.json`), and the same shape for any other MCP client:

```json
{
  "mcpServers": {
    "portainer": {
      "command": "/path/to/dist/portainer-mcp",
      "env": {
        "PORTAINER_URL": "https://portainer.example.com",
        "PORTAINER_TOKEN": "ptr_..."
      }
    }
  }
}
```

## Configuration

Flags override environment variables, which override a `.env` file in the working directory.

| Environment variable | Flag | Default | Meaning |
|---|---|---|---|
| `PORTAINER_URL` | `-server` | — | Portainer server URL (required) |
| `PORTAINER_TOKEN` | `-token` | — | API token (required) |
| `PORTAINER_SKIP_TLS_VERIFY` | `-skip-tls-verify` | `false` | Skip TLS verification, for self-signed certificates |
| `TOOL_SURFACE` | `-tool-surface` | `dynamic` | `dynamic`, `meta` or `individual` |
| `PORTAINER_READ_ONLY` | `-read-only` | `false` | Disable every mutating action |
| `PORTAINER_SAFE_MODE` | `-safe-mode` | `false` | Intercept mutating actions and return a preview instead of calling |
| `LOG_LEVEL` | — | `info` | `debug`, `info`, `warn` or `error` |

`-version` prints the build metadata and exits.

**Read-only mode** removes every mutating action from the catalog, so the assistant cannot call one even by mistake. **Safe mode** keeps them callable but answers with a preview of what would be sent — it reports field *names* only, never values, so a credential in a request body is not echoed back to the model.

## Tool surfaces

One action catalog, projected three ways. The default suits most clients; the others exist because tool-count limits and discovery behaviour differ between them.

| Surface | Tools published | Use when |
|---|---|---|
| `dynamic` *(default)* | 2 — `portainer_find_action`, `portainer_execute_action` | Almost always. The model searches the catalog, then calls what it found, so the tool list stays small however far coverage grows. |
| `meta` | one per domain (`portainer_docker`, `portainer_custom_templates`, …), each taking an `action` parameter | A client that discovers tools poorly but handles a modest, fixed list well. |
| `individual` | one per action (`portainer_tags_list`, …) | A client that needs every action visible as its own tool. Grows with the catalog. |

## Development

```bash
make build     # → dist/portainer-mcp
make test      # unit tests
make check     # format, lint, vulncheck, test — what CI runs
```

Everything written to stdout is the MCP transport itself, so a stray `fmt.Println` corrupts the protocol; CI enforces this and logging goes to stderr through `internal/logging`.

The catalog is generated rather than hand-maintained, and a set of audits keeps it honest:

```bash
make audit-1to1           # which API operations the catalog covers
make audit-spec-drift     # has any action drifted from the vendored specification?
make audit-e2e-gaps       # which actions no e2e test touches
make audit-spec-reality   # does the vendored specification match a live server?
```

Divergences between Portainer's documents and its actual behaviour are recorded, with the measurement that established each one, in [`docs/api-divergences.md`](docs/api-divergences.md).

### End-to-end (e2e) testing

`test/e2e` provisions a real, disposable Portainer estate — two servers (Community and, when a
licence is available, Business Edition), a Portainer agent, an edge agent, and a Kubernetes leg in
k3d — and drives every action under test against it over the actual MCP transport, not mocks.
Everything runs on its own Docker-in-Docker daemon, so it never touches the host's containers or
its Docker socket.

```bash
make e2e-up        # bring the compose estate up from empty (~25s)
make e2e-k8s-up     # additionally bring up the k3d/Kubernetes leg (~2 min); needs k3d, kubectl, helm
make test-e2e       # run the e2e suite against the live estate (go test -tags e2e)
make e2e-k8s-down    # tear the Kubernetes leg down
make e2e-down        # tear the compose estate down
```

`make e2e-up` and `make e2e-down` need only Docker and Docker Compose. `make e2e-k8s-up` /
`make e2e-k8s-down` additionally need `k3d`, `kubectl` and `helm` on `PATH` — the scripts fail with
a named message if any is missing rather than doing something partial.

### Running the estate on another machine

The estate normally runs on the Docker daemon of the machine you are on. Set one key in the
gitignored `.env` at the repository root to run it somewhere else instead:

```dotenv
PORTAINER_E2E_DOCKER_SSH=truenas
```

The value is an SSH destination — a `Host` from your `~/.ssh/config`, or `user@host`. Key-based,
passphrase-free authentication is required: the scripts pass `BatchMode=yes` and will fail rather
than prompt.

**Setting the key changes nothing on its own.** `make e2e-up` and `make e2e-k8s-up` always use the
local Docker daemon, whatever `.env` says — this is a direct requirement from the repository
owner: a distracted `make e2e-up` must never reach their production NAS. Remote runs need their
own targets, and each fails loudly and creates nothing if `PORTAINER_E2E_DOCKER_SSH` is unset,
rather than silently falling back to local:

```bash
make e2e-up-remote        # the compose legs, on the remote host
make e2e-k8s-up-remote    # the Kubernetes leg — can legitimately be a different host
```

Teardown takes no flag. `up` records where it went — `test/e2e/.docker-host` for the compose legs,
`test/e2e/.docker-host-kubernetes` for the Kubernetes leg, kept separate because the two legs can
end up on different machines — and `make e2e-down` / `make e2e-k8s-down` read that marker back, so
the ordinary sequence works unchanged regardless of where anything actually ran:

```bash
make e2e-up-remote && make e2e-k8s-up-remote
go test -tags e2e -timeout 15m -count=1 ./test/e2e/suite/...
make e2e-k8s-down && make e2e-down
```

Ports are forwarded back over SSH, so `http://localhost:19000`, `http://localhost:19001` and the
`k3d-portainer-mcp-e2e` kube context mean the same thing they do locally. The Kubernetes leg needs
*two* forwards, not one. The k3s API port is pinned with `--api-port` (rather than left for k3d to
choose at random) because the tunnel has to be told which port to forward before the cluster even
exists — and once it is forwarded, `kubectl` is pointed at `https://127.0.0.1:<api_port>` rather than
whatever host k3d would otherwise write into the kubeconfig, because the k3s serving certificate
covers `127.0.0.1` and not the SSH alias or the host's LAN address. Portainer's own NodePort is a
second, separate forward, added once Helm creates the Service: the in-cluster server registers itself
at `server_ip:nodeport`, an address only the Docker host itself can reach, and the tunnel is the only
route this process — which may not be the Docker host — has to it. That forward carries no
certificate concern of its own; the in-cluster Portainer's self-signed certificate is read out of the
running pod and pinned separately (see `fetch_k8s_ca` in `test/e2e/scripts/lib.sh`).

**This needs TCP forwarding enabled on the remote sshd** — check with
`ssh <dest> 'sshd -T | grep -i allowtcpforwarding'`. It is off by default on some appliance
operating systems, TrueNAS among them, where every tunnel died with `connection reset by peer`
while the same port answered fine from the remote's own loopback. The persistent fix there is
`midclt call ssh.update '{"tcpfwd": true}'` — editing `/etc/ssh/sshd_config` directly does not
survive, because the middleware regenerates that file on its own.

Nothing on the remote host outside the `portainer-mcp-e2e` compose project and the
`portainer-mcp-e2e` k3d cluster is touched, and the one file written outside them — the CDI
specification under `/tmp` — is removed by teardown.

**GPUs.** If the remote host has an NVIDIA card, the estate finds it and offers it to both legs, with
different levels of confidence. The Docker leg is confirmed end to end: the containers Portainer
manages get the card through a hookless CDI specification bind-mounted into the estate's own dind
(nested `--gpus` cannot work there — see `docs/api-divergences.md`), and a container that requests it
reports the real device. That leg's override (`test/e2e/docker-compose.gpu.yml`, applying the `gpus:`
key) needs Docker Compose v2.30 or newer on the Docker host — an older Compose does not understand
the key. The Kubernetes leg is confirmed only as far as **node capacity** — the k3s
node advertises `nvidia.com/gpu`, which is the fact `GetKubernetesGPUInfo` reads — not as far as
running a workload through it: a pod that *claims* the GPU still fails at container creation with
`unresolvable CDI devices …`, an open item recorded in `docs/api-divergences.md` §10.3. No extra key
is needed for either leg — pointing the estate at a machine with a card is the whole opt-in. `--gpus
all` is only ever passed to `k3d cluster create` when a card AND a working NVIDIA Container Toolkit
were both actually detected: on a host with the driver but no toolkit at all, passing it unconditionally
fails the whole cluster with `failed to discover GPU vendor from CDI: no known GPU vendor found`.
Suites that need a GPU skip with a named reason everywhere else, so running locally stays exactly as
fast as it was.

**Docker Swarm.** `make e2e-up` also puts the estate's own dind into Swarm mode and creates one
long-lived fixture service (`portainer-mcp-e2e-swarm-probe`, `busybox sleep`), so Swarm-dependent
catalog actions — `docker.service_image_status` today — have something real to exercise instead of
the plain-engine 500 they get without it. The Swarm init and fixture-service steps are themselves
idempotent: running either again against a daemon that already has them reuses what exists rather than
failing on Docker's own "already part of a swarm"/"name conflicts with an existing object". That does
not extend to `make e2e-up` as a whole — the provisioner it runs afterwards unconditionally calls
`POST /users/admin/init`, which an already-initialized Portainer refuses, so a second `make e2e-up`
with no intervening `make e2e-down` still fails at that step regardless of the Swarm leg's own
idempotency. Like the GPU leg, Swarm needs no extra key — it is attempted unconditionally — and
degrades the same way: a host where `docker swarm init` is refused for any reason gets a warning, not
an aborted `make e2e-up`, and Swarm-dependent suites skip via `harness.Estate.HasSwarm()` instead of
failing. `make e2e-down` destroys the dind container wholesale and passes compose's own `-v` when it
does (`test/e2e/scripts/down.sh`), taking the swarm and the fixture service with it: `docker:28-dind`
declares `/var/lib/docker` as a volume, so without `-v` that state would survive in an anonymous volume
across the container's removal — `-v` is what actually makes there nothing extra to release or clean
up at teardown, not the container's writable layer alone.

**Business Edition licence.** Business Edition and the edge-only domains need a licence key in a
gitignored `.env` at the repository root (see `.env.example`):

```dotenv
PORTAINER_LICENSE=your-business-edition-key
```

Without it, `make e2e-up` still provisions the Community Edition leg; suites that need Business
Edition skip with a named reason instead of failing. The licence is released back
(`POST /licenses/remove`) from every server that attached it — compose EE and the Kubernetes leg
alike — before that server's container is destroyed, on both the success and the failure path, so
no run keeps a licence attached past its own teardown. `make e2e-licence-release` recovers a licence
left stranded by a run that crashed before it could release: it attaches the licence to a throwaway
server and releases it immediately, and is safe to run even when nothing is actually stranded.

**In CI** (`.github/workflows/e2e.yml`), the same is true: a pull request from a fork has no access
to the `PORTAINER_LICENSE` repository secret, so the workflow logs that Community Edition legs only
will run and creates no `.env` at all — the Business Edition legs then skip with that same named
reason, and the build does not go red for a secret a contributor cannot supply. When the secret is
available, the key is written to `.env` from the environment, never as a
command-line argument or an echoed value, and the file is removed on every exit path, including a
failed run.

### Security

To report a vulnerability, see [SECURITY.md](SECURITY.md). Please use **private disclosure** — do not open public issues for security bugs.

## License

MIT — see [LICENSE](LICENSE). Copyright (c) 2026 José M. Requena Plens.

This repository began as a fork of the [official Portainer MCP server](https://github.com/portainer/portainer-mcp)
but is no longer one: the server was rewritten from scratch and none of that project's
code remains. It is relicensed accordingly.

### Third-party material

`api/specs/*.json` are vendored copies of Portainer's published OpenAPI documents,
fetched from [api-docs.portainer.io](https://api-docs.portainer.io/). The Community
Edition document carries its own licence declaration (zlib) inline, and the generated
client under `internal/portainer/gen/` is produced from those documents — its type and
field comments are copied from their descriptions. That material remains © Portainer.io
under its own terms; the MIT licence above covers this project's own code.
