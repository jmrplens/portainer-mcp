# Debugging & Troubleshooting

**When to use**: Diagnosing test failures, build errors, handler bugs, proxy issues, version mismatch errors, or unexpected MCP tool behaviour.
**Triggers on**: debug, error, failing test, build error, handler bug, version mismatch, proxy issue, MCP inspector, zerolog, test failure
**Covers**: common error patterns, running targeted tests, reading zerolog output, MCP inspector usage, integration test debugging, version check bypass

---

## Common Error Patterns

### Tool not registered / "tool not found"

The server logs a warning at startup for any tool constant that has no matching entry in `tools.yaml`:

```
WARN tool not found, will not be registered for MCP usage  tool=myNewTool
```

**Causes:**
- Tool constant in `schema.go` doesn't match the key in `tools.yaml` (case-sensitive).
- `tools.yaml` was not re-embedded — run `make build` to pick up YAML changes (the file is embedded at compile time via `internal/tooldef/`).
- Handler registered with wrong constant name.

**Fix:** Verify the string in `schema.go` exactly matches the YAML key. Rebuild.

---

### Version mismatch on startup

```
failed to get Portainer server version: ...
unsupported Portainer server version: 2.40.0, only version 2.39.x is supported
```

**Causes:** Connected Portainer instance is a different major.minor than `SupportedPortainerVersion` in `server.go`.

**Fix for development:** Pass `--disable-version-check` flag. Do not change `SupportedPortainerVersion` unless intentionally upgrading support.

---

### Handler returns `result.IsError == true` unexpectedly

Check in order:
1. **Parameter parsing** — is the parameter name in the test map exactly matching the YAML definition?
2. **Type mismatch** — JSON numbers are `float64`; use `float64(1)` not `int(1)` in test params.
3. **Validation** — `validatePositiveID` rejects `0` and negative values.
4. **Mock not set up** — if `mockClient.On(...)` is missing, testify panics or returns zero values.

---

### Mock panic: "interface conversion: interface {} is nil"

This happens when a mock method returns `nil` for a value type without the nil guard:

```go
// WRONG — panics when returning (nil, err)
func (m *MockPortainerClient) GetUsers() ([]models.User, error) {
    args := m.Called()
    return args.Get(0).([]models.User), args.Error(1)  // panics if Get(0) is nil
}
```

```go
// CORRECT — nil guard prevents panic
func (m *MockPortainerClient) GetUsers() ([]models.User, error) {
    args := m.Called()
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).([]models.User), args.Error(1)
}
```

All mock methods in `mocks_test.go` that return `(T, error)` must include this nil guard.

---

### `go vet` or `gofmt` failures in CI

```bash
# Check formatting diff (no output = clean)
gofmt -d .

# Apply formatting
gofmt -s -w .

# Run vet
go vet ./...
```

Both must be clean before committing. CI runs these as part of the build job.

---

### Integration test: Portainer container fails to start

```bash
# Run integration tests with verbose output
go test -v ./tests/... -timeout 120s

# Check Docker is running
docker ps

# Pull the Portainer image manually if slow
docker pull portainer/portainer-ee:2.39.1
```

Integration tests use `testcontainers-go` to spin up a real Portainer instance. They require Docker and take ~30–60s. Skip them with `-short` flag or `make test` (unit only).

---

## Running Targeted Tests

```bash
# Single handler test
go test -v ./internal/mcp/ -run TestHandleGetUsers

# All tests in a domain
go test -v ./internal/mcp/ -run "TestHandle.*Stack"

# All model conversion tests
go test -v ./pkg/portainer/models/

# All client adapter tests
go test -v ./pkg/portainer/client/

# With race detector
go test -race ./...

# With coverage
make test-coverage
go tool cover -html=coverage.out   # open in browser
```

---

## Reading zerolog Output

The server logs to **stderr** (never stdout — stdout is the MCP transport). Log output uses structured JSON fields:

```bash
# Run with debug logging
dist/portainer-mcp-enhanced --server ... --token ... 2>&1 | jq .

# Filter for warnings only
dist/portainer-mcp-enhanced ... 2>&1 | jq 'select(.level == "warn")'
```

Key log fields:
- `level` — `debug`, `info`, `warn`, `error`
- `tool` — tool name (appears in registration warnings)
- `error` — error message string
- `caller` — source file and line

---

## MCP Inspector

The MCP Inspector is an interactive browser-based tool for testing tool calls without a full AI client. It runs via `npx` and launches the binary as a subprocess:

```bash
make inspector
# Equivalent to: npx @modelcontextprotocol/inspector dist/portainer-mcp-enhanced
# The inspector UI opens in your browser automatically
```

The binary receives `--server` and `--token` via the inspector's connection UI — you do not pass them on the command line when using `make inspector`. Use it to:
- Verify tool registration (all 15 meta-tools or 98 granular tools appear)
- Test individual tool calls with custom parameters
- Inspect raw JSON responses
- Confirm read-only mode filters write tools

---

## Proxy Issues (Docker / Kubernetes)

The Docker and Kubernetes proxy handlers pass requests through to the Portainer API. Common issues:

| Symptom | Cause | Fix |
|---------|-------|-----|
| `path traversal detected` | Path contains `..` | Remove `..` from the API path |
| `response too large` | Response > 10MB | Use more specific API paths to reduce response size |
| `invalid HTTP method` | Method not in allowlist | Use `GET`, `POST`, `PUT`, `DELETE`, `HEAD`, or `PATCH` |
| `401 Unauthorized` | Token lacks permission | Use a token with appropriate Portainer role |

---

## Build Errors

```bash
# Clean build
make clean && make build

# Verify module dependencies
go mod tidy
go mod verify

# Check for import cycles
go build ./...
```

If `internal/tooldef/` fails to embed `tools.yaml`, ensure the file exists at the project root and the `//go:embed` directive in `tooldef.go` is intact.
