# portainer-mcp-enhanced — Project Intelligence

MCP (Model Context Protocol) server in Go that connects AI assistants to Portainer, enabling container management through natural language. Exposes 98 granular tools (grouped into 15 meta-tools by default) covering environments, stacks, Docker, Kubernetes, users, teams, registries, and more.

> This file is the canonical source of project intelligence. `CLAUDE.md` and `.github/copilot-instructions.md` are derived from it.

---

## Build & Run

```bash
# Build
make build                              # → dist/portainer-mcp-enhanced
make PLATFORM=linux ARCH=amd64 build   # cross-compile

# Test
make test                   # unit tests (no external deps)
make test-integration       # requires Docker + Portainer container
make test-all               # unit + integration
make test-coverage          # unit tests with coverage report

# Lint & format
make fmt                    # gofmt -s -w .
make vet                    # go vet ./...
make lint                   # vet + additional checks

# Run
dist/portainer-mcp-enhanced \
  --server https://portainer.example.com \
  --token <api-token>

# MCP Inspector (interactive debugging)
make inspector
```

### CLI Flags

| Flag | Description |
|------|-------------|
| `--server` | Portainer server URL (required) |
| `--token` | API authentication token (required) |
| `--tools` | Path to tools.yaml file (optional, uses embedded default) |
| `--read-only` | Disable write operations |
| `--granular-tools` | Expose all 98 individual tools instead of 15 meta-tools |
| `--disable-version-check` | Skip Portainer version compatibility check |
| `--skip-tls-verify` | Skip TLS certificate verification |

---

## Architecture

```
cmd/
  portainer-mcp-enhanced/   CLI entry point, flags, version via ldflags
  token-count/              Token counting utility for tools YAML
internal/
  mcp/                      Core: server, handlers, metatool system, client_interfaces.go
  tooldef/                  YAML tool definitions → MCP tool structs (embedded at build)
  k8sutil/                  Kubernetes response stripping utilities
pkg/
  portainer/
    client/                 HTTP client wrapper for Portainer API
                            adapter.go + adapter_<domain>.go (16 domain files)
    models/                 Local data models + ConvertXxx() from raw API models (35 files)
  toolgen/                  Tool YAML loader + parameter parsing
tests/
  integration/              Docker-based integration tests
  live/                     Tests against real Portainer instance
docs/                       Starlight/Astro documentation site
tools.yaml                  Declarative tool definitions (v1.2 format, embedded at build)
```

---

## Key Patterns

### Meta-tool System

`metatool_registry.go` defines 15 groups that aggregate 98 tools behind an `action` enum parameter. Default mode uses meta-tools; `--granular-tools` exposes individual tools.

Groups: `manage_environments`, `manage_stacks`, `manage_access_groups`, `manage_users`, `manage_teams`, `manage_docker`, `manage_kubernetes`, `manage_helm`, `manage_registries`, `manage_templates`, `manage_backups`, `manage_webhooks`, `manage_edge`, `manage_settings`, `manage_system`.

### YAML-Driven Tools

Tool definitions live in `tools.yaml`, parsed by `internal/tooldef/`. Tool names are constants in `internal/mcp/schema.go` (e.g., `ToolListUsers = "listUsers"`). Each handler references its tool by constant name via `s.addToolIfExists(ToolName, s.HandleFunc())`.

### Handler Pattern

Each domain has paired files: `<domain>.go` + `<domain>_test.go` in `internal/mcp/`. Handlers follow:

```go
func (s *PortainerMCPServer) HandleXxx() server.ToolHandlerFunc {
    return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        parser := toolgen.NewParameterParser(request)
        // parse params, call s.cli.Method(), return jsonResult() or NewToolResultText()
    }
}
```

### Two-Layer Model

- **Raw Models** (`github.com/portainer/client-api-go/v2/pkg/models`) — direct API mapping, imported as `apimodels`
- **Local Models** (`pkg/portainer/models`) — simplified structs imported as `models`, with `ConvertXxx()` functions
- **Client Wrapper** (`pkg/portainer/client`) — transforms between raw and local models

Handlers never touch raw models. The client layer owns all conversion.

### Interface-Based Client

`PortainerClient` interface in `server.go` composes 18 domain-specific sub-interfaces defined in `client_interfaces.go`:

