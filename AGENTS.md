# portainer-mcp — Project Intelligence

MCP server in Go exposing the Portainer REST API. Rewrite in progress: the
canonical action catalog, generated client and tool surfaces described in the
design specification are being built phase by phase.

> This is the canonical AI context. `CLAUDE.md` mirrors it.

## Current state

P0 (foundations) is complete: module, configuration, logging, MCP server
skeleton over stdio, and CI. The Portainer client, action catalog and tool
surfaces do not exist yet — do not assume any tool beyond
`portainer_mcp_status`.

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
    internal/config/       .env → environment → flags resolution
    internal/logging/      slog logger, stderr only
    internal/version/      ldflags-injected build metadata

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
