# portainer-mcp — Project Intelligence

MCP server in Go that exposes the Portainer REST API as a catalog of named
actions. A model calls `teams.create` or `stacks.deploy` rather than an HTTP
route: the server validates the input against a published schema, calls
Portainer, and redacts credential-shaped fields out of the response.

The two safety modes are not the same mechanism, and the difference is worth
knowing. Read-only drops every mutating action from the catalog outright, so
a model never sees one. Safe mode keeps them and intercepts the call,
answering with a preview of what it would have done.

> Canonical source: `AGENTS.md`. This file is the Claude-formatted version.

## The action catalog

Actions are grouped into domains. Twelve are declared — `system`, `tags`,
`registries`, `docker`, `custom_templates`, `stacks`, `endpoints`,
`endpoint_groups`, `teams`, `team_memberships`, `roles` and
`resource_controls` — for **109 actions**.

They cover part of Portainer's REST surface, not all of it: 108 of Business
Edition's 442 operations and 86 of Community Edition's 252.
`api/coverage-baseline.yaml` is the ratchet that stops those numbers falling,
and `make audit-1to1` reports which operations are still uncovered — take
every figure from the tool, never from this paragraph.

All three tool surfaces (`dynamic`, `meta`, `individual`) are live and
exercised end to end against a real estate.

A domain is scaffolded once from the vendored specification and owned as
ordinary Go source from then on; nothing regenerates it. See
`docs/domain-wave-checklist.md` for the procedure and
`docs/api-divergences.md` for every recorded disagreement between Portainer
and the documents that describe it.

## Build & run

    make build            # → dist/portainer-mcp
    make test             # unit tests
    make check            # format, lint, vulncheck, test

    PORTAINER_URL=https://portainer.example.com \
    PORTAINER_TOKEN=ptr_... dist/portainer-mcp

## Hard constraints

- **Nothing writes to stdout except the MCP transport.** Standard output
  carries JSON-RPC; a stray `fmt.Println` corrupts the protocol. CI enforces
  this. Log through `internal/logging`, which is pinned to stderr.
- Module path is `github.com/jmrplens/portainer-mcp`, no version suffix.
- MCP SDK is `github.com/modelcontextprotocol/go-sdk` v1.7.0.
- All project artefacts are written in English.
- **Plans and progress live outside the repository.** `plan/` and
  `docs/superpowers/` are gitignored, as are `.env` and `*.license`. What is
  committed is documentation for developers and for users of the MCP — never
  this project's own planning, progress or working notes. A fact worth
  keeping goes to `docs/api-divergences.md`, complete enough to stand alone;
  the work built on that fact stays local.

## Layout

    cmd/portainer-mcp/     entry point, flags, transport, tool files
    cmd/gen_action_inputs/ scaffolds a domain from the vendored specification
    cmd/audit_*/           the standing audits; CI gates on spec-drift and the 1:1 ratchet
    internal/config/       .env → environment → flags resolution
    internal/logging/      slog logger, stderr only
    internal/portainer/    HTTP client; gen/ is the oapi-codegen client
    internal/tools/        one package per domain, plus the three tool surfaces
    internal/toolutil/     ActionSpec, schema, redaction, parameter guidance
    internal/wiring/       builds the catalog and the MCP server
    internal/version/      ldflags-injected build metadata
    test/e2e/              live-estate suite (build tag `e2e`) and its harness

## Configuration

| Variable | Default | Meaning |
|---|---|---|
| `PORTAINER_URL` | — | Portainer server URL (required in stdio mode) |
| `PORTAINER_TOKEN` | — | API token (required in stdio mode) |
| `PORTAINER_SKIP_TLS_VERIFY` | `false` | Skip TLS verification for self-signed certificates |
| `TOOL_SURFACE` | `dynamic` | `dynamic`, `meta` or `individual` |
| `PORTAINER_READ_ONLY` | `false` | Disable every mutating tool |
| `PORTAINER_SAFE_MODE` | `false` | Intercept mutating tools and return a preview |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error` |

Flags override environment variables, which override `.env`.

## Conventions

- Test naming: `TestUnit_Scenario_ExpectedResult`, table-driven with `t.Run`.
- Errors wrapped with context: `fmt.Errorf("context: %w", err)`.
- Conventional commits.
