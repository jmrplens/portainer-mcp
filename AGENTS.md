# portainer-mcp — Project Intelligence

MCP server in Go exposing the Portainer REST API. Rewrite in progress: the
canonical action catalog, generated client and tool surfaces described in the
design specification are being built phase by phase.

> This is the canonical AI context. `CLAUDE.md` mirrors it.

## Current state

Wave 2 stage A — the access model — is complete. Twelve domains are declared
— `system`, `tags`, `registries`, `docker`, `custom_templates`, `stacks`,
`endpoints`, `endpoint_groups`, `teams`, `team_memberships`, `roles` and
`resource_controls` — for **109 actions**, covering 108 of Business
Edition's 442 operations and 86 of Community Edition's 252
(`api/coverage-baseline.yaml` is the ratchet, `make audit-1to1` the "are we
done" report; take every figure from the tool, not from this paragraph). All
three tool surfaces (`dynamic`, `meta`, `individual`) are live and exercised
end to end against a real estate.

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
- `plan/`, `docs/superpowers/`, `.env` and `*.license` are gitignored working
  artefacts and must never be committed.

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
