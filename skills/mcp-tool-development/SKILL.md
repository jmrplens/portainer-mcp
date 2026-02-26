# MCP Tool Development

Step-by-step guide for adding or modifying MCP tools in this project.

## Anatomy of a Tool

Each tool consists of:
1. **YAML definition** in `tools.yaml` — name, description, parameters, annotations
2. **Name constant** in `internal/mcp/schema.go` — Go constant referencing the YAML name
3. **Handler method** in `internal/mcp/<domain>.go` — implements the tool logic
4. **Registration** in `Add<Domain>Features()` — connects constant to handler
5. **Meta-tool entry** in `metatool_registry.go` — groups into a category
6. **Client method** in `pkg/portainer/client/<domain>.go` — API call logic
7. **Interface update** in `internal/mcp/server.go` — `PortainerClient` interface

## Step-by-Step: Adding a New Tool

### Step 1: Define in tools.yaml
```yaml
myNewTool:
  description: "Do something useful"
  parameters:
    id:
      type: integer
      description: "Resource ID"
      required: true
    name:
      type: string
      description: "Resource name"
  annotations:
    title: "My New Tool"
    readOnlyHint: true
    destructiveHint: false
    idempotentHint: true
    openWorldHint: false
```

### Step 2: Add constant in schema.go
```go
ToolMyNewTool = "myNewTool"
```

### Step 3: Add client method (if calling Portainer API)
```go
// In pkg/portainer/client/<domain>.go
func (c *PortainerClient) MyNewAction(id int) (models.Result, error) {
    raw, err := c.cli.SomeAPICall(int64(id))
    if err != nil {
        return models.Result{}, fmt.Errorf("failed to do action: %w", err)
    }
    return models.ConvertResult(raw), nil
}
```

### Step 4: Update PortainerClient interface in server.go
```go
type PortainerClient interface {
    // ... existing methods ...
    MyNewAction(id int) (models.Result, error)
}
```

### Step 5: Implement handler
```go
// In internal/mcp/<domain>.go
func (s *PortainerMCPServer) HandleMyNewTool() server.ToolHandlerFunc {
    return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        parser := toolgen.NewParameterParser(request)

        id, err := parser.GetInt("id", true)
        if err != nil {
            return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
        }
        if err := validatePositiveID("id", id); err != nil {
            return mcp.NewToolResultError(err.Error()), nil
        }

        result, err := s.cli.MyNewAction(id)
        if err != nil {
            return mcp.NewToolResultErrorFromErr("failed to perform action", err), nil
        }

        return jsonResult(result, "failed to marshal result")
    }
}
```

### Step 6: Register in AddFeatures
```go
func (s *PortainerMCPServer) Add<Domain>Features() {
    s.addToolIfExists(ToolMyNewTool, s.HandleMyNewTool())
    // For write operations, wrap in: if !s.readOnly { ... }
}
```

### Step 7: Add to meta-tool registry
```go
// In metatool_registry.go, in the appropriate group's actions slice:
{name: "my_new_tool", handler: (*PortainerMCPServer).HandleMyNewTool, readOnly: true},
```

### Step 8: Write tests (see testing-patterns skill)

## Key Patterns

### Parameter Parsing
```go
parser := toolgen.NewParameterParser(request)
strVal, err := parser.GetString("name", true)     // required string
intVal, err := parser.GetInt("id", true)           // required int
boolVal, err := parser.GetBool("force", false)     // optional bool
```

### Response Helpers
```go
// JSON serialization for structured data
return jsonResult(data, "failed to marshal X")

// Plain text response
return mcp.NewToolResultText("Operation completed"), nil

// Error response (handler-level, not Go error)
return mcp.NewToolResultError("something went wrong"), nil
return mcp.NewToolResultErrorFromErr("context", err), nil
```

### Validation Helpers
```go
validatePositiveID("id", id)       // checks id > 0
isValidUserRole(role)              // validates enum values
validateComposeYAML(content)       // validates compose file syntax
validateCronExpression(expr)       // validates cron syntax
```

### Read-Only Mode
Write handlers are only registered when `!s.readOnly`. In meta-tools, mark `readOnly: false` for write actions — they are automatically filtered.

## Checklist for New Tools

- [ ] YAML definition in `tools.yaml`
- [ ] Constant in `schema.go`
- [ ] Client method + interface update (if new API call)
- [ ] Model + conversion (if new data type)
- [ ] Handler implementation with proper error handling
- [ ] Registration in `Add<Domain>Features()`
- [ ] Meta-tool entry in `metatool_registry.go`
- [ ] Unit tests (success, API error, invalid params)
- [ ] Documentation update in `docs/`
