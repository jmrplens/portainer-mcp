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
  devices and mounts come from the specification. Measured: a pod requesting
  `nvidia.com/gpu: 1` then reports `NVIDIA GeForce RTX 4060, 8188 MiB`.
- **The hand-written hookless specification is not needed on the k3s node.**
  Measured by removing it and re-running the pod, which still succeeded: the
  device plugin generates the specification this leg uses. Keeping both would
  be two sources of truth for the same devices.

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
configuration point at the same directory the plugin writes to. Until this
is resolved, treat `nvidia.com/gpu` capacity on the Kubernetes leg as
reliable, and a scheduled GPU workload there as unverified.