| Interface | Domain |
|-----------|--------|
| `TagClient` | Environment tags |
| `EnvironmentClient` | Environments (endpoints) |
| `EnvironmentGroupClient` | Environment groups |
| `AccessGroupClient` | Access groups (edge groups) |
| `StackClient` | Edge stacks + regular stacks |
| `TeamClient` | Teams and membership |
| `UserClient` | User accounts |
| `SettingsClient` | Server settings + SSL |
| `TemplateClient` | App templates + custom templates |
| `DockerClient` | Docker API proxy + dashboard |
| `KubernetesClient` | Kubernetes API proxy + dashboard |
| `RegistryClient` | Container registries |
| `BackupClient` | Backups + S3 |
| `WebhookClient` | Webhooks |
| `EdgeClient` | Edge jobs + update schedules |
| `HelmClient` | Helm repos, charts, releases |
| `AuthClient` | Authentication |
| `SystemClient` | Version, status, roles, MOTD |

All handlers use `s.cli` — never direct HTTP calls. Tests mock this interface.

### Docker/K8s Proxy

Shared parameter parsing via `parseProxyParams()` and `readProxyResponse()` in `utils.go`. Direct API pass-through with 10MB response size limit (`maxProxyResponseSize`). Path traversal (`..`) is rejected. Handlers in `docker.go` and `kubernetes.go`.

### Version Validation

- `MinimumToolsVersion = "v1.0"` — minimum tools.yaml version
- `SupportedPortainerVersion = "2.39.1"` — required Portainer version (major.minor must match, patch is flexible)

---

## Code Style

- **Go 1.24+**, `CGO_ENABLED=0` — static binary
- **Formatting**: `gofmt -s` — run before every commit
- **Static analysis**: `go vet ./...` — must be clean before committing
- **Error handling**: `fmt.Errorf("context: %w", err)` — always wrap with operation context
- **Logging**: `github.com/rs/zerolog` — structured fields, never `fmt.Println`, never log to stdout (MCP transport uses stdout)
- **MCP SDK**: `github.com/mark3labs/mcp-go` v0.32.0
- **Testing**: standard `testing` package + `testify/assert` + `testify/mock`
- **Build injection**: `Version`, `Commit`, `BuildDate` via ldflags
- **Imports**: stdlib → external → internal, blank line separated; alias `apimodels` for raw SDK models
- **Naming**: files `snake_case`, exported `PascalCase`, private `camelCase`
- **Commit messages**: conventional commits — `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`

---

## Adding New Tools

1. **Define in `tools.yaml`** — add YAML entry with name, description, parameters (array format), annotations
2. **Add constant** in `internal/mcp/schema.go` — e.g., `ToolMyAction = "myAction"`
3. **Add client method** in `pkg/portainer/client/adapter_<domain>.go` + update interface in `internal/mcp/client_interfaces.go`
4. **Add model** in `pkg/portainer/models/` if needed (with `ConvertXxx()`)
5. **Add handler** in `internal/mcp/<domain>.go` — implement `HandleMyAction()`
6. **Register** in `Add<Domain>Features()` via `s.addToolIfExists(ToolMyAction, s.HandleMyAction())`
7. **Add to meta-tool** in `metatool_registry.go` — append to appropriate category's `actions` slice
8. **Update domain interface** in `client_interfaces.go` if new client method added
9. **Write tests** in `internal/mcp/<domain>_test.go` — table-driven with mock client
10. **Update docs** in `docs/src/content/docs/`

---

## Testing Strategy

- **Unit tests** (`make test`): mock-based, no external dependencies. Mock client in `mocks_test.go` uses `testify/mock`. Table-driven tests with `t.Run()`.
- **Integration tests** (`make test-integration`): require Docker + Portainer container. Compare MCP handler output against direct API calls.
- **Live tests** (`tests/live/`): run against real Portainer instance for smoke testing.
- **Coverage**: `make test-coverage` generates `coverage.out`.

Minimum test cases per handler: success path, API error, missing required param, invalid ID (≤ 0).

---

## Skills

Deeper workflow guides are in `skills/`:

| Skill | When to use |
|-------|-------------|
| `mcp-tool-development` | Adding or modifying MCP tools |
| `portainer-api-expert` | Working with the Portainer REST API and Go SDK |
| `testing-patterns` | Writing unit, integration, or model conversion tests |
| `debugging-troubleshooting` | Diagnosing test failures, build errors, handler bugs |
| `release-workflow` | Cutting a release, bumping version, publishing artifacts |
| `docs-authoring` | Adding or updating the Starlight/Astro documentation site |
| `architecture-decisions` | Making or reviewing architectural changes |

---

## Documentation

Starlight/Astro site in `docs/`, built with `pnpm` (not npm):

```bash
cd docs
pnpm install
pnpm run dev    # localhost:4321
pnpm run build  # production build
```

Design decision records live in `docs/design/` with format `YYMMDD-N-short-description.md`.

## Release

GoReleaser config in `.goreleaser.yaml`. Multi-platform builds (linux/darwin/windows × amd64/arm64). Docker images pushed to `ghcr.io/jmrplens/portainer-mcp-enhanced`. Triggered by pushing a `v*` tag.
