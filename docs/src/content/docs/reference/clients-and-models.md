---
title: Clients & Models
description: Understanding the dual-client architecture and model layers.
---

import { Aside } from '@astrojs/starlight/components';

## Client Architecture

The project uses a two-layer client architecture:

```
┌──────────────────────┐
│   MCP Handlers        │  Uses local models
│   (internal/mcp/)     │
└──────────┬───────────┘
           │
┌──────────▼───────────┐
│   Wrapper Client      │  Translates between layers
│   (pkg/portainer/     │
│    client/)           │
└──────────┬───────────┘
           │
┌──────────▼───────────┐
│   Raw SDK Client      │  Direct Portainer API
│   (client-api-go/v2)  │
└──────────────────────┘
```

### Raw Client (`portainer/client-api-go/v2`)

- Auto-generated from Portainer's OpenAPI/Swagger specification
- Communicates directly with the Portainer REST API
- Works with raw models from `github.com/portainer/client-api-go/v2/pkg/models`
- Used in **integration tests** for ground-truth comparisons

### Wrapper Client (`pkg/portainer/client/`)

- Abstraction layer designed specifically for MCP
- Simplifies the raw client's interface (fewer parameters, cleaner return types)
- Handles data transformation between raw and local models
- Configures HTTP transport (TLS, timeouts, scheme)
- Used by **MCP server handlers**

## Model Layers

### Raw Models (`client-api-go/v2/pkg/models`)

Direct mapping to Portainer API data structures. These may contain:
- Fields not relevant to MCP
- Complex nested structures
- Pointer types for optional fields

**Convention**: Prefix variables with `raw`:

```go
rawSettings, err := s.client.GetSettings()
rawEndpoint, err := s.client.GetEnvironment(id)
```

### Local Models (`pkg/portainer/models/`)

Simplified structures designed for MCP responses:
- Only include fields relevant to AI assistants
- Use convenient Go types (strings, ints — not pointers where possible)
- Include full godoc documentation
- Define `FromXxx()` conversion functions

```go
// Local model — clean, simple, documented
type Environment struct {
    ID     int    `json:"Id"`
    Name   string `json:"Name"`
    URL    string `json:"URL"`
    Type   int    `json:"Type"`
    Status int    `json:"Status"`
    // ...
}

// Conversion from raw model
func EnvironmentFromAPI(raw *apimodels.PortainerEndpoint) *Environment {
    if raw == nil {
        return nil
    }
    return &Environment{
        ID:     int(raw.ID),
        Name:   raw.Name,
        URL:    raw.URL,
        // ...
    }
}
```

## Import Conventions

```go
import (
    // Local models — default import name
    "github.com/portainer/portainer-mcp/pkg/portainer/models"

    // Raw API models — aliased to distinguish
    apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)
```

This convention is used consistently across the codebase to prevent confusion between the two model layers.

## Testing with Both Layers

### Unit Tests

Mock the `PortainerClient` interface to test handler logic without a running Portainer instance:

```go
type mockClient struct {
    // Implement interface methods as needed
}

func (m *mockClient) GetEnvironment(id int) (*models.Environment, error) {
    return &models.Environment{ID: id, Name: "test"}, nil
}
```

### Integration Tests

Use both the raw client and MCP handler to verify accuracy:

```go
// Ground truth: call Portainer API directly
rawResult, err := rawClient.GetEndpoint(id)

// MCP handler result
mcpResult, err := mcpHandler(request)

// Compare
assert.Equal(t, rawResult.Name, mcpResult.Name)
```

<Aside type="note">
Integration tests require a running Portainer instance. Use `make test-integration` which spins up Docker containers automatically.
</Aside>
