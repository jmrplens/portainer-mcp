---
applyTo: "**"
---
# portainer-mcp-enhanced

> Canonical source: `AGENTS.md`. This file is the Copilot-formatted version.

## Project
- Go 1.24+ MCP server bridging AI assistants to the Portainer API. Module: `github.com/jmrplens/portainer-mcp-enhanced`.
- MCP SDK: `github.com/mark3labs/mcp-go` v0.32.0. CGO_ENABLED=0 — statically linked binary.
- 98 tools grouped into 15 meta-tools by default. `--granular-tools` exposes all individually.

## Build & Test
- Build: `make build` → `dist/portainer-mcp-enhanced`. Cross-compile: `make PLATFORM=linux ARCH=amd64 build`.
- Test: `make test` (unit), `make test-integration` (requires Docker), `make test-all`, `make test-coverage`.
- Format: `gofmt -s -w .`. Lint: `go vet ./...`. Both must be clean before committing.
- Docs site: `cd docs && pnpm install && pnpm run dev` (use `pnpm`, not `npm`).

## Architecture
- `internal/mcp/` — handlers, metatool system, `client_interfaces.go`, `schema.go`, `server.go`.
- `pkg/portainer/client/` — HTTP client: `adapter.go` + `adapter_<domain>.go` (16 domain files).
- `pkg/portainer/models/` — local models + `ConvertXxx()` from raw API models (35 files).
- `tools.yaml` — declarative tool definitions, embedded at build via `internal/tooldef/`.

## Key Patterns
- **Handler pattern**: `func (s *PortainerMCPServer) HandleXxx() server.ToolHandlerFunc { return func(...) {} }`.
- **Parameter parsing**: `toolgen.NewParameterParser(request)` → `GetString`, `GetInt`, `GetBool(name, required)`.
- **Response helpers**: `jsonResult(data, "msg")` for structured data; `mcp.NewToolResultText("msg")` for plain text; `mcp.NewToolResultErrorFromErr("ctx", err)` for errors.
- **Two model layers**: Raw API models (aliased `apimodels`) → Local models (`models`) via `ConvertXxx()`. Handlers never touch raw models.
- **Interface-based client**: `PortainerClient` in `server.go` composes 18 sub-interfaces from `client_interfaces.go`. All handlers use `s.cli` — never direct HTTP.
- **Tool constants**: Tool names are string constants in `internal/mcp/schema.go`. Always add new tools there first.
- **YAML-driven tools**: Definitions in `tools.yaml` (array `parameters:` format). Keep YAML and Go handler in sync.
- **Meta-tool registry**: `metatool_registry.go` groups tools into 15 categories. New tools must be added to the appropriate group.
- **Read-only mode**: Write handlers excluded at registration time. Mark `readOnly: true/false` in metatool actions.

## Code Style
- Error wrapping: `fmt.Errorf("context: %w", err)` — always provide operation context.
- Logging: `github.com/rs/zerolog` — structured fields, never `fmt.Println`, never log to stdout (MCP transport uses stdout).
- Imports: stdlib → external → internal (blank line separated). Alias: `apimodels "github.com/portainer/client-api-go/v2/pkg/models"`.
- Naming: files `snake_case`, exported `PascalCase`, private `camelCase`.
- Commit messages: conventional commits — `feat:`, `fix:`, `docs:`, `test:`, `refactor:`, `chore:`.

## Adding New Tools (checklist)
1. YAML definition in `tools.yaml` (array `parameters:` format, correct annotations).
2. Constant in `internal/mcp/schema.go`.
3. Client method in `pkg/portainer/client/adapter_<domain>.go` + interface in `client_interfaces.go`.
4. Local model in `pkg/portainer/models/` with `ConvertXxx()` if needed.
5. Handler in `internal/mcp/<domain>.go`.
6. Registration in `Add<Domain>Features()` via `s.addToolIfExists(...)`.
7. Meta-tool entry in `metatool_registry.go`.
8. Unit tests in `internal/mcp/<domain>_test.go`.
9. Docs update in `docs/src/content/docs/`.

## Skills
Deeper workflow guides in `skills/`:
- `mcp-tool-development` — adding/modifying tools end-to-end.
- `portainer-api-expert` — Portainer REST API and Go SDK patterns.
- `testing-patterns` — unit, integration, and model conversion tests.
- `debugging-troubleshooting` — diagnosing failures, build errors, handler bugs.
- `release-workflow` — cutting releases, publishing Docker images.
- `docs-authoring` — Starlight/Astro documentation site.
- `architecture-decisions` — architectural changes and ADR format.

---
applyTo: "**/*_test.go"
---
## Test Conventions
- Table-driven tests: `tests := []struct{ name string; ... }` with `t.Run(tt.name, ...)`.
- Mock `PortainerClient` using `MockPortainerClient` from `mocks_test.go` (testify/mock).
- Mock setup: `mockClient.On("MethodName", args...).Return(result, error)`.
- Always call `mockClient.AssertExpectations(t)` at end of each test case.
- Create server: `&PortainerMCPServer{cli: mockClient}` — no real HTTP needed.
- Use `testify/assert`: `assert.NoError`, `assert.Equal`, `assert.Contains`, `assert.True`.
- Error cases: verify `result.IsError == true`; check message in `result.Content[0].(mcp.TextContent).Text`.
- Success cases: unmarshal `result.Content[0].(mcp.TextContent).Text` as JSON; compare to expected models.
- Write operations: also verify success message string (e.g., `"created successfully"`).
- Minimum cases per handler: success, API error, missing required param, invalid ID (≤ 0), invalid enum value.
- Integration tests in `tests/integration/` use real Docker containers — do not run in CI without Docker.
