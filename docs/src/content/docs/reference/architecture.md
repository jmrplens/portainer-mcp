---
title: Architecture
description: Internal architecture and design of the Portainer MCP server.
---

import { Aside } from '@astrojs/starlight/components';

## System Overview

```
┌─────────────────────┐      MCP Protocol       ┌─────────────────────┐      HTTPS       ┌───────────────┐
│   AI Assistant       │ ◄──── (stdio/JSON-RPC) ──►│  Portainer MCP      │ ◄──────────────► │  Portainer    │
│  Claude / Copilot    │                          │  Server             │                  │  API          │
│  Cursor / etc.       │                          │  (Go binary)        │                  │  v2.31.2      │
└─────────────────────┘                          └─────────────────────┘                  └───────────────┘
```

The server acts as a bridge between MCP-compatible AI assistants and the Portainer management platform. It translates MCP tool calls into Portainer API requests and returns structured results.

## Package Structure

```
portainer-mcp/
├── cmd/portainer-mcp/     # Entry point, CLI flag parsing
│   └── mcp.go             # main(), flag definitions, server bootstrap
├── internal/
│   ├── mcp/               # MCP server implementation
│   │   ├── server.go      # Server struct, PortainerClient interface, options
│   │   ├── metatool_registry.go  # 15 meta-tool definitions
│   │   ├── metatool_handler.go   # Meta-tool routing logic
│   │   ├── schema.go      # Tool constants, HTTP validation
│   │   └── *.go           # Domain handlers (docker, kubernetes, helm, etc.)
│   └── k8sutil/           # Kubernetes response metadata stripping
├── pkg/
│   ├── portainer/
│   │   ├── client/        # Wrapper client over raw SDK
│   │   │   └── adapter.go # Adapter with functional options
│   │   └── models/        # Local model definitions + converters
│   └── toolgen/           # YAML tool definition loader + parameter extraction
├── tools.yaml             # Embedded tool definitions (98 tools)
├── tests/integration/     # Integration test suite
└── docs/                  # Documentation site (Starlight)
```

## Key Components

### Entry Point (`cmd/portainer-mcp/mcp.go`)

Parses CLI flags, creates the Portainer client, and starts the MCP server. Handles:
- Flag validation (required `-server` and `-token`)
- Client construction with functional options
- Server creation with `WithClient()`, `WithReadOnly()`, `WithGranularTools()`
- Portainer version compatibility check
- Tool registration (meta or granular)
- `ServeStdio()` with graceful signal handling

### MCP Server (`internal/mcp/server.go`)

The core `Server` struct holds:
- `PortainerClient` interface — abstraction over the Portainer API
- Configuration flags (read-only, granular tools)
- All tool handler registrations

The `PortainerClient` interface defines ~170 methods covering the entire Portainer API surface. It enables easy testing through mocking.

### Meta-Tool System

**Registry** (`metatool_registry.go`): Defines 15 `MetaToolDef` structures, each containing:
- Tool name and description
- List of `MetaAction` entries (action name → handler function → read-only flag)
- Parameter definitions for each action

**Handler** (`metatool_handler.go`): Provides `RegisterMetaTools()` which:
1. Iterates over all meta-tool definitions
2. Filters actions based on read-only mode
3. Builds the `action` enum from available actions
4. Merges parameters from all actions (deduplicating shared params like `id`)
5. Creates a routing handler that dispatches by action name

### Tool Definitions (`tools.yaml`)

A YAML file embedded in the binary at build time. Each tool definition includes:
- Name, description, parameter schemas
- MCP annotations (readOnlyHint, destructiveHint, idempotentHint, openWorldHint)
- Required/optional parameter flags

The `pkg/toolgen` package loads and validates these definitions, converting them into MCP-compatible tool schemas.

### Wrapper Client (`pkg/portainer/client/`)

An abstraction layer over the raw Portainer SDK client (`portainer/client-api-go/v2`). It:
- Simplifies the SDK's interface for MCP use
- Handles data transformation between raw API models and local models
- Configures HTTP transport (TLS, timeouts, scheme detection)

Uses the **functional options pattern** for configuration:

```go
client.NewAdapter(
    client.WithHost("portainer.example.com:9443"),
    client.WithToken("ptr_abc123"),
    client.WithSkipTLSVerify(false),
)
```

### Local Models (`pkg/portainer/models/`)

Simplified data structures tailored for MCP responses. Each model:
- Contains only relevant fields (not the full API response)
- Uses convenient Go types
- Includes a `FromXxx()` conversion function from the raw API model
- Is fully documented with godoc comments

## Request Flow

1. **AI assistant** sends an MCP `tools/call` request via stdin
2. **MCP framework** routes to the registered handler
3. **Handler** extracts and validates parameters
4. **Wrapper client** translates to a Portainer API call
5. **Portainer API** processes the request and returns data
6. **Local model** conversion transforms the response
7. **Handler** serializes the model to JSON
8. **MCP framework** returns the response via stdout

## Error Handling

Errors follow a consistent pattern:

```go
result, err := s.client.GetEnvironment(id)
if err != nil {
    return nil, fmt.Errorf("failed to get environment: %w", err)
}
```

- All errors include descriptive context
- Errors are wrapped with `%w` for chain inspection
- Parameter validation happens before API calls
- Invalid parameters return clear error messages

## Graceful Shutdown

The server handles `SIGINT` and `SIGTERM` signals:

1. Signal received → context cancelled
2. Server stops accepting new requests
3. In-flight requests complete
4. Server exits cleanly

## Testing Strategy

| Level | Location | Purpose |
|:------|:---------|:--------|
| Unit tests | `internal/mcp/*_test.go` | Handler logic, parameter validation, meta-tool routing |
| Model tests | `pkg/portainer/models/*_test.go` | Model conversion, nil safety, edge cases |
| Integration tests | `tests/integration/` | End-to-end with real Portainer (Docker) |

Integration tests use Docker containers to spin up a Portainer instance, then compare MCP handler results against direct API calls for accuracy.
