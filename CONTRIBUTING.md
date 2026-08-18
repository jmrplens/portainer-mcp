# Contributing

Thanks for taking a look. This is a rewrite in progress: the action catalog is being built
domain by domain, and most of what follows exists to keep that process honest rather than fast.

## Getting set up

| Tool | Version | For |
|---|---|---|
| [Go](https://go.dev/doc/install) | 1.26.6+ | building and testing |
| Make | any | everything below |
| [Docker](https://docs.docker.com/get-docker/) + Compose | 20+ | the end-to-end estate |
| [k3d](https://k3d.io/), kubectl, [helm](https://helm.sh/) | — | only for the Kubernetes leg of the estate |

```bash
make build     # → dist/portainer-mcp
make test      # unit tests
make check     # format, lint, vulncheck, test — exactly what CI runs
```

## The one rule that breaks everything

**Nothing writes to stdout except the MCP transport.** Standard output carries JSON-RPC; a stray
`fmt.Println` corrupts the protocol and the failure looks like a client bug. Log through
`internal/logging`, which is pinned to stderr. CI has a dedicated job for this.

## How work is judged here

Two conventions matter more than style, and reviews are strict about both.

**A guard is not proven by a passing test. It is proven by mutating what it guards and watching
the test fail.** When you add an assertion, break the thing it protects, run the test, and keep
the failure message. If the test still passes, the assertion is decoration. Check that your
mutation actually landed in the file before trusting the result — a mutation that silently fails
to apply produces a passing test that reads exactly like a non-discriminating one, and that has
fooled people here more than once.

**Nothing is verified without the real infrastructure standing.** A unit test does not verify that
an action works against Portainer. Bring the estate up, call the thing, and record the literal
request and response. Where a measurement contradicts Portainer's own documents — which happens
often — the measurement wins and goes in [`docs/api-divergences.md`](docs/api-divergences.md) with
the evidence beside it. Never present an inference as a measurement; if something was not probed,
say it was not probed.

## Adding a domain

Actions are generated from the vendored OpenAPI documents in `api/specs/`, not hand-written, and
the generator refuses rather than guesses when a specification is wrong about something. The full
procedure — with the traps, in order — is in
[`docs/domain-wave-checklist.md`](docs/domain-wave-checklist.md). Read it before scaffolding
anything; several of its steps exist because skipping them produced a green build over broken code.

The audits are the safety net:

```bash
make audit-1to1           # which API operations the catalog covers
make audit-spec-drift     # has any action drifted from the specification it was generated from?
make audit-e2e-gaps       # which actions no e2e test touches
make audit-spec-reality   # does the vendored specification match a live server?
```

`audit-e2e-gaps` exits 0 by design — it informs, it does not gate. Read it anyway; a domain has
shipped with most of its actions untouched while the catalog reported full coverage.

## End-to-end tests

`make e2e-up` provisions a disposable estate (Community and Business Edition servers, an agent, an
edge agent, a git server, optionally Kubernetes) on its own Docker-in-Docker daemon, so it never
touches the host's containers. `make test-e2e` runs against it, `make e2e-down` removes it.

Remote runs have their own targets and never happen by accident — see the README.

## Conventions

- Conventional commits.
- Test names read `TestUnit_<Scenario>_<ExpectedResult>`; table-driven with `t.Run`.
- Errors carry context: `fmt.Errorf("context: %w", err)`.
- English throughout.
- Never begin a comment line with the word `nolint`, even in prose — golangci-lint parses it as a
  directive and, binding to no declaration, silences every linter in the whole file.

## Reporting something

Bugs and questions: open an issue. Security vulnerabilities: **do not** open a public issue — see
[SECURITY.md](SECURITY.md).
