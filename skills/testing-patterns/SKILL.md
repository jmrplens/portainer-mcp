# Testing Patterns

Comprehensive guide for writing tests in this project: mocking, table-driven tests, builder patterns, and integration tests.

## Mock Client

All unit tests mock the `PortainerClient` interface. The mock is in `internal/mcp/mocks_test.go`:

```go
type MockPortainerClient struct {
    mock.Mock
}

func (m *MockPortainerClient) GetUsers() ([]models.User, error) {
    args := m.Called()
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).([]models.User), args.Error(1)
}
```

**Key mock patterns:**
- Methods returning `(T, error)` include a nil check on `args.Get(0)` to avoid panics
- Methods returning only `error` use `args.Error(0)` directly
- The nil check is critical — without it, returning `nil, someError` causes type assertion panics

## Table-Driven Test Pattern

Every handler test follows this structure:

```go
func TestHandleMyTool(t *testing.T) {
    tests := []struct {
        name        string
        params      map[string]interface{}   // tool parameters
        mockSetup   func(*MockPortainerClient) // mock configuration
        expectError bool                      // expect result.IsError
        validate    func(*testing.T, *mcp.CallToolResult) // custom assertions
    }{
        {
            name: "successful operation",
            params: map[string]interface{}{"id": float64(1)},
            mockSetup: func(m *MockPortainerClient) {
                m.On("GetThing", 1).Return(models.Thing{ID: 1, Name: "test"}, nil)
            },
            expectError: false,
            validate: func(t *testing.T, result *mcp.CallToolResult) {
                var thing models.Thing
                err := json.Unmarshal([]byte(result.Content[0].(mcp.TextContent).Text), &thing)
                assert.NoError(t, err)
                assert.Equal(t, 1, thing.ID)
            },
        },
        {
            name: "api error",
            params: map[string]interface{}{"id": float64(1)},
            mockSetup: func(m *MockPortainerClient) {
                m.On("GetThing", 1).Return(models.Thing{}, fmt.Errorf("connection refused"))
            },
            expectError: true,
        },
        {
            name:        "missing required parameter",
            params:      map[string]interface{}{},
            mockSetup:   func(m *MockPortainerClient) {},
            expectError: true,
        },
        {
            name:        "invalid id (zero)",
            params:      map[string]interface{}{"id": float64(0)},
            mockSetup:   func(m *MockPortainerClient) {},
            expectError: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockClient := &MockPortainerClient{}
            tt.mockSetup(mockClient)

            srv := &PortainerMCPServer{cli: mockClient}
            handler := srv.HandleMyTool()

            request := mcp.CallToolRequest{}
            request.Params.Arguments = tt.params

            result, err := handler(context.Background(), request)

            assert.NoError(t, err) // handler never returns Go errors
            if tt.expectError {
                assert.True(t, result.IsError)
            } else {
                assert.False(t, result.IsError)
                if tt.validate != nil {
                    tt.validate(t, result)
                }
            }

            mockClient.AssertExpectations(t)
        })
    }
}
```

## Minimum Test Cases

Every handler test MUST include:
1. **Success path** — valid params, mock returns data, verify JSON output
2. **API error** — mock returns error, verify `result.IsError == true` and error message
3. **Missing required param** — omit required field, verify error
4. **Invalid ID** — for ID params, test `0` and negative values

Write operations should also test:
5. **Success message** — verify the text matches expected pattern (e.g., "created successfully")
6. **Invalid enum value** — for params with restricted values (roles, types)

## Result Assertions

```go
// Check error result
assert.True(t, result.IsError)
text := result.Content[0].(mcp.TextContent).Text
assert.Contains(t, text, "expected error substring")

// Check JSON success result
assert.False(t, result.IsError)
text := result.Content[0].(mcp.TextContent).Text
var data models.MyType
err := json.Unmarshal([]byte(text), &data)
assert.NoError(t, err)
assert.Equal(t, expected, data)

// Check text success result (write operations)
text := result.Content[0].(mcp.TextContent).Text
assert.Contains(t, text, "successfully")
```

## Integration Tests

Located in `tests/integration/`. Use Docker containers with real Portainer instances:

```go
func TestIntegration_Users(t *testing.T) {
    env := helpers.NewTestEnv(t)
    defer env.Cleanup()

    // Test MCP handler response matches direct API call
    mcpResult := callMCPHandler(env.MCPServer, "listUsers", nil)
    apiResult := env.RawClient.ListUsers()

    assert.Equal(t, len(apiResult), len(mcpResult))
}
```

## Model Conversion Tests

Located in `pkg/portainer/models/*_test.go`. Test that `ConvertXxx()` functions correctly map fields:

```go
func TestConvertUser(t *testing.T) {
    id := int64(1)
    role := int64(1)
    raw := &apimodels.PortainereeUser{
        ID:       &id,
        Username: "admin",
        Role:     &role,
    }
    result := ConvertUser(raw)
    assert.Equal(t, 1, result.ID)
    assert.Equal(t, "admin", result.Username)
    assert.Equal(t, "admin", result.Role)
}
```

## Running Tests

```bash
go test ./...                          # all unit tests
go test -v ./internal/mcp/ -run TestHandleGetUsers  # single test
go test -race ./...                    # race condition detection
make test-coverage                     # with coverage report
make test-integration                  # Docker-based integration tests
```
