# Portainer API divergences

This is the committed record of every measured way in which the Portainer
server disagrees with the documents that describe it — the vendored OpenAPI
specifications under `api/specs/`, and this project's own design
specification. It exists because that disagreement is not hypothetical: this
file catalogues six broad categories of it (§1-§6 below — route existence,
behavioural divergence, understated requirements, secret-leaking responses,
edition asymmetry, and defects in the document itself). That is a wider
scope than `docs/domain-wave-checklist.md`'s "four times": that count is
specifically the spec-vs-server mismatches found by accident, one at a time,
before `cmd/audit_spec_reality` existed to look systematically (see that
command's own package doc for the four). Every one of those four is folded
into this file's broader six-category record; the two counts are not in
tension, they answer different questions. Before this tool existed, every
discovery was by accident, usually by someone who had already spent hours
suspecting their own handler.

Read this before implementing a new domain, not after. If your domain
appears below, the finding tells you what to expect from the live server
before you write a line against the documented shape.

## Scope, and the difference from `plan/carry-forward.md`

| File | Status | Holds |
|---|---|---|
| `docs/api-divergences.md` (this file) | Committed, permanent | Settled facts about how Portainer diverges from its own documentation, distilled and organised for lookup |
| `plan/carry-forward.md` | Gitignored scratch pad | In-progress reasoning, transcripts, deferred decisions, anything not yet settled enough to distil |

Anything in the working scratch pad that a future contributor would need and
could not reconstruct belongs here instead. The scratch pad is one fresh
clone away from not existing.


## The `jwt`-only security declaration is a documentation defect

**Measured 2026-08-04 against an ephemeral Portainer 2.44.0 Community Edition, with an API key.**

Sixteen operations in the vendored EE specification declare `security: [{jwt: []}]` with no
`ApiKeyAuth` alternative, against 402 that declare both and 24 that inherit. By tag: `docker` 8,
`endpoints` 4, `registries` 2, `stacks` 1, `users` 1.

This server authenticates with `X-API-Key` and nothing else (`internal/portainer/client.go`). Taken
at face value, the declaration would make all eight `docker`-tag operations uncallable by this
binary — the whole domain.

It is not true. Probed with an API key against a live server:

```text
GET /api/docker/{env}/dashboard   -> 200
GET /api/docker/{env}/images      -> 200
GET /api/docker/{env}/snapshot            -> 404   (no snapshot yet; 404 means auth passed)
GET /api/docker/{env}/snapshot/containers -> 404   (same)
GET /api/stacks/{id}/images_status        -> 404   (no stack yet; same)
```

A rejected key answers 401. Every probe got past authentication. The `jwt`-only declaration
describes nothing the server enforces.

**Why this was nearly missed, and why it is recorded here rather than left to be rediscovered.**
Neither existing audit can see it. `audit_spec_reality` classifies a route by whether it answers
Go's plain-text `404 page not found` or a JSON body, so a 401 from an API-key-rejecting route would
read as "route exists". And the only two `jwt`-only operations already in the catalog —
`registries.ecr_delete_repository` and `registries.ecr_delete_tags` — are exercised by a test that
asserts the call *fails*, so a 401 is indistinguishable from the expected "no real ECR backend"
error. The existing green test is not evidence either way.

**Consequence for planning:** treat the `security` field as unreliable. Do not gate work on it, and
do not add an allow-list entry for it — nothing in the toolchain reads it today, and nothing should
start.


## How a claim here is traced

Every entry carries an evidence label. There are three, and nothing is
recorded without one:

- **Probed live** — measured against a running Portainer 2.44.0, with the
  request and its response recorded at the time.
- **Vendored spec** — read directly from `api/specs/ce-2.44.0.json` or
  `api/specs/ee-2.44.0.json`, both committed and reproducible.
- **Diagnosed** — inferred from a server's or agent's own log output or
  error text, with the reasoning stated. Weaker than the other two; treated
  as a strong hypothesis, not a certainty, unless the entry says otherwise.

All live measurements below were taken against Portainer 2.44.0, Community
and Business Edition, on the Docker Compose estate that `make e2e-up`
builds. A finding is a fact about that server version; a version bump
invalidates nothing automatically but re-opens every one of these questions.

---

## 1. Routes the specification documents that the server does not serve

**Evidence: probed live**, by `cmd/audit_spec_reality` (P3.1 Task 5,
2026-08-03), the first systematic run of its kind. The mechanism: Portainer
answers an absent *route* with Go's own default-mux fallback, the plain-text
`404 page not found`, regardless of credentials; it answers everything else
— an absent resource, an auth failure, a downstream error — with its own
JSON `{"message","details"}` body. Those two are always distinguishable.
Every probe carries a credential that is not and never will be valid, so no
probe can act on the server; `auditLeg` additionally probes a manufactured
canary path on every run and refuses to report anything unless that path
classifies as absent.

### 1.1 The arithmetic, stated precisely

The headline number is **21 of 692**. It counts edition legs, not distinct
operations, and the two readings must not be confused:

| Reading | Count |
|---|---|
| Community Edition operations probed | 251 |
| Business Edition operations probed | 441 |
| Combined probes (251 + 441) | 692 |
| Community Edition operations not served | 9 |
| Business Edition operations not served | 12 |
| **Combined not served (9 + 12)** | **21** |
| **Distinct operations behind those 21** | **12** |

Nine of the twelve distinct operations are documented in both editions and
therefore counted twice; three (`addons`) exist only in the Business Edition
document. The 251/441 figures are the operations that carry an `operationId`
— the Community document has 265 path-item operations of which 14 are
unnamed, the Business document 442 of which 1 is unnamed; unnamed operations
are skipped because there is nothing to look up for them. Re-verified
against the committed specs on 2026-08-03; the counts reconcile exactly.

### 1.2 The full list, by domain

**`kubernetes` tag — 7 operations, both editions**

| Operation ID | Documented route | Verdict |
|---|---|---|
| `GetAllKubernetesClusterRoleBindings` | `GET /kubernetes/{id}/clusterrolebindings` | Naming mismatch (§1.3) |
| `GetAllKubernetesClusterRoles` | `GET /kubernetes/{id}/clusterroles` | Naming mismatch (§1.3) |
| `GetKubernetesRoleBindings` | `GET /kubernetes/{id}/rolebindings` | Naming mismatch (§1.3) |
| `GetKubernetesServiceAccounts` | `GET /kubernetes/{id}/serviceaccounts` | Naming mismatch (§1.3) |
| `GetKubernetesVolume` | `GET /kubernetes/{id}/volumes/{namespace}/{volume}` | Unknown (§1.5) |
| `GetKubernetesMetricsForAllPods` | `GET /kubernetes/{id}/metrics/pods/{namespace}` | Unknown (§1.5) |
| `GetKubernetesMetricsForPod` | `GET /kubernetes/{id}/metrics/pods/{namespace}/{name}` | Unknown (§1.5) |

**`helm` tag — 1 operation, both editions**

| Operation ID | Documented route | Verdict |
|---|---|---|
| `HelmShow` | `GET /templates/helm/{command}` | Wrong route shape (§1.4) |

**`upload` tag — 1 operation, both editions**

| Operation ID | Documented route | Verdict |
|---|---|---|
| `UploadTLS` | `POST /upload/tls/{certificate}` | Unknown (§1.5) |

**`addons` tag — 3 operations, Business Edition only** (the Community
document does not describe `addons` at all — confirmed against the vendored
spec)

| Operation ID | Documented route | Verdict |
|---|---|---|
| `AddonInstall` | `POST /addons/{id}` | Unknown (§1.6) |
| `AddonUninstall` | `DELETE /addons/{id}` | Unknown (§1.6) |
| `AddonAccessUpdate` | `PUT /addons/{id}/access` | Unknown (§1.6) |

### 1.3 Confirmed: four naming-convention mismatches, not missing features

Four of the seven `kubernetes` divergences are the same defect: the server
routes a **snake_case** path where the specification documents a
**concatenated-word** one. These are not absent features. Probed by hand
with the estate's real API key, so credential choice is not a confound:

```text
GET /kubernetes/1/clusterrolebindings   -> 404 page not found          (documented)
GET /kubernetes/1/cluster_role_bindings -> 500 {"message":"Unable to fetch
                                            cluster role bindings.",...}  (real)
GET /kubernetes/1/serviceaccounts       -> 404 page not found          (documented)
GET /kubernetes/1/service_accounts      -> 500 {"message":"unable to prepare
                                            kube client...",...}          (real)
```

The 500 is Portainer's own downstream error for endpoint id 1 not being a
Kubernetes-type environment on the Compose estate — and is itself the proof
the route exists, because an absent route never reaches a handler at all, it
returns the literal Go text measured one line above.

| Operation ID | Documented segment | Real segment |
|---|---|---|
| `GetAllKubernetesClusterRoleBindings` | `clusterrolebindings` | `cluster_role_bindings` |
| `GetAllKubernetesClusterRoles` | `clusterroles` | `cluster_roles` |
| `GetKubernetesRoleBindings` | `rolebindings` | `role_bindings` |
| `GetKubernetesServiceAccounts` | `serviceaccounts` | `service_accounts` |

Evidence strength differs within this table and the difference is worth
keeping: `cluster_role_bindings` and `service_accounts` have the literal
transcript above. `cluster_roles` and `role_bindings` are recorded as
reproducing the same pattern, without a transcript preserved. Whoever
implements `kubernetes` should re-probe those two rather than trust them.

### 1.4 Confirmed: `HelmShow`'s route takes a query parameter, not a path segment

The specification documents `GET /templates/helm/{command}`. The real route
is `GET /templates/helm` with **no `{command}` path segment**; it takes
`repo` or `registryId` as a **query parameter** instead. Probed live: the
real route answers `400 "Either repo or registryId query parameter is
required"`, not a 404, which both locates the route and names its
parameters.

An input struct generated from the documented shape will therefore carry a
required path parameter that does not exist, and omit the query parameter
that does. This is the one divergence in this section that a generator
cannot be left to resolve on its own.

### 1.5 Unresolved: four genuine unknowns

`GetKubernetesVolume`, `GetKubernetesMetricsForAllPods`,
`GetKubernetesMetricsForPod` and `UploadTLS` return the literal 404 on their
documented paths. A few plausible alternate paths were tried and none
resolved. These are **not** confirmed naming defects — they are unknowns,
and the possibilities (a snake_case variant not yet guessed, a route moved
elsewhere, a feature genuinely removed in 2.44.0) have not been
distinguished.

This is the first thing to settle when the `kubernetes` and `upload` domains
are implemented — before writing a handler against the documented path, not
after a user reports the tool call is broken.

### 1.6 Unresolved: three `addons` operations, Business Edition only

`AddonInstall`, `AddonUninstall` and `AddonAccessUpdate` are not mounted.
The same tag's other two operations, `AddonList` (`GET /addons`) and
`AddonDetail` (`GET /addons/{id}`), **are** served — they answer 401 against
the probe credential, exactly like every other real route. So this is not
"the addons feature is absent"; specifically the three mutating operations
are not routed. No root cause has been investigated. A licence-gated or
build-gated feature is one plausible explanation and has not been tested.

### 1.7 What a route-existence audit cannot see, and did not probe

- **Behavioural divergences are invisible to it.** A route that exists,
  answers success, and does not do what it claims (see §2) passes this
  audit. `registries.configure` and the Kubernetes auto-registration gap
  were both already known and neither was, or could have been, found here.
- **The Kubernetes leg was deliberately not probed.** Only the Community and
  Business Edition Compose legs were. The stated reasoning: route existence
  is a property of the server binary, the Helm-deployed leg runs the same
  pinned 2.44.0 image, and how a binary is deployed does not change which
  routes its own router mounts. That reasoning is specific to route
  existence and does not extend to behaviour — §2.2 is the counter-example
  that proves the distinction matters.

---

## 2. Behavioural divergences: the route exists and answers, but lies

These are the class that no 404-versus-404 check can ever find, and the
class that costs the most debugging time, because the failure looks like
your own bug.

### 2.1 `registries.configure` returns success and does not persist

**Evidence: probed live**, with `curl` directly against the API, entirely
outside this project's client and handler.

`POST /registries/{id}/configure` returns 204/success, and a subsequent
`registries.inspect` always shows `ManagementConfiguration.Authentication =
false` and `.Username = ""`, whatever was sent. Reproduced against registry
types `custom` (3), `Docker Hub` (6) and `ECR` (7), with and without the
`endpointId` query parameter on the inspect. The same non-effect reproduces
with this project's code completely out of the path, so it is Portainer
2.44.0's own behaviour, not a defect in `registryConfigure`.

Consequences for the wave that implements `registries` in full:

- Do not write a read-back assertion for `configure`. It will always fail.
- Do not spend time suspecting the handler. That has been done.
- See `test/e2e/suite/registries_test.go` (`TestRegistries_Configure`) for
  the full investigation trail.

### 2.2 A Kubernetes-deployed server does not acquire its own cluster

**Evidence: probed live**, against a Helm-deployed Portainer 2.44.0 in k3d.

The design specification states that a Portainer deployed inside a cluster
"acquires its local Kubernetes environment automatically". It does not.
After provisioning, `GET /endpoints` returns an empty list. The environment
must be created explicitly:

```text
POST /endpoints -F Name=k3d -F EndpointCreationType=5     (Local Kubernetes)
```

With that, `Status=1` and Portainer then does autodetect storage classes
(`local-path`) and ingress classes (`traefik`). The proxy is real and was
verified end to end: `GET /endpoints/1/kubernetes/api/v1/namespaces` returns
`default, kube-node-lease, kube-public, kube-system, portainer`.

### 2.3 `system.info` carries no `ServerVersion`

**Evidence: probed live, and confirmed against the generated type.**

Only `system.version` and `system.status` carry `ServerVersion`.
`GithubComPortainerPortainerEeApiHttpHandlerSystemSystemInfoResponse` in
`internal/portainer/gen/types.gen.go` exposes only `addonsAvailable`,
`agents`, `edgeAgents`, `edgeDevices` and `platform`. An assertion on
`ServerVersion` over a `system.info` result compiles and fails forever with
`<nil>`. See `test/e2e/suite/system_test.go`.

### 2.4 `ServiceImageStatus` answers from a stale cache that outlives the service, and outlives Swarm itself

**Evidence: probed live**, 2026-08-08, against the `wave1-stage-a-docker` e2e
estate's Business Edition Compose leg, using the fixture Swarm service
(`portainer-mcp-e2e-swarm-probe`, service id
`szhjqsafzx4ksaihnrx9ul9g3` in the run quoted below) that `make e2e-up`
creates for this domain's coverage.

`GET /docker/{environmentId}/services/{serviceId}/image_status`
(`docker.service_image_status`) without the optional `refresh` query
parameter answers a cached `200 {"Status":"updated","Message":""}` for a
service id it once resolved successfully — and keeps answering it after the
service is gone. The first version of this domain's e2e test asserted only
"no error, `Status` non-empty, not `\"error\"`"; after deleting the fixture
service (`docker service rm portainer-mcp-e2e-swarm-probe`) and re-running
it, **the test still passed**, against a service that no longer existed.
Pushing further — leaving Swarm on that node entirely
(`docker swarm leave --force`), with the service already deleted — and
querying the same endpoint directly with `curl` got the identical stale
answer, while adding `refresh=true` forced a live `docker service inspect`
and got the honest failure:

```text
$ docker exec portainer-mcp-e2e-docker-1 docker swarm leave --force
$ curl .../docker/1/services/szhjqsafzx4ksaihnrx9ul9g3/image_status
{"Status":"updated","Message":""}     # 200, swarm INACTIVE, service already deleted, cached answer
$ curl .../docker/1/services/szhjqsafzx4ksaihnrx9ul9g3/image_status?refresh=true
{"message":"Unable to get the status of this image","details":"...This node is not a swarm manager..."}   # 500, refresh forces a live check and it fails honestly
```

`refresh: true` is the remedy, confirmed by two further breaks made
*after* adding it to the test: with the fixture service torn down but Swarm
still active, the call now fails with the real
`Error response from daemon: service ... not found`; with Swarm left
entirely (service recreated first), the precondition call
(`docker.service_image_status_clear`) now fails with the real
`This node is not a swarm manager` error. All three states — healthy
(pass), broken (fail, with the live server's own error text), stale-without-
refresh (silent pass on dead data) — were observed directly. See
`test/e2e/suite/docker_test.go`
(`TestDocker_ServiceImageStatus_AgainstARealSwarmService`) for the test that
now passes `refresh: true` for exactly this reason, and
`internal/tools/docker/docker.go`'s `narrative` function for the
model-facing description of it.

**What this does not establish.** Only `ServiceImageStatus` was probed. How
long the cache lives, whether it is keyed per-service or is a single global
cache, and whether any other `docker`-tagged endpoint shares it, were not
measured — treat all three as unknown, not as "probably the same".
`ContainerImageStatus` (`GET
/docker/{environmentId}/containers/{containerId}/image_status`) was **not**
probed for this behaviour. It does declare the identical optional `refresh`
boolean query parameter in the vendored specification, with the identical
parameter description ("Refresh will force a refresh of the image status
cache" — checked against `api/specs/ee-2.44.0.json`, both operations,
2026-08-08), which is why its own narrative flags the same risk — but
flags it as unverified, not measured. Whoever next has a live container to
delete out from under a running Portainer should confirm or refute it
directly rather than assume this section already covers it.

**A related but distinct finding, measured while building this domain's e2e
coverage of `ContainerImageStatus` itself (2026-08-08).** Unlike
`DockerContainerGpusInspect` — which answers 404
(`{"message":"Unable to find the container","details":"...No such
container: <id>"}`) for a container id Docker never assigned —
`ContainerImageStatus` does not fail for one at all, even with
`refresh=true`:

```text
$ curl .../docker/1/containers/<fabricated-64-hex>/image_status?refresh=true
{"Status":"skipped","Message":""}     # 200, container never existed
```

So `Status: "skipped"` is not proof a container's image check was
genuinely skipped for some policy reason; it is also indistinguishable from
"this container id does not exist". A caller that wants to tell the two
apart needs to confirm the container exists separately (for instance via
`DockerContainerGpusInspect` or the Docker proxy's own inspect route, both
of which do 404 on an unknown id) rather than trust `ContainerImageStatus`
alone. See `test/e2e/suite/docker_test.go`
(`TestDocker_ContainerImageStatus_AgainstARealContainer`) for the test that
asserts this directly, and `internal/tools/docker/docker.go`'s `narrative`
function for the model-facing description of it.

### 2.5 `POST /custom_templates/create/file` ignores the multipart filename, and `create/repository` returns no `EntryPoint` at all

**Evidence: measured** against a live Portainer 2.44.0. The two-filenames
comparison was run on Community Edition; the resulting `EntryPoint` value was
separately confirmed on Business Edition, and is asserted on both legs by
`TestCustomTemplates_CreateFile_StoresTheUploadedStackFile`. Recorded
2026-08-18 (wave 1 stage B, task 7).

`internal/tools/custom_templates/handlers.go` writes a constant filename,
`template.yml`, into the multipart `File` part's `Content-Disposition`,
with a comment stating that what Portainer does with the name was never
measured. It does nothing with it. Two uploads of identical content
differing only in filename produce identical templates:

```text
POST /custom_templates/create/file
  -F Title=... -F Description=... -F Note=n -F Platform=1 -F Type=2
  -F "File=@stack.yml;filename=template.yml"
→ 200 {"Id":11, ..., "EntryPoint":"docker-compose.yml", ...}

POST /custom_templates/create/file            (same body, one part renamed)
  -F "File=@stack.yml;filename=my-custom-stack.yaml"
→ 200 {"Id":12, ..., "EntryPoint":"docker-compose.yml", ...}

GET /custom_templates/11/file → {"FileContent":"services:\n  hello:\n ..."}
GET /custom_templates/12/file → {"FileContent":"services:\n  hello:\n ..."}   (identical)
```

`EntryPoint` is the literal `docker-compose.yml` in both, and the stored
file is byte-identical in both. So the constant is mechanically necessary —
Go's multipart reader files a part under `Form.File` only when its
`Content-Disposition` carries a filename, and Portainer parses the body with
exactly that reader — but its **value** is unobservable, and there is no
case for deriving it from the caller or adding an input field for it. The
measured value is pinned by
`TestCustomTemplates_CreateFile_StoresTheUploadedStackFile`, so a Portainer
that starts honouring the filename shows up as a failing assertion rather
than as a silently wrong `EntryPoint`.

The same field behaves differently on the git route:
`POST /custom_templates/create/repository` returns `EntryPoint: ""` and puts
the path in `GitConfig.ConfigFilePath` instead. Nothing in the vendored
document says either.

### 2.6 `PUT /custom_templates/{id}` answers 200 and does not store `FileContent` for a git-backed template

**Evidence: measured** against a live Portainer 2.44.0, Community and
Business Edition alike; recorded 2026-08-18 (wave 1 stage B, task 7). The
Business Edition leg was re-run deliberately rather than assumed, after §3.8
turned out to be an edition asymmetry: the same sequence there answers 200
and leaves the file at the git content too.

`FileContent` is required on the update route, and for a template created
from an inline string it is stored, as expected:

```text
PUT /custom_templates/13   {"Title":"...","Description":"...","FileContent":"...edited by update...","Platform":1,"Type":2}
→ 200
GET /custom_templates/13/file → {"FileContent":"...edited by update..."}   (stored)
```

For a template created from a git repository, the same call answers 200 and
the stored file does not change:

```text
PUT /custom_templates/6    {"Title":"...","Description":"...","FileContent":"...edited by update...","Platform":1,"Type":2,
                            "RepositoryURL":"http://git:8080/cgi-bin/git/repo.git","ComposeFilePathInRepository":"docker-compose.yml"}
→ 200  (Title and Description are updated)
GET /custom_templates/6/file → {"FileContent":"...the git content..."}     (unchanged)
```

This is the same family as §2.1: a route that reports success for a change
it did not make. It is narrower — the rest of the payload IS stored, so the
call is not a whole no-op — and it is arguably reasonable (the file belongs
to the repository, and `custom_templates.git_fetch` is what refreshes it),
but nothing in the document hints at it, and `custom_templates.update`'s own
narrative currently tells a caller that "every field sent is stored".

Consequence for a caller: to change a git-backed template's stack file, push
to the repository and call `custom_templates.git_fetch`. Sending different
content through `update` is accepted and lost.

---

## 3. Requirements the documents understate or omit

### 3.1 `X-Setup-Token` on `POST /users/admin/init`

**Evidence: probed live** for the behaviour; **vendored spec** for what is
documented, re-verified 2026-08-03.

Against an uninitialized Portainer 2.44.0, `POST /users/admin/init` without
the header returns:

```text
403 {"message":"Invalid or missing setup token. Provide the X-Setup-Token
     header with the token printed in the server logs at startup."}
```

The header is **mandatory** on an uninitialized instance unless the server
was started with `--no-setup-token`. What the documents say:

| Document | What it says |
|---|---|
| `api/specs/{ce,ee}-2.44.0.json` | Documents `X-Setup-Token` as a header parameter on `UserAdminInit`, but **does not mark it `required`** |
| This project's design specification, §6.2 | Describes cold start as "fully scriptable via the API" and omits the step entirely |

So the divergence is a spec that understates a mandatory header as optional,
plus a design document that omits it. Note that `cmd/audit_spec_reality`'s
own package doc describes this as "a required header the spec never
mentions" — that phrasing does not hold against the committed OpenAPI as of
2.44.0, and is corrected here.

Two ways to satisfy it, both verified live:

- `docker run ... --no-setup-token` removes the step entirely and is
  idempotent. Preferred for an ephemeral bench.
- Inside Kubernetes, where the flag is awkward to pass, read the token from
  the logs: `kubectl -n portainer logs deploy/portainer | grep -oE
  'setup_token=[0-9a-f]{64}'`, then send it as the header. Verified: init
  returns 200.

### 3.2 `POST /endpoints` is `multipart/form-data`, not JSON

**Evidence: probed live.** Only `Name` and `EndpointCreationType` are
mandatory. The form additionally accepts `URL`, `TLS`, `TLSCACertFile`,
`TLSCertFile`, `TLSKeyFile`, `TLSSkipClientVerify` and `TLSSkipVerify`.
Creation types measured: `1` local Docker, `2` agent, `4` edge agent, `5`
local Kubernetes.

### 3.3 Registering a Portainer agent needs `TLSSkipClientVerify`

**Evidence: probed live.** The agent always serves over TLS with a
self-signed certificate valid only for `localhost`, so it cannot be
registered by hostname without skipping verification. The two obvious
attempts both fail with a 400:

```text
-F TLS=true -F TLSSkipVerify=true
  -> 400 "Invalid certificate file. Ensure that the file is uploaded correctly"
no TLS / TLS=false
  -> 400 "tls: failed to verify certificate: x509: certificate is valid for
          localhost, not pmcp-agent"
```

What works is the third, non-obvious flag:

```text
POST /endpoints -F Name=agent -F EndpointCreationType=2 -F URL=tcp://<agent>:9001 \
                -F TLS=true -F TLSSkipVerify=true -F TLSSkipClientVerify=true
  -> 200, Status=1
```

### 3.4 Edge Compute must be enabled before an edge environment can be created

**Evidence: probed live.** `POST /endpoints -F EndpointCreationType=4`
against a freshly provisioned Business Edition returns
`{"message":"API server URL not set in Edge Compute settings"}` — no
`Status`, no registration attempt. `PUT /api/settings` must come first, with
`EnableEdgeComputeFeatures: true`, `EdgePortainerUrl` (the address the agent
will poll — the in-network address, never the port published to the host)
and `Edge.TunnelServerAddress`.

### 3.5 `EndpointID` is not `EdgeID`

**Evidence: probed live**, after a full diagnosis cycle spent on the
confusion.

Creating an edge environment returns two identifiers that look
interchangeable and are not: `Id`, the ordinary numeric Portainer endpoint
id, and `EdgeID`, a UUID minted for that environment's edge identity. The
agent's `EDGE_ID` variable wants the UUID. Passing the numeric one does not
fail silently, but the error names a numeric id and so points the wrong way:

```text
"Permission denied to access environment. The device has not been trusted yet:
 Unauthorized Edge endpoint operation: invalid Edge identifier. Environment ID: 2"
```

`harness.EdgeCredentials` now separates `EndpointID int` from `EdgeID
string` for exactly this reason.

### 3.6 The Portainer agent's Docker proxy ignores `DOCKER_HOST`

**Evidence: diagnosed** from the agent's own `LOG_LEVEL=DEBUG` output;
attributed to `http/proxy/local.go` in the agent binary.

The agent's own start-up (engine detection, self-IP discovery) respects
`DOCKER_HOST`. The handler that actually proxies a registered environment's
Docker calls does not — it hard-codes `unix:///var/run/docker.sock` with no
flag or environment variable to redirect it. Under a Docker-in-Docker
topology the symptom is an opaque 500 on registration
(`"...check if the server supports the requested API version"`) that no
combination of TLS flags fixes, and the real error only appears in the
agent's debug log:

```text
msg="Unable to proxy the request via the Docker socket"
error="dial unix /var/run/docker.sock: connect: no such file or directory"
```

The fix in this project's bench is a named volume mounted at `/var/run` in
both the dind daemon and the agent. Recorded here because the misleading
symptom, not the fix, is what costs the time.

### 3.7 The custom-template create routes' `required` arrays are wrong in both directions

**Evidence: measured** against a live Portainer 2.44.0, Community and
Business Edition alike; recorded 2026-08-14 (wave 1 stage B, task 4).

`POST /custom_templates/create/repository` and
`POST /custom_templates/create/string` each publish a `required` array that
contradicts what the server enforces, once in each direction:

| Field | Vendored `required` | Server | Measurement |
|---|---|---|---|
| `Platform` (both routes) | absent | **enforced** for Docker stacks | `Type: 2` with no `Platform` → `500 "Invalid custom template platform"` |
| `SourceID` (`create/repository`) | present | **not required** | `RepositoryURL` + `Platform: 1`, no `SourceID` → `200`, repository cloned. `SourceID: 0` sent explicitly → `200`, identical. `SourceID: 99999` → `500 "Source not found"` |
| `Note` (`create/file`) | present | **not required** | a multipart body with no `Note` part at all → `200`, and the template comes back with `"Note":""` — measured 2026-08-18 on Community **and** Business Edition |

`Platform`'s own field description already says "Required for Docker
stacks", so on that route the field prose is right and the `required` array
is wrong. `SourceID` is read when a real identifier is supplied and
validated against `/gitops/sources`, so it is optional rather than ignored;
zero is genuinely unset.

This matters more than a documentation defect normally would, because
`toolutil.ActionSpec.ValidateInput` enforces required-ness *locally*, before
the handler runs (`internal/tools/register.go`). Publishing the vendored
arrays verbatim would mean a model that fills exactly the required fields —
which is what a model does — omits `Platform` and takes the 500 every time,
while a caller cloning from the inline repository fields is refused outright
for lacking a `SourceID` the server never wanted. The catalog therefore
publishes `Platform` required and `SourceID` optional on those two routes,
each with a dated `api/spec-drift-allowlist.yaml` entry.

Deliberately **not** changed on `PUT /custom_templates/{id}`: `Platform`
carries the same "Required for Docker stacks" note there, but that route was
never probed, and inferring a requirement onto an unmeasured route is how a
schema starts lying in the other direction.

`Note` on `POST /custom_templates/create/file` was measured later than the
other two rows (2026-08-18, wave 1 stage B task 7), recorded uncorrected at
the time because correcting it changes the shipped surface, and **corrected
separately once that decision was taken**. `Note` is now `*string` with
`omitempty` in `customTemplateCreateFileInput`, the hand-written multipart
handler emits it through `OptionalField` beside its sibling optionals, and
the allow-list carries a dated entry for
`(CustomTemplateCreateFile, note)`.

It is worth recording how this one fails, because it is quieter than the
other two and that is what delayed it: an over-required field can never
produce a server error, only a local refusal from
`toolutil.ActionSpec.ValidateInput`. The cost was a caller with nothing to
note being made to invent one — irritating, and invisible in every log.

Related: the inline repository fields (`RepositoryURL`,
`RepositoryUsername`, `RepositoryPassword`, `RepositoryAuthentication`,
`RepositoryAuthorizationType`, `RepositoryProvider`) are all marked
*"Deprecated: use SourceID instead"* in the vendored document, yet that
deprecated path is the one measured working end to end — over smart HTTP on
both editions, and over `git://` on Business Edition only; see §3.8.

### 3.8 Git transport support differs by edition: Community Edition cannot clone `git://` at all, and neither edition can clone dumb HTTP

**Evidence: measured** against a live Portainer 2.44.0, Community and
Business Edition, 2026-08-18 (wave 1 stage B, task 7), re-measured the same
day after review: the first pass reported this as "smart HTTP only" for both
editions, which is wrong, and the worked example at the end of this section
is how a plausible-looking wrong conclusion was produced.

Nothing in the vendored document says what git transports
`POST /custom_templates/create/repository` can use; `RepositoryURL` is
documented as "URL of a Git repository hosting the Stack file" and nothing
more. What the two editions accept is not the same:

| Transport | real `git` client | Community Edition | Business Edition |
|---|---|---|---|
| dumb HTTP (static files + `git update-server-info`) | `git clone` succeeds | `500 … failed to clone git repository: unexpected EOF` | `500 … failed to list repository refs: unexpected EOF` |
| `git://`, daemon listening, no authentication asked for | `git ls-remote` succeeds | `500 … failed to clone git repository: invalid auth method` | **`200`, repository cloned** |
| `git://`, daemon listening, `RepositoryAuthentication: true` | n/a | `500 … invalid auth method` | `500 … failed to list repository refs: invalid auth method` |
| `git://`, nothing listening on 9418 | connection refused | `500 … invalid auth method` (identical) | `500 … dial tcp 172.20.0.2:9418: connect: connection refused` |
| `git://`, hostname that does not resolve | n/a | `500 … invalid auth method` (identical) | `500 … dial tcp: lookup no-such-host-anywhere on 127.0.0.11:53: no such host` |
| smart HTTP (`git-http-backend` behind a CGI server) | `git clone` succeeds | **`200`** | **`200`** |

Every "real `git` client" cell was run, not inferred: `git ls-remote` or
`git clone` from a throwaway container on the same network, before any
Portainer call.

Business Edition dials, and its errors say what happened at the wire:

```text
POST /api/custom_templates/create/repository            (EE)
{"Title":"e2e-e1","Description":"anonymous","RepositoryURL":"git://gitdaemon/repo.git",
 "ComposeFilePathInRepository":"docker-compose.yml","Platform":1,"Type":2}
200 {"Id":1,...,"GitConfig":{"URL":"git://gitdaemon/repo","ConfigFilePath":"docker-compose.yml",...}}

same body plus "RepositoryAuthentication":true,"RepositoryUsername":"u","RepositoryPassword":"p"
500 {"message":"Unable to create custom template",
     "details":"Unable to fetch git repository id: failed to list repository refs: invalid auth method"}
```

That second failure is correct behaviour, not a defect: the git protocol has
no authentication mechanism, so go-git refuses an `AuthMethod` on a `git://`
transport. Asking for authentication over `git://` is the caller's mistake.

Community Edition never gets that far. The identical anonymous body against
the same listening daemon:

```text
POST /api/custom_templates/create/repository            (CE)
{"Title":"e2e-d1","Description":"anonymous","RepositoryURL":"git://gitdaemon/repo.git",
 "ComposeFilePathInRepository":"docker-compose.yml","Platform":1,"Type":2}
500 {"message":"Unable to create custom template",
     "details":"Unable to clone git repository: failed to clone git repository: invalid auth method"}
```

and it answers that same string for a URL where nothing listens, and for a
hostname that does not resolve at all — so it is not reaching DNS, let alone
the network. `SourceID: 0`, `RepositoryAuthentication: false`, empty
`RepositoryUsername`/`RepositoryPassword` and an explicit
`RepositoryReferenceName` were each tried, alone and combined: every one
answers the same 500. On this edition the clone path attaches a credential
object unconditionally, and `git://` is unusable through this route no
matter what the caller sends.

The two editions' error prefixes are worth knowing on their own, because
they identify which code path answered: Community Edition says
`Unable to clone git repository: failed to clone git repository: …`,
Business Edition says
`Unable to fetch git repository id: failed to list repository refs: …`.
Telling those apart is what resolved a disagreement between two people who
had each measured one leg and written "Portainer".

Consequences:

- A caller on Business Edition can clone `git://` anonymously through
  `custom_templates.create_repository` today. The generated request body
  always carries `Description`, `SourceID`, `Title` and `Type` (non-pointer
  fields) and carries `RepositoryAuthentication` only when the caller sets
  it, so the catalog never triggers the authenticated-`git://` failure by
  itself.
- The same call on Community Edition always fails. Nothing in the catalog
  can fix that; it is the server's own path.
- Dumb HTTP is unusable on both editions, whatever the repository's own
  `git clone` does.

`test/e2e/docker-compose.yml`'s `git` service therefore serves **smart
HTTP**, and the reasons survive the correction above: it is the only one of
the three transports that works on **both** editions, it is the transport
real deployments use, and it is the only one that can also carry the
**authenticated** clones `stacks` and `edge_stacks` will need later in this
wave — `git://` cannot express a credential at all. It is not, as this
section first claimed, the only transport that works.

**How the wrong conclusion was produced**, recorded because the mechanism is
reusable: the first pass measured Community Edition only, and wrote
"Portainer". Worse, by the time the row was written the estate's `git`
service had been switched from `git daemon` to `httpd`, so nothing was
listening on 9418 any more — and because Community Edition answers
`invalid auth method` whether or not anything is listening, the response
alone cannot distinguish "the client refuses this transport" from "the port
is closed". A reviewer re-running the probe against the switched fixture saw
a dial failure on the other edition and concluded the original row was an
artefact; it was not, but it was also not what it claimed to be. One
`git ls-remote git://…` from a throwaway container — the same check the
fixture's own healthcheck performs — separates the two states in a second,
and both parties had skipped it. Separate "the server is not there" from
"the client refuses" before writing either down, and name the edition every
time the two legs have not both been run.

---

## 4. Responses that leak secrets

### 4.1 `GET /api/licenses` returns the full licence key and the holder's name

**Evidence: probed live** against a Business Edition server with a real
licence attached.

The response contains the **complete, unredacted licence key** and the real
name of its holder in the `company` field. Nothing in the generated client
or the tool layer knows this endpoint needs redaction. The only redaction
that exists today (`redactSecret` / `redactKeyShape` in
`test/e2e/harness/provision.go`) covers two harness functions that talk to
Portainer directly, and nothing else.

This must be designed **before** the `licenses` domain is written, not
discovered while writing it. All four licence operations are Business
Edition only and absent from the Community document:

| Operation ID | Route | Leaks |
|---|---|---|
| `licensesList` | `GET /licenses` | Full key, holder name |
| `licensesInfo` | `GET /licenses/info` | To be checked — same family, not separately measured |
| `licensesAttach` | `POST /licenses/add` | Key is sent, not returned; returns `conflictingKeys` |
| `licensesDelete` | `POST /licenses/remove` | Key is sent, not returned |

Any action exposing `licensesList` or `licensesInfo` without its own
redaction layer hands a full licence key straight into a model's context —
and into any e2e assertion failure message that happens to print the
response. The precedent for how to handle it already exists: the generator
refuses a bare handler for a credential-returning operation and requires a
declared `redact<OperationID>` wrapper.

---

## 5. Edition asymmetry: Business Edition is not a superset of Community

**Evidence: vendored spec**, measured across both committed documents during
the P2 pre-scan, re-verified 2026-08-03.

The client is generated from the Business Edition document alone. That is a
deliberate decision, but it is only sound because the gaps are known and
shimmed by hand.

| Measurement | Value |
|---|---|
| Operations that exist only in Community Edition | 2 |
| Shared schemas that differ between editions | 42 |
| Paths that differ between editions | 118 |
| Shared schemas that **lose** Community fields under the Business shape | 4 |

The two Community-only operations have **no generated method at all**:

| Operation ID | Route |
|---|---|
| `systemUpgrade` | `POST /system/upgrade` |
| `GetKubernetesConfig` | `GET /kubernetes/config` (kubeconfig download) |

The four lossy schemas, each of which needs a hand-written type in its
domain before that domain ships, or the route silently drops fields against
a Community server:

| Schema | Fields lost |
|---|---|
| `auth.authenticatePayload` | `Password`, `Username` |
| `endpoints.endpointSettingsUpdatePayload` | 10 fields |
| `gitops.repositoryFilePreviewPayload` | 2 fields |
| `settings.publicSettingsResponse` | `IsDockerDesktopExtension` |

The other 31 differing schemas only **add** fields in the Business document,
which is harmless when decoding.

---

## 6. Defects in the vendored document itself

**Evidence: vendored spec**, re-verified 2026-08-03.

### 6.1 A content type with a leading space

Community Edition's `GET /kubernetes/config` declares its 200 response
content type as `" application/yaml"` — with a leading space. Still present
in the committed spec; `cmd/fetch_spec/normalise.go` has rules for duplicate
enums and `*/*` wildcards but not for this. Add a content-type trimming
rule, with its test, when the `kubernetes` domain is implemented.

### 6.2 Operations with no `operationId`

The Community document has 14 path-item operations with no `operationId`,
the Business document 1. Every tool in `cmd/` that reads these documents
skips them: there is no name to derive a client method, a catalog entry or
an audit key from. They are therefore invisible to coverage figures as well
as to the reality audit, and are the reason the probed totals are 251 and
441 rather than 265 and 442.

### 6.3 Four identifiers declared `integer` that Portainer never treats as a number

**Evidence: vendored spec** for the declaration; **diagnosed** for what Portainer actually does with
each value, from the shape of the identifier itself and Docker's/Docker Swarm's own ID conventions;
recorded 2026-08-04 (P3.3 task 7).

Four path parameters across three `docker`-tagged operations and one endpoint-scoped one declare
`"type": "integer"` in the vendored Business Edition specification, yet the identifier each one
names is never actually a number:

| Operation ID | Route | Parameter | Real shape |
|---|---|---|---|
| `dockerContainerGpusInspect` | `GET /docker/{environmentId}/containers/{containerId}/gpus` | `containerId` | Docker's 64-character hex container ID |
| `containerImageStatus` | `GET /docker/{environmentId}/containers/{containerId}/image_status` | `containerId` | same |
| `snapshotContainerInspect` | `GET /docker/{environmentId}/snapshot/containers/{containerId}` | `containerId` | same |
| `ServiceImageStatus` | `GET /docker/{environmentId}/services/{serviceId}/image_status` | `serviceId` | Docker Swarm's own alphanumeric service ID (e.g. `9mnpnzenvg8p8tdbtq4wvbkcz`) |

Left as generated, all four actions were uncallable: `cmd/gen_action_inputs` rendered each field as
Go `int`, publishing JSON Schema `"type": "integer"`, and `toolutil.ActionSpec.ValidateInput` (the
same check every real tool call goes through, via `tools.Execute`) refused the only values that could
ever work — neither identifier round-trips through an integer at all, let alone the specific one
Docker or Swarm assigned. `cmd/gen_action_inputs/fields.go`'s `pathParamTypeOverrides` now renders
all four as `string` instead; see that table's own doc comment for the mechanism, and
`pathParamMinimumExceptions`'s doc comment for why the pre-existing `containerId` carve-out there
(which suppressed a numeric `"minimum"`, not the type itself) needed a type-level fix on top, and why
`serviceId` needed a fifth minimum-exception entry once its own type changed.

This does not, on its own, make the four operations generatable. Every generated client method
(`internal/portainer/gen`, built by `oapi-codegen` from the identical wrong declaration) *also* takes
`containerId`/`serviceId` as a Go `int` — the wrong type is baked into two independently generated
layers, not one. `cmd/gen_action_inputs/handler.go`'s own path-argument type check
(`goTypeMatchesReflectType`) refuses to bind a `string` Input field to an `int` client parameter, so
once a fixed `docker`/`endpoints` domain is scaffolded, all four of these operations will refuse
generation there too — correctly: no automatically generated handler can call any of them, whichever
type is published, because the generated client's own signature cannot carry the real identifier
either. Whoever scaffolds `docker`/`endpoints` must hand-write these four handlers, the same way the
four existing pilot actions (`EcrDeleteTags`, `RegistryConfigure`, `RepositoryTagsDelete`,
`SystemUpgrade`) already bypass generation for their own reasons — building the HTTP request directly
with the real string identifier rather than going through the generated client's typed wrapper.

**The cheat this is written down to forbid.** `docker.service_image_status`'s `serviceId` can be made
to look correct without actually being correct: label a probe container (or a Swarm service, in the
e2e estate) `com.docker.swarm.service.id=1` and pass the plain integer `1` as `serviceId`. That value
is a real, resolvable service ID on that specific probe — so a test that only checks "a value I chose
resolves successfully" passes — while the schema underneath is still wrong for every service Portainer
did not have a test author hand-label. A real deployment's Swarm service IDs are assigned by Swarm,
never `1`. `containerId` has no equivalent shortcut at all: Docker, not a test author, assigns the
64-hex container ID, so there is no small integer that could ever be a real one to fake acceptance
with. That asymmetry is exactly why the string fix above is not optional for either field, and why
any handler or test written against these four operations must validate with a realistic, non-trivial
identifier and must separately assert that a plain integer is refused — accepting a realistic string
alone proves nothing that an unfixed, still-`"integer"` schema could not also have passed by coincidence.

### 6.4 `ecrDeleteTags.RepositoryName` was typed wrong

Named in `cmd/audit_spec_reality`'s package doc as the fourth of the four
spec defects found by accident before that tool existed — "a field typed
wrong for what it plainly holds". The working scratch pad never recorded
what the wrong type was or how it was resolved, so this entry is a pointer,
not a description. See `internal/tools/registries/registries.go` and
`internal/tools/registries/inputs.go` for the shape it settled into (renamed
from `inputs.gen.go` when the pilot domains were converted to owned files —
see §9.1).

### 6.5 `CustomTemplateCreateRepository.Type` declares an enum narrower than the server accepts

**Evidence: vendored spec** for the declaration, verified 2026-08-14;
**measured** against a live Portainer 2.44.0, Community and Business Edition
alike, 2026-08-18 (wave 1 stage B, task 7).

`POST /custom_templates/create/repository` declares `Type` with
`enum: [1, 2]` while the same field's description reads:

```text
Type of created stack:
* 1 - swarm
* 2 - compose
* 3 - kubernetes
```

The two sibling routes disagree with it: `POST /custom_templates/create/string`
and `PUT /custom_templates/{id}` both declare `enum: [1, 2, 3]` with the
same prose.

This section previously recorded the enum as published-as-declared, with an
explicit instruction for settling it: *"create a git-backed custom template
with `Type: 3` against a live 2.44.0 (both editions). If it answers 200,
widen the enum to `[1, 2, 3]` with an allow-list entry citing that
measurement; if it answers 4xx/5xx, record the message here and leave the
enum alone."* That was done:

```text
POST /api/custom_templates/create/repository        (Community Edition)
{"Title":"e2e-probe-repo-type3","Description":"probe: type 3 on create/repository",
 "RepositoryURL":"http://httpgit:8080/cgi-bin/git/repo.git",
 "ComposeFilePathInRepository":"docker-compose.yml","Platform":1,"Type":3}

200 {"Id":8,...,"Type":3,...,"GitConfig":{"URL":"http://httpgit:8080/cgi-bin/git/repo",...}}
```

The identical body against the Business Edition server answers `200` with
`"Type":3` as well. The server accepts it, stores it as type 3, and clones
the repository exactly as it does for types 1 and 2.

So the enum is now published as `[1, 2, 3]`, carrying a dated
`api/spec-drift-allowlist.yaml` entry for
(`CustomTemplateCreateRepository`, `type`) that cites this measurement, and
`custom_templates.create_repository`'s narrative no longer sends a caller
wanting a Kubernetes template to `custom_templates.create_string`. The
sibling route's declaration was not the evidence for widening — a live
server was; §3.7's own warning against trusting a neighbouring route's
declaration still stands.

`POST /custom_templates/create/file` was probed in the same pass and also
answers `200` for `Type: 3`, which matches the enum it already declares.

### 6.6 An enum value with a leading space, overriding a clean `$ref` — trimmed in the generator, and by hand in the six values already emitted

**Evidence: vendored spec**, verified 2026-08-14 (wave 1 stage B, task 6);
**fixed** 2026-08-18.

`portaineree.CustomTemplateRelativePathSettings` declares both
`PerDeviceConfigsMatchType` and `PerDeviceConfigsGroupMatchType` as a
`$ref` to `portainer.PerDevConfigsFilterType` — whose own enum is the clean
`["file", "dir"]` — while attaching an inline `enum` of its own as a
sibling of that `allOf`:

```json
"PerDeviceConfigsMatchType": {
  "allOf": [{"$ref": "#/components/schemas/portainer.PerDevConfigsFilterType"}],
  "description": "Per device configs match type",
  "enum": ["file", " dir"],
  "example": "file"
}
```

The second value carries a **leading space**. This is the same class of
defect as §6.1's content type, in a different keyword.

The malformed value was the one that survived resolution, not the clean
one: `cmd/gen_action_inputs/schema.go`'s `resolve()` merges every `allOf`
branch first and then overlays the node's own directly-declared keywords
over the result, because a sibling keyword next to `$ref`/`allOf` is meant
to take precedence. So the generated `EnumParams()` on all three nested
`...EdgeSettingsRelativePathSettings` structs in
`internal/tools/custom_templates/inputs.go` read `{"file", " dir"}`,
verbatim from the document.

**The server does not validate this field at all** — measured 2026-08-18
against a live 2.44.0 Business Edition, by creating four custom templates
through `POST /custom_templates/create/string` that differed only in
`EdgeSettings.RelativePathSettings.PerDeviceConfigsMatchType`:

| Sent | Response | Stored |
|---|---|---|
| `"dir"` | `200` | `dir` |
| `" dir"` | `200` | ` dir` |
| `"file"` | `200` | `file` |
| `"zzz-not-a-value"` | `200` | `zzz-not-a-value` |

So the enum is a **client-side constraint only**: `toolutil.ActionSpec.ValidateInput`
enforces it before the request leaves, and nothing on the server would
catch a wrong value afterwards. That is the real argument for trimming, and
it is stronger than "the server rejects the space" would have been — since
the server catches nothing here, the published enum is the only thing
steering a model, and steering it toward a value with a leading space is
steering it wrong for no benefit. An earlier draft of this section asserted
the server "accepts `dir`, never ` dir`"; the second half of that is false,
and it was written from the document rather than from a server.

Only the Business Edition document defines this schema; Community Edition
has neither it nor `portainer.PerDevConfigsFilterType`.

**What is true now.** `resolve()` no longer transcribes the typo. Every
enum value entering the generator passes through `normaliseEnumValues`
(`cmd/gen_action_inputs/schema.go`), which strips leading and trailing
whitespace off every string value as it is read — the single place enum
values enter this generator, so every published enum is built from what it
returns. The rule is general rather than a carve-out for this one schema:
surrounding whitespace on an enum value is never intentional, and this
occurrence proves it by contradicting the very component the same node
references. Two outcomes are deliberate rather than silent:

- **An empty string is preserved untouched.** It is a real member in both
  vendored documents — `portaineree.EndpointOperationStatus` spells its
  "done" state as `""`, named `EndpointOperationStatusDone` by that
  schema's own `x-enum-varnames`, and `portainer.UserThemeSettings.color`
  offers `""` for "no theme chosen". Only a value that is non-empty in the
  document and becomes empty once trimmed (a single space, a tab) is
  refused, as an ordinary resolution error that reports and skips the
  operation carrying it: emitting `""` there would invent the one string
  that already means something else in this very specification. Neither
  vendored document contains such a value today, so that half is a guard,
  not a repair.
- **A value colliding with an earlier one after trimming is dropped**,
  first-occurrence order preserved — the same repair `cmd/fetch_spec`'s
  `deduplicateEnums` already applies to values that were exact duplicates
  as published, one stray space later.

The six values already emitted into
`internal/tools/custom_templates/inputs.go` were corrected **by hand**, not
by regenerating: that file carries wave 1 stage B's measured hand edits
(`Platform` made non-pointer and `SourceID` optional on the two JSON create
routes, against the vendored `required` arrays — see §3.7), and a
regeneration silently discards them. Confirmed rather than assumed: a
scratch `-allow-overwrite` run into a copied tools tree re-emits
`CustomTemplateCreateRepository`'s `Platform` as an optional pointer again.
That same scratch run is how the generator half was proven — it emits
`{"file", "dir"}` at all three sites, with no ` dir` anywhere in the
emitted tree — and the run itself was thrown away rather than committed.

**Why the generator and not `cmd/fetch_spec/normalise.go`**, which is where
this section previously said such a rule belonged, alongside the
duplicate-enum and `*/*` rules and next to §6.1's content type. Two
reasons. `normalise` runs at *fetch* time, so a rule added there changes
nothing about the document already vendored: the correction would not exist
until somebody re-fetched 2.44.0, and the six emitted values would stay
wrong in the meantime. And reading is where every consumer converges —
`resolve()` is the only place an enum keyword becomes a published
constraint, whatever route the document arrived by, so a rule there cannot
be bypassed by a spec that was hand-patched or vendored some other way.
The old objection — that trimming would put the generator "in the business
of guessing which of a spec's two disagreeing declarations is the intended
one" — does not apply: the trim does not adjudicate between the `$ref`'s
enum and the sibling's on the merits. The `allOf` precedence rule is
untouched and the sibling still wins; it is simply no longer malformed when
it does. Adding the same rule to `normalise.go` remains harmless and is now
redundant.

**No spec-drift allow-list entry accompanies this**, deliberately.
`cmd/audit_spec_drift` compares only an operation's *top-level* properties:
`internal/specdiff.ShapeFromCatalog` reads `schema["properties"]` one level
deep, and its spec-side counterpart does the same, so a nested object's own
fields are outside the audit's field set entirely, in both directions.
Measured, not inferred: a probe entry for
(`CustomTemplateCreateRepository`, `perDeviceConfigsMatchType`) added to
`api/spec-drift-allowlist.yaml` produced `0 gating finding(s), 1 stale
allow-list entr(y/ies)` and exit status 2 — itself a build error, which is
exactly why §6.5's enum-widening entry could be added and this one cannot.
The probe was removed. Should the drift audit ever learn to recurse into
nested objects, this divergence will need an entry then, citing this
section.

Guarded by `TestUnit_Resolve_EnumValueWithSurroundingWhitespace_IsTrimmed`,
`TestUnit_Resolve_WhitespaceOnlyEnumValue_IsRefused`,
`TestUnit_Resolve_EnumValuesCollidingAfterTrim_AreDeduplicated` and
`TestUnit_AssembleOperationFields_CustomTemplateRelativePathSettings_PublishesCleanDir`
(`cmd/gen_action_inputs/enum_trim_test.go`) for the generator, and by
`TestUnit_RegisteredSpecs_NoPublishedEnumValueCarriesWhitespace`
(`internal/wiring/nested_enum_test.go`), which sweeps every registered
action's published schema at every depth rather than only these six values.
Each was confirmed failing with its fix reverted before being kept.

### 6.7 `CustomTemplateList.type` is declared `explode: false`, and the server cannot parse what that produces — fixed by a hand-written handler

**Evidence: measured** against a live Portainer 2.44.0, Community and
Business Edition alike; recorded 2026-08-18 (wave 1 stage B, task 7).

`GET /custom_templates` declares its required `type` parameter as an array
with `style: form` and **`explode: false`**. That combination means one
comma-joined value, and the generated client encodes it exactly so — via
`runtime.StyleParamWithOptions("form", false, "type", ...)` in
`NewCustomTemplateListRequest`. Portainer's own handler then parses each
value with `strconv.Atoi` and fails, while the repeated form it expects
answers 200:

```text
GET /api/custom_templates?type=2
200 [...]

GET /api/custom_templates?type=1,2,3
400 {"message":"Invalid Custom template type",
     "details":"Failed parsing template type: strconv.Atoi: parsing \"1,2,3\": invalid syntax"}

GET /api/custom_templates?type=1&type=2&type=3
200 [...]
```

Neither implementation is wrong on its own terms: the client encodes what
the document declares, and the server implements what `explode: true` would
have declared. The document is wrong, and the cost lands squarely on the
catalog, because `type` is `required: true`: there is no "omit it and get
everything" escape, so a list action published on the generated client works
for exactly one stack type and fails on the most obvious call a list action
has.

**What the catalog does about it.** `custom_templates.list` is hand-written
(`internal/tools/custom_templates/handlers.go`, declared in
`handWrittenSpecs()` beside `custom_templates.create_file`) and builds the
query itself with `url.Values`, which renders one `type=` per value — the
form the server accepts. Everything else about the action is unchanged: the
same `customTemplateListInput` (`type` as `[]int`, `edge` optional), the same
`redactCustomTemplateList` wrapper over a response that still carries
`GitConfig`, the same edition and flags.

**No allow-list entry, and that is a considered answer rather than an
omission.** `api/spec-drift-allowlist.yaml` excuses differences between the
parameter shape the catalog PUBLISHES and the one the vendored specification
declares; `cmd/audit_spec_drift` compares exactly that. This change alters
neither: the published input is byte-identical to what the generator emitted,
and only the wire encoding of an already-declared array parameter differs.
`make audit-spec-drift` reports no finding for it, and adding an entry
anyway would be reported as stale, which is itself a build error.

Pinned from three directions, because a list call is easy to test in a way
that cannot fail:

- `TestUnit_CustomTemplateList_SendsTheTypeParameterRepeated` asserts the
  literal query string (`type=1&type=2&type=3`, and no comma anywhere),
  including for the single-type call the broken encoding also got right.
- `TestCustomTemplates_List_ReturnsSeveralStackTypesInOneCall`
  (`test/e2e/suite/custom_templates_test.go`) seeds two templates of
  different stack types and requires the one live multi-type call to return
  both, on both editions and all three surfaces — so a handler that quietly
  sent one type and dropped the rest fails it.
- The same test's last subtest calls the GENERATED client against the same
  server at the same moment and requires it to still fail with the
  `strconv.Atoi` message above. If Portainer ever starts accepting the comma
  form, that subtest fails and this section needs revisiting.

Confirmed discriminating: reverting the handler to a comma-joined value
makes the unit test report
`query = "type=1%2C2%2C3", want "type=1&type=2&type=3"` and all six
(edition, surface) pairs of the e2e test fail with the server's own 400.

---

## 7. Adjacent constraint, not an API divergence

Worth knowing when choosing parameter types for a new domain, though it is a
property of the MCP SDK rather than of Portainer: `go-sdk` v1.7.0 decodes
`CallToolParams.Arguments` into `map[string]any`, so **every number any tool
receives loses precision above 2^53**. Measured:
`9223372036854775807` arrives as `9223372036854776000`. This affects all
three surfaces identically and is not avoided by `json.RawMessage`, by a
typed `int64` field (which fails to deserialise outright) or by an explicit
input schema. Harmless for Portainer today — identifiers are small and
timestamps are in seconds — but a domain that accepts a large integer
(nanosecond timestamps, 64-bit identifiers from an external system) must
either say so in the action's description or accept the value as a string.

---

## 8. Open questions this file cannot settle

Recorded so that the next contributor does not mistake an unresolved
question for a settled fact.

1. **A number that does not reconcile.** The source entry reads "four of the
   nine `kubernetes`-tagged divergences". The `kubernetes` tag has **seven**
   divergent operations, not nine; nine is the Community Edition leg total
   (7 `kubernetes` + 1 `helm` + 1 `upload`). The four named operations are
   all genuinely `kubernetes`-tagged, so the list in §1.3 is unambiguous —
   only the denominator in the original wording was wrong.
2. **Two of the four naming mismatches lack a transcript** (§1.3).
   Re-probe `cluster_roles` and `role_bindings` rather than trust them.
3. **The three `addons` operations have no root cause** (§1.6). Licence or
   build gating has not been ruled out.
4. **Four operations remain genuinely unlocated** (§1.5).
5. **`licensesInfo` was never separately measured** for the leak that
   `licensesList` demonstrably has (§4.1). Assume it leaks until measured.
6. **`ecrDeleteTags.RepositoryName`'s original defect is not recoverable**
   from the working notes (§6.4).
7. **The Kubernetes leg has never been probed for route existence** (§1.7).
   The reasoning for skipping it is sound for route existence only; if a
   wave ever finds a route that exists only under a Helm deployment, that
   reasoning is what to revisit first.
8. **A Kubernetes pod that claims `nvidia.com/gpu` still cannot start one**
   (§10.3). Node capacity is reliable; a scheduled GPU workload through
   Kubernetes is not yet, and the root cause is not confirmed.

---

## 9. Tooling/process caveats recorded permanently (not API divergences)

Two findings from the freeze that retired the generated-code freshness check
(P3.2). Neither is a Portainer API divergence — both are properties of this
project's own tooling — but each was, until now, recorded only in a
gitignored working document (`.superpowers/sdd/.../task-4-report.md`) whose
own `.gitignore` excludes it entirely from the repository. A citation to that
path from tracked source (`.github/workflows/ci.yml`,
`internal/toolutil/narrative.go`) dangles the moment this branch merges: the
file that citation points to does not exist in the clone anyone else has.
Recorded here instead, permanently, alongside this file's other durable
findings.

### 9.1 The freshness check's replacement is proven equivalent on one real case, not universally

`.github/workflows/ci.yml` used to regenerate every domain's Input structs
and handlers, then `git diff --exit-code internal/tools/` — failing CI the
moment regeneration produced anything different from what was committed.
That job is retired: domains are scaffolded once (`make scaffold-domain`)
and hand-maintained from then on (see `docs/domain-wave-checklist.md`), so
regeneration no longer runs against an owned domain at all, and a check that
compared regenerated output to committed output would have nothing
meaningful to compare. `make audit-spec-drift` (`cmd/audit_spec_drift`)
replaces it: a standing comparison between every declared action's
parameter shape and the vendored specification operation it names, gating
the build on real drift however it arose.

**What was actually demonstrated**, live, on the one mutation this project's
own history already had a name for (P2's original defect): hand-edited
`internal/tools/registries/actions.go` (`registries/actions.gen.go` at the
time), changing `registryInspect`'s `return redactRegistryInspect(resp.JSON200), nil`
to `return resp.JSON200, nil` — the exact edit that shipped a leaked
credential the first time. Regenerating and diffing (the retired job's own
mechanism, run manually against the mutation) failed exactly as CI would
have. `make audit-spec-drift` against the identical mutation failed too,
naming the same operation: `RegistryInspect (registries.inspect): success
response can carry [AccessToken Password TLSCACert TLSCert TLSKey], but
Handler never calls redactRegistryInspect [GATING]`. Both reverted, both
clean afterward.

That is one demonstrated equivalence, on one mutation, not a proof the two
checks are equivalent on every possible hand edit — see §1 of this
document's own habit of stating what was measured rather than what was
merely plausible. `.github/workflows/ci.yml`'s own comment states the
narrower, correct claim: the replacement covers strictly more against
*specification drift* (a hand edit, or the vendored spec moving out from
under the catalog — either is caught, where the old job only caught a
regeneration disagreeing with what was committed) and strictly less against
*arbitrary hand edits to scaffolded code that introduce no spec drift at
all* — deleting a whole action's declaration from `generatedSpecs()`, for
instance, shrinks `make audit-spec-drift`'s own `Actions audited` count with
no gating finding at all, because this audit iterates the actions the
catalog declares and an absent action is not a shape it can compare (see
`cmd/audit_1to1`'s ratchet for the orthogonal, count-only check that catches
*that* case, imperfectly: it floors a total, so removing one action and
adding an unrelated one passes it silently).

### 9.2 `-allow-overwrite` does not discard a hand edit to an already-owned domain

`scanHandOverrides` (`cmd/gen_action_inputs/handler.go`) treats every
`*.go` file directly inside a domain directory as a hand-written override
except one whose name ends in `.gen.go`. Before the freeze this correctly
separated the generator's own previous output (`actions.gen.go`,
`inputs.gen.go`) from genuinely hand-added files in the same directory
(`system.go`'s hand-declared `SystemNodesCount`, for instance). After the
freeze, a scaffolded domain's owned files are named `actions.go` and
`inputs.go` — no `.gen.go` suffix, by design (see `scaffoldHeader`'s own doc
comment: an owned file is linted and reviewed like any other source file,
which golangci-lint's generated-file heuristic and a `.gen.go` name would
both exempt it from). `scanHandOverrides` was not changed to account for
this, so it now reads a domain's own `actions.go`/`inputs.go` as hand-written
overrides too — which they now, correctly, are.

The consequence: passing `-allow-overwrite` (`FORCE=1` via `make
scaffold-domain`) to force regeneration over a domain that already has
`actions.go`/`inputs.go` does not discard the hand edits made since that
domain was scaffolded, the way `docs/domain-wave-checklist.md` used to
claim. `domainAlreadyScaffolded`'s skip is bypassed, as intended, but
`scanHandOverrides` then reports every mechanically-named operation already
declared in `actions.go` as "already covered by hand-written code" and
generates nothing for it — indistinguishable, from the generator's point of
view, from a domain author who added a genuinely new hand-written handler
under the mechanical name on purpose. `-allow-overwrite` still helps for
scaffolding *new* operations added to a domain's tag since it was last
scaffolded (nothing already declares those OperationIDs), but it does not
re-scaffold what is already there.

This is a known, accepted limitation of the rare, explicit "start over"
path, not a silent trap: the ordinary path this project actually
recommends — scaffold once, hand-edit forever, `-allow-overwrite` never
used — is entirely unaffected by it. Whoever genuinely needs to discard a
domain's accumulated hand edits and start over should delete
`actions.go`/`inputs.go` by hand first (so `domainAlreadyScaffolded` no
longer sees them and `scanHandOverrides` has nothing stale left to
misread), then run `make scaffold-domain` without `FORCE`.

### 9.3 Per-field edition pruning does not recurse into a nested struct

`toolutil.FieldEditions` (`internal/toolutil/edition_fields.go`) inspects
only a struct's own top-level fields: an anonymous embedded field is
flattened into its parent (the same promotion `encoding/json` already
applies), but a *named* nested struct field — the shape every generated
object-typed property takes (`typeOf` in `cmd/gen_action_inputs/fields.go`)
— is never recursed into. `actioncatalog.Build`, which is the only place
this function's result is now consulted (see that package's `Catalog`
doc comment), therefore only ever prunes a whole top-level property or
nothing: when a top-level field is itself tagged `edition:"EE"`, pruning
drops the entire property value, nested subtree included, which is why a
nested `edition:"EE"` tag *under an already-gated parent* is harmless — the
parent's own removal already took it with it. The gap is the opposite
case: a nested struct reached through a field that is **not** itself
gated. A tag on one of that struct's own fields is computed by
`cmd/gen_action_inputs/fields.go`'s `applyFieldEditionGate` (which *is*
nested-inclusive — it walks every `structSpec` the operation produced, not
only the top-level one) and rendered into the generated source as a real
`edition:"EE"` struct tag, but `FieldEditions` never looks at it, so it is
silently never pruned: a Community catalog would publish that nested field
to a server that has never heard of it.

**Measured directly**, by running `cmd/gen_action_inputs` fresh against the
full vendored Business Edition specification (every domain `toolutil.DomainTags`
names, in a scratch `-tools-dir`, so every operation could be generated
regardless of whether its own domain is scaffolded yet) and searching every
emitted `inputs.go` for a struct that (a) is not a top-level `...Input`
struct, (b) carries at least one `edition:"EE"`-tagged field of its own, and
(c) is reached from its operation's top-level Input struct through a field
that is not itself gated: **seven operations**, none in wave 1 —

| Domain | Operation | Nested struct | Inert field(s) |
|---|---|---|---|
| gitops | `GitOpsSourcesTest` | `Authentication` | `provider`, `sharedCredentialId`, `type` |
| gitops | `GitOpsSourcesTestById` | `Authentication` | `provider`, `sharedCredentialId`, `type` |
| kubernetes | `CreateKubernetesNamespace` | `ResourceQuota` | `cpuLimit`, `cpuRequest`, `memoryLimit`, `memoryRequest` |
| kubernetes | `UpdateKubernetesNamespace` | `ResourceQuota` | `cpuLimit`, `cpuRequest`, `memoryLimit`, `memoryRequest` |
| kubernetes | `UpdateKubernetesNamespaceDeprecated` | `ResourceQuota` | `cpuLimit`, `cpuRequest`, `memoryLimit`, `memoryRequest` |
| ldap | `LDAPCheck` | `LDAPSettings` (and its own nested `AdminGroupSearchSettings`/`Kerberos`) | 13 fields, see `cmd/gen_action_inputs/fields.go`'s `RequiresEdition` doc comment |
| users | `UserUpdate` | `Theme` | `subtleUpgradeButton` |

Thirty-two fields across those seven operations, today; a scratch
regeneration also refuses 123 other operations across 24 domains for
reasons unrelated to this one (mostly a missing hand-declared redaction
wrapper, since a wave's domain files do not exist yet in that scratch
copy), so this count is a **floor**, not a ceiling — an operation refused
for an unrelated reason during measurement was never inspected for a
nested tag at all, and may also carry one once its domain's redaction
wrappers are declared for real.

Not fixed in this branch: implementing nested pruning is a larger,
separately-reviewable change (it would need to walk the schema tree and the
Go type in lockstep, handling a pointer, a slice-of-struct and a
map-value-of-struct the way `typeOf` itself does), and none of wave 1's
five domains (`endpoints`, `stacks`, `custom_templates`, `docker`,
`templates`) is affected. Whichever wave scaffolds `gitops`, `kubernetes`,
`ldap` or `users` must either implement nested pruning first or hand-verify
every nested `edition:"EE"` tag that domain's generated inputs carry is
subsumed by an already-gated ancestor field, the same way this table was
produced.

### 9.4 A redaction wrapper typed `any` defeats its own generated guard, vacuously

`cmd/gen_action_inputs/render.go` emits, per domain, a reflective test
(`TestUnit_RedactionGuards_RemoveEveryCredentialShapedField`) that
constructs a zero value of each redaction wrapper's own declared parameter
type via `reflect.New(fn.Type().In(0)).Elem()`, populates it with
`toolutil.PopulateForCredentialAudit`, calls the wrapper, and asserts
nothing credential-shaped survived. `PopulateForCredentialAudit`
(`internal/toolutil/credential.go`) walks every reflect.Kind it can
meaningfully allocate a dummy value for — except `reflect.Interface`, which
it deliberately leaves untouched ("an interface field has no concrete type
to populate, and guessing one would put a value in a response the real
client would never produce").

That single, deliberate exception is also a hole: a redaction wrapper
declared to take `any` instead of its real response type —
`func redactRegistryInspect(r any) any { return r }` in place of the
generated `func redactRegistryInspect(r *apigen.PortainereeRegistry) any` —
makes `fn.Type().In(0)` the empty interface itself. The constructed argument
is then a genuinely empty interface value with no concrete type ever set,
`PopulateForCredentialAudit` does nothing to it (correctly, by its own
stated rule), the wrapper's pass-through returns that same empty value, and
`AssertRedacted` finds nothing populated to report — because there is
nothing there to find. The guard test **passes**, for the one reason it
exists to prevent: a wrapper that does not actually redact anything.
`make audit-spec-drift` does not catch it either, since drift auditing
checks that a redaction function of the expected *name* exists, never that
its signature is the real response type. Confirmed directly: mutating
`internal/tools/registries/registries.go`'s `redactRegistryInspect` to this
`any`-typed pass-through leaves both its generated guard test
(`TestUnit_RedactionGuards_RemoveEveryCredentialShapedField/RegistryInspect`,
`redaction_test.go`) *and* `TestUnit_RedactionGuards_HandlerRedactsCredentialShapedFields/RegistryInspect`
green, and `audit-spec-drift` reports "No drift". Only
`registries_test.go`'s own hand-written, concretely-typed handler tests —
`TestRegistryInspect_ResponseWithPassword_IsRedacted` and
`TestRegistryInspect_ResponseWithNestedManagementCredentials_IsRedacted`,
which build a real HTTP response and call the real `RegistryInspect`
handler rather than reflecting over a wrapper in isolation — still fail.
Mutation reverted, byte-identical, before continuing.

Not introduced by this branch, and not fixed here: it is a structural gap
in the generated guard's own test harness that predates this branch by
several tasks. Recorded because it is not merely theoretical for what
comes next — every one of wave 1's `73` operations that returns a
credential-shaped field needs a declared redaction wrapper
(`checkCredentialRedaction`, `cmd/gen_action_inputs/credential.go`) before
it can generate at all, and the generator has no way to refuse a
hand-written wrapper for being typed `any` instead of the real response
type; a domain author copying the wrong signature by hand would ship
exactly this hole with a fully green build. Closing it durably means either
teaching `PopulateForCredentialAudit`/the generated guard test to refuse an
`any`-typed (or otherwise underspecified) wrapper parameter outright, or
having `cmd/gen_action_inputs` itself check a hand-written wrapper's
declared parameter type against the operation's real response type before
accepting it as satisfying `checkCredentialRedaction`. Either is a
generator/toolchain change large enough to deserve its own review, not a
line-item inside this one.

### 9.5 A nested struct's `EnumParams`/`MinimumParams` was never called

**Measured** 2026-08-14; **fixed** 2026-08-18.

`toolutil.ActionSpec.InputSchema` (`internal/toolutil/schema.go`) reflects
the Input type into a JSON Schema and then applies the two constraint
interfaces the reflector cannot express as struct tags. It used to do that
by asserting on the **top-level Input value only**:

```go
if enumer, ok := s.Input.(EnumParams); ok { ... }
if minimums, ok := s.Input.(MinimumParams); ok { ... }
```

`applyEnumParams` writes each entry into the schema's own `properties`, so
a *nested* struct type's `EnumParams()` was never consulted, however
faithfully `cmd/gen_action_inputs` had generated it: no enum was published
for any field of a nested object, and the same held for `MinimumParams`,
reached by the identical assertion two lines later. This was §9.3's shape —
a generated per-field annotation that is correct in the source and inert at
runtime — for enums and minimums rather than for edition tags.

**The measurement**, taken by calling `InputSchema()` on the registered
`custom_templates.create_repository` spec and reading the result: the
top-level `platform` published `"enum": [1, 2]`, while
`edgeSettings.relativePathSettings.perDeviceConfigsMatchType` — whose
generated nested `EnumParams()` returns `{"file", "dir"}` — published
`{"description": "Per device configs match type", "type": ["null",
"string"]}` and no `enum` keyword at all. Six generated nested
`EnumParams()` methods exist, all in
`internal/tools/custom_templates/inputs.go`: an
`...EdgeSettingsRelativePathSettings` and an `...EdgeSettingsStaggerConfig`
for each of `CustomTemplateCreateRepository`, `CustomTemplateCreateString`
and `CustomTemplateUpdate`. Between them they declare 15 field-level enum
constraints that no surface published and no `ValidateInput` enforced.

**What is true now.** `InputSchema` calls `applyTypeConstraints`
(`internal/toolutil/schema.go`), which walks the Go type tree alongside the
reflected schema and applies the constraints declared by *every* struct
type it reaches. The walk has to match google/jsonschema-go v0.4.3's own
rendering exactly, or a constraint lands on the wrong node or on nothing:
the library inlines everything, with no `$defs` or `$ref` to follow, and
renders a pointer as its element's schema with `"null"` added to `"type"`,
a slice or array through `items`, a string-keyed map through
`additionalProperties`, and an *embedded* struct's fields flattened into
the embedding struct's own `properties`. That last one is why an embedded
type's constraints are applied to the embedding node rather than to a
sub-schema of their own: there is no sub-schema for an embedded struct to
own. `reflect.VisibleFields` already reports every level of embedding as an
anonymous field of the outermost struct, so a struct embedded inside an
embedded struct is reached without recursing through it. Property names are
computed the way the library's own `fieldJSONInfo` computes them, and a
constraint declared on a pointer receiver is honoured as well as one on a
value receiver. The top-level refusal travels down with the constraint: a
nested entry naming a property its own struct does not have is an error,
named together with the property path that led to it, not a silent skip.

So `custom_templates.create_repository` now publishes
`"enum": ["file", "dir"]` for
`edgeSettings.relativePathSettings.perDeviceConfigsMatchType`, and
`ValidateInput` — which resolves the same map — refuses a value outside it
before any handler runs. Both consequences this section used to state have
reversed: a caller now gets help filling `staggerOption` and
`perDeviceConfigsMatchType`, and §6.6's malformed `" dir"` is no longer
harmless-by-invisibility. That is precisely why §6.6 had to be, and was,
fixed first: registering this change over the untrimmed values would have
started publishing ` dir` to models as the only spelling this catalog
admits.

`MinimumParams` is fixed by the same walk, but that half is **currently
unexercised by any real type**: every `MinimumParams()` in the tree today
is on a top-level `...Input` type, so no generated nested one exists to
exercise it. It is covered by fixture types in
`internal/toolutil/nested_constraints_test.go` and by nothing else.

Guarded by
`TestUnit_InputSchema_NestedStructConstraints_ArePublishedAtEveryShape`
(a bare struct, a pointer, two pointer levels, a slice of structs, a slice
of pointers, an array, a map value, an embedded struct and a
doubly-embedded struct),
`TestUnit_InputSchema_NestedConstraintNamingAnUnknownProperty_IsRefused`,
`TestUnit_InputSchema_NestedConstraintOnAPointerReceiver_IsStillApplied`,
`TestUnit_InputSchema_NestedStructWithNoConstraints_PublishesNone` and
`TestUnit_ValidateInput_NestedEnum_RefusesAnOutOfEnumValue`
(`internal/toolutil/nested_constraints_test.go`), plus
`TestUnit_RegisteredSpec_NestedEnumConstraints_ArePublishedAndTrimmed`
(`internal/wiring/nested_enum_test.go`), which asserts the measured
observable on the registered specs themselves. Reverting the fix to the old
top-level-only assertion was confirmed to fail all of them except the two
embedded cases, which pass either way because Go promotes an embedded
type's methods onto the embedding type; those two are kept as guards on the
opposite mistake, a walk that looks for a sub-schema an embedded struct
does not have.

§9.3's nested *edition* pruning remains open. It needs the same kind of
lockstep walk, over `toolutil.FieldEditions` and `actioncatalog.Build`
rather than over these two constraint interfaces, and none of wave 1's
domains is affected by it — so it was left alone here rather than folded
into a change that is already two coupled defects wide.

---

## 10. GPU passthrough through nested virtualisation (not an API divergence, but a permanent finding)

Two more findings from the remote-GPU-estate work (P6), in the same spirit as
§9: neither is a defect in the Portainer API, but each cost real debugging
time and belongs in one place rather than being rediscovered by the next
person who touches the estate's GPU support. Measured 2026-08-05 against a
remote Docker host with an NVIDIA GeForce RTX 4060 and driver 570.172.08 (see
`test/e2e/scripts/lib.sh`, `test/e2e/docker-compose.gpu.yml` and
`test/e2e/k8s/nvidia-device-plugin.yaml` for the code these findings shaped).

### 10.1 The estate's dind cannot host the NVIDIA container toolkit

Nested GPU containers — Portainer, inside the estate, creating a container
that reaches a real card — cannot use `--gpus`.

- `--gpus all` on the **dind container itself** works: `/dev/nvidia0`,
  `nvidiactl`, `nvidia-uvm`, `nvidia-uvm-tools` and `nvidia-caps` appear
  inside it, along with `/usr/bin/nvidia-smi` and the driver libraries under
  `/usr/lib/x86_64-linux-gnu`.
- `--gpus all` **one level deeper**, from the daemon inside the dind, fails
  with `could not select device driver "" with capabilities: [[gpu]]`. That
  daemon has only `runc`; the `nvidia` runtime lives on the outer host.
- Installing the toolkit inside the dind is not available. `docker:28-dind` is
  Alpine; `apk` has no `nvidia-container*` package; and the binaries cannot be
  copied in, because `nvidia-container-cli` and `nvidia-cdi-hook` are linked
  against glibc. With `gcompat` the loader resolves but NVML does not:
  `Error relocating /usr/bin/nvidia-cdi-hook: nvmlDeviceGetRowRemapperHistogram: symbol not found`.

CDI is the way through, with one modification. A specification from
`nvidia-ctk cdi generate`, copied into the dind's `/etc/cdi` (`gpu_cdi_spec`
in `test/e2e/scripts/lib.sh`), makes the inner daemon discover
`nvidia.com/gpu=0`, `nvidia.com/gpu=GPU-…` and `nvidia.com/gpu=all` — but a
container requesting one still fails, at `error running createContainer hook
#0: fork/exec /usr/bin/nvidia-cdi-hook: no such file or directory`, because
every hook the generator emits invokes that same glibc binary. Stripping the
`hooks:` blocks (`strip_cdi_hooks`, same file) leaves a specification that
needs no binary at all, and the nested container then gets the device nodes.

Two consequences worth knowing before reading a confusing failure:

1. **`ldconfig` first.** The stripped hooks are what create the SONAME
   symlinks and refresh the ldcache, so `nvidia-smi` in a nested container
   fails with `couldn't find libnvidia-ml.so` even though the library is
   mounted. A plain `ldconfig` before the workload fixes it — measured:
   `NVIDIA GeForce RTX 4060, 570.172.08, 8188 MiB, 0 %`.
2. **Nothing is lost on Portainer's side.** A CDI request still populates the
   field Portainer reads:
   `HostConfig.DeviceRequests = [{"Driver":"cdi","Count":0,"DeviceIDs":["nvidia.com/gpu=all"],"Capabilities":null,"Options":null}]`.
   So `dockerContainerGpusInspect` sees a real request, and the estate's GPU
   coverage is not weakened by taking the CDI route. Verified end to end,
   through Portainer's own Docker proxy API on the real remote estate: a
   container created via `POST /docker/{env}/containers/create` reports
   `NVIDIA GeForce RTX 4060, 8188 MiB`, and its inspect response's
   `HostConfig.DeviceRequests` carries exactly that `Driver: "cdi"` entry
   (`test/e2e/suite/gpu_test.go`,
   `TestE2E_GPU_PortainerRunsAContainerOnTheRealCard`).

Separately, and useful for GPU-less machines: creating (not starting) a
container with `--gpus all` succeeds with no runtime at all, recording
`{"Driver":"","Capabilities":[["gpu"]]}`. Any assertion that only needs the
*metadata* path — which is most of Portainer's GPU surface — can therefore run
anywhere.

### 10.2 The Kubernetes leg fails the same way, and needs the opposite fix

The k3s node in a k3d cluster has the same shape as the dind — devices and
driver libraries injected by `--gpus all`, no toolkit — but the fix is not the
same, and using the dind's fix here does nothing.

- **CDI pod annotations do not work.** A pod annotated
  `cdi.k8s.io/gpu: "nvidia.com/gpu=all"`, on a node with a hookless CDI
  specification installed at `/etc/cdi/nvidia.yaml`, started with no GPU at
  all: no `nvidia-smi`, no device nodes, exit 1. containerd 2.x (k3s v1.35's
  config is `version = 3`) takes CDI devices through the CRI field the kubelet
  fills from a device plugin, not from annotations.
- **`nvidia.com/gpu` capacity needs the device plugin, and the published
  manifest does not work here.** It assumes the NVIDIA runtime is the node
  default. `test/e2e/k8s/nvidia-device-plugin.yaml` sets four environment
  variables, each found by watching the previous attempt fail:
  - `DEVICE_LIST_STRATEGY=cdi-cri` — CDI devices only, through the CRI field
    the kubelet fills from the plugin.
  - `NVIDIA_DRIVER_ROOT=/` — the driver root as the **node** sees it, since
    the CDI specification the plugin generates writes host paths relative to
    this value.
  - `CONTAINER_DRIVER_ROOT=/driver-root` — where that same root is mounted
    inside the plugin's own pod (a `/` hostPath). With only a lib directory
    the plugin dies on `failed to locate libcuda.so.570.172.08`; with the
    driver root set to the in-pod path instead of the node path, containers
    fail on `failed to stat CDI host device "/driver-root/dev/nvidia-uvm"`.
  - `LD_LIBRARY_PATH=/host-libs`, paired with a fourth hostPath
    (`/usr/lib/x86_64-linux-gnu`, mounted read-only at `/host-libs`) —
    **added after the first real-hardware run, and load-bearing.** Without
    it both plugin pods crash-loop with `error starting plugins: unable to
    validate flags: CDI --device-list-strategy options are only supported on
    NVML-based systems` — a message that names the CDI flag, not the missing
    library path, so nothing about it points here. `CONTAINER_DRIVER_ROOT`
    alone is not enough: the plugin still resolves its shared libraries
    through the dynamic linker's own search path, which a bind-mounted root
    at a non-standard mount point (`/driver-root`, not `/`) does not
    populate. Measured directly, live: both pods this DaemonSet creates
    (`k3d-up.sh` runs `--agents 1`, so a two-node cluster) went `Ready` and
    advertised `nvidia.com/gpu: 1` only once this variable and the
    `host-libs` volume/mount were both added.
- **The plugin's own CDI specification has hooks**, and they invoke
  `/usr/bin/nvidia-ctk`, which the k3s node cannot provide — that image has no
  standard libc layout (neither `/lib/ld-musl*` nor
  `/lib64/ld-linux-x86-64.so.2`), and even the `nvidia-smi` that `--gpus all`
  injects is not executable in it. A two-line `#!/bin/sh` / `exit 0` shim at
  that path is enough: the hook only has to exist and exit zero, because the
  devices and mounts come from the specification, not the hook. On the
  **single-node** cluster this reconnaissance used, this was enough: a pod
  requesting `nvidia.com/gpu: 1` reported `NVIDIA GeForce RTX 4060, 8188 MiB`.
  That result did **not** reproduce once the shipped harness's two-node
  (`--agents 1`) cluster was exercised for real — see §10.3, which is the
  authoritative, current account of running a GPU workload through
  Kubernetes. The shim is still installed (it costs nothing on a node that
  never gets that far), but do not read this bullet as proof a scheduled pod
  gets the card; it is not.
- **The hand-written hookless specification is not needed on the k3s node**,
  as far as this single-node reconnaissance could tell: removing it and
  re-running the pod still succeeded there, and the device plugin generates
  the specification this leg uses on its own. This, too, predates the
  two-node reproduction failure in §10.3 and has not been re-confirmed
  against it — recorded here as the reasoning that shaped the DaemonSet's
  design, not as a currently-verified fact about pod scheduling.

The shim's cost is that `update-ldcache` and `create-symlinks` never run, so
any GPU workload must call `ldconfig` before touching the driver libraries.
A manifest copied from NVIDIA's documentation will fail with
`couldn't find libnvidia-ml.so` and the reason will not be visible in the
error.

### 10.3 Open item: a pod that claims the GPU still cannot start one

**Node capacity works. Running a GPU workload through Kubernetes does not,
yet.** With the device plugin healthy and both nodes advertising
`nvidia.com/gpu: 1` — the fact `GetKubernetesGPUInfo` will read, once that
domain exists; `test/e2e/suite/gpu_kubernetes_test.go`'s
`TestE2E_GPU_KubernetesNodeAdvertisesTheCard` asserts exactly this and
passes — a pod that *requests* `nvidia.com/gpu: 1` still fails at container
creation:

```
unresolvable CDI devices k8s.device-plugin.nvidia.com/gpu=<uuid>
```

Measured on the real remote host, on both nodes: `/etc/cdi` is empty and
`/var/run/cdi` does not exist, so the device plugin's own generated CDI
specification is not landing where containerd reads it. This did not
reproduce during the reconnaissance that produced §10.2's DaemonSet, where a
CDI specification had already been hand-installed into `/etc/cdi` before the
plugin ever ran — the working hypothesis is that the plugin's write only
succeeds, or is only picked up, once that directory already has content, but
this has not been confirmed by reproducing the reconnaissance's starting
state and watching the plugin's own write succeed or fail.

Whoever picks this up next should start there: does the plugin actually
write `/etc/cdi/k8s.device-plugin.nvidia.com-gpu.json` at all (permissions?
does the hostPath mount even allow it to?), and does containerd's own
configuration point at the same directory the plugin writes to.

**A second, unverified candidate cause**, not yet ruled out and not
mutually exclusive with the one above: until this branch's own review, the
`nvidia-ctk` shim (§10.2's two-line `#!/bin/sh` / `exit 0` file) was
installed on `k3d-<cluster>-server-0` only, never on the cluster's agent
node — a gap fixed alongside this write-up. If the pod whose failure is
quoted above happened to schedule on the node without the shim, the
`unresolvable CDI devices` error could be this gap surfacing under a
different name, rather than (or in addition to) the empty-`/etc/cdi`
hypothesis. This has not been tested either way: the fix landed without a
hardware run to confirm or rule it out, so it is recorded here as a second
hypothesis, not a second finding. Whoever reproduces this next should check
which node the failing pod actually landed on before assuming the first
hypothesis is the whole story.

Until this is resolved, treat `nvidia.com/gpu` capacity on the Kubernetes leg as
reliable, and a scheduled GPU workload there as unverified.
