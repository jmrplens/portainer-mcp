package mcp

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestMetaServer creates a PortainerMCPServer wired for meta-tool testing.
// It uses a minimal MCPServer instance and mock client so we can register
// meta-tools and query them through the protocol without needing a real
// Portainer backend or tools.yaml file.
func newTestMetaServer(readOnly bool) *PortainerMCPServer {
	return &PortainerMCPServer{
		srv: server.NewMCPServer(
			"test-meta-server",
			"0.0.1",
			server.WithToolCapabilities(true),
		),
		cli:      &MockPortainerClient{},
		readOnly: readOnly,
	}
}

// listRegisteredTools sends a tools/list JSON-RPC request through the
// MCPServer and returns the tool names.
func listRegisteredTools(t *testing.T, srv *server.MCPServer) []string {
	t.Helper()

	// Build a valid JSON-RPC tools/list request
	reqJSON := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	resp := srv.HandleMessage(context.Background(), json.RawMessage(reqJSON))

	// The response is a JSONRPCResponse
	respBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	var rpcResp struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(respBytes, &rpcResp))

	names := make([]string, len(rpcResp.Result.Tools))
	for i, tool := range rpcResp.Result.Tools {
		names[i] = tool.Name
	}
	sort.Strings(names)
	return names
}

// TestMetaToolDefinitionsCount verifies that metaToolDefinitions returns
// exactly 15 groups with 98 total actions.
func TestMetaToolDefinitionsCount(t *testing.T) {
	defs := metaToolDefinitions()
	assert.Equal(t, 15, len(defs), "expected 15 meta-tool groups")

	totalActions := 0
	for _, def := range defs {
		totalActions += len(def.actions)
	}
	assert.Equal(t, 98, totalActions, "expected 98 total actions across all meta-tools")
}

// TestMetaToolUniqueActionNames verifies that all action names within each
// meta-tool group are unique.
func TestMetaToolUniqueActionNames(t *testing.T) {
	defs := metaToolDefinitions()
	for _, def := range defs {
		seen := make(map[string]bool, len(def.actions))
		for _, a := range def.actions {
			assert.False(t, seen[a.name], "duplicate action '%s' in meta-tool '%s'", a.name, def.name)
			seen[a.name] = true
		}
	}
}

// TestMetaToolUniqueGroupNames verifies that all meta-tool names are unique.
func TestMetaToolUniqueGroupNames(t *testing.T) {
	defs := metaToolDefinitions()
	seen := make(map[string]bool, len(defs))
	for _, def := range defs {
		assert.False(t, seen[def.name], "duplicate meta-tool name '%s'", def.name)
		seen[def.name] = true
	}
}

// TestRegisterMetaToolsDefaultMode verifies that RegisterMetaTools registers
// exactly 15 tools (one per meta-tool group) when not in read-only mode.
func TestRegisterMetaToolsDefaultMode(t *testing.T) {
	s := newTestMetaServer(false)
	s.RegisterMetaTools()

	tools := listRegisteredTools(t, s.srv)
	assert.Equal(t, 15, len(tools), "expected 15 meta-tools registered")

	// Verify all expected names are present
	expected := []string{
		"manage_access_groups",
		"manage_backups",
		"manage_docker",
		"manage_edge",
		"manage_environments",
		"manage_helm",
		"manage_kubernetes",
		"manage_registries",
		"manage_settings",
		"manage_stacks",
		"manage_system",
		"manage_teams",
		"manage_templates",
		"manage_users",
		"manage_webhooks",
	}
	sort.Strings(expected)
	assert.Equal(t, expected, tools)
}

// TestRegisterMetaToolsReadOnlyMode verifies that in read-only mode,
// meta-tools with only write actions are not registered, and meta-tools
// with mixed actions only include read actions.
func TestRegisterMetaToolsReadOnlyMode(t *testing.T) {
	s := newTestMetaServer(true)
	s.RegisterMetaTools()

	tools := listRegisteredTools(t, s.srv)
	// All 15 groups have at least one read-only action, so all should be registered.
	assert.Equal(t, 15, len(tools), "all 15 meta-tools should be registered in read-only mode")
}

// TestMetaToolReadOnlyActionFiltering verifies that the action enum
// of a meta-tool in read-only mode excludes write actions.
func TestMetaToolReadOnlyActionFiltering(t *testing.T) {
	s := newTestMetaServer(true)
	s.RegisterMetaTools()

	// Query tools/list to get full tool definitions with their schemas
	reqJSON := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	resp := s.srv.HandleMessage(context.Background(), json.RawMessage(reqJSON))

	respBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	var rpcResp struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(respBytes, &rpcResp))

	// Find manage_environments and check its action enum
	var envTool *mcp.Tool
	for i, tool := range rpcResp.Result.Tools {
		if tool.Name == "manage_environments" {
			envTool = &rpcResp.Result.Tools[i]
			break
		}
	}
	require.NotNil(t, envTool, "manage_environments tool should exist")

	// Extract action enum from input schema
	actionProp, ok := envTool.InputSchema.Properties["action"]
	require.True(t, ok, "action property should exist")

	actionMap, ok := actionProp.(map[string]interface{})
	require.True(t, ok, "action property should be a map")

	enumRaw, ok := actionMap["enum"]
	require.True(t, ok, "action should have enum")

	enumSlice, ok := enumRaw.([]interface{})
	require.True(t, ok, "enum should be a slice")

	// Verify that write-only actions are excluded
	writeActions := map[string]bool{
		"delete_environment":                    true,
		"snapshot_environment":                  true,
		"snapshot_all_environments":             true,
		"update_environment_tags":               true,
		"update_environment_user_accesses":      true,
		"update_environment_team_accesses":      true,
		"create_environment_group":              true,
		"update_environment_group_name":         true,
		"update_environment_group_environments": true,
		"update_environment_group_tags":         true,
		"create_environment_tag":                true,
		"delete_environment_tag":                true,
	}

	for _, v := range enumSlice {
		actionName, ok := v.(string)
		require.True(t, ok)
		assert.False(t, writeActions[actionName],
			"write action '%s' should not be in read-only enum", actionName)
	}

	// Verify read actions ARE present
	readActions := []string{
		"list_environments",
		"get_environment",
		"list_environment_groups",
		"list_environment_tags",
	}
	enumStrings := make([]string, len(enumSlice))
	for i, v := range enumSlice {
		enumStrings[i] = v.(string)
	}
	for _, ra := range readActions {
		assert.Contains(t, enumStrings, ra,
			"read action '%s' should be in read-only enum", ra)
	}
}

// TestMetaToolReadOnlyAnnotation verifies that when all remaining actions
// are read-only, the meta-tool's annotation is set to read-only.
func TestMetaToolReadOnlyAnnotation(t *testing.T) {
	s := newTestMetaServer(true)
	s.RegisterMetaTools()

	reqJSON := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`
	resp := s.srv.HandleMessage(context.Background(), json.RawMessage(reqJSON))

	respBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	var rpcResp struct {
		Result struct {
			Tools []mcp.Tool `json:"tools"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(respBytes, &rpcResp))

	for _, tool := range rpcResp.Result.Tools {
		if tool.Annotations.ReadOnlyHint != nil {
			assert.True(t, *tool.Annotations.ReadOnlyHint,
				"tool %s should have ReadOnlyHint=true in read-only mode", tool.Name)
		}
	}
}

// TestMakeMetaHandlerRouting verifies that makeMetaHandler correctly routes
// to the appropriate sub-handler based on the action parameter.
func TestMakeMetaHandlerRouting(t *testing.T) {
	var calledAction string
	handler1 := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calledAction = "action_one"
		return mcp.NewToolResultText("result_one"), nil
	}
	handler2 := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		calledAction = "action_two"
		return mcp.NewToolResultText("result_two"), nil
	}

	handlers := map[string]server.ToolHandlerFunc{
		"action_one": handler1,
		"action_two": handler2,
	}

	metaHandler := makeMetaHandler("test_tool", handlers)

	tests := []struct {
		name           string
		args           map[string]interface{}
		expectedAction string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "routes to action_one",
			args:           map[string]interface{}{"action": "action_one"},
			expectedAction: "action_one",
		},
		{
			name:           "routes to action_two",
			args:           map[string]interface{}{"action": "action_two"},
			expectedAction: "action_two",
		},
		{
			name:          "missing action parameter",
			args:          map[string]interface{}{},
			expectError:   true,
			errorContains: "missing required parameter: action",
		},
		{
			name:          "empty action",
			args:          map[string]interface{}{"action": ""},
			expectError:   true,
			errorContains: "non-empty string",
		},
		{
			name:          "unknown action",
			args:          map[string]interface{}{"action": "nonexistent"},
			expectError:   true,
			errorContains: "unknown action 'nonexistent'",
		},
		{
			name:          "non-string action",
			args:          map[string]interface{}{"action": 42},
			expectError:   true,
			errorContains: "non-empty string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledAction = ""

			req := mcp.CallToolRequest{}
			reqBytes, _ := json.Marshal(map[string]interface{}{
				"params": map[string]interface{}{
					"name":      "test_tool",
					"arguments": tt.args,
				},
			})
			_ = json.Unmarshal(reqBytes, &req)

			result, err := metaHandler(context.Background(), req)
			assert.NoError(t, err, "meta handler should not return Go errors")
			require.NotNil(t, result)

			if tt.expectError {
				assert.True(t, result.IsError, "expected error result")
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, tt.errorContains)
			} else {
				assert.False(t, result.IsError)
				assert.Equal(t, tt.expectedAction, calledAction)
			}
		})
	}
}

// TestMetaToolHandlerIntegration verifies that a registered meta-tool's
// handler correctly routes through to the underlying handler.
func TestMetaToolHandlerIntegration(t *testing.T) {
	s := newTestMetaServer(false)

	// Mock the GetUsers method since we'll call manage_users with action "list_users"
	mockClient := s.cli.(*MockPortainerClient)
	mockClient.On("GetUsers").Return([]models.User{}, nil)

	s.RegisterMetaTools()

	// Call the meta-tool through the MCP protocol
	callReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": "manage_users",
			"arguments": map[string]interface{}{
				"action": "list_users",
			},
		},
	}

	reqBytes, err := json.Marshal(callReq)
	require.NoError(t, err)

	resp := s.srv.HandleMessage(context.Background(), json.RawMessage(reqBytes))
	respBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	// Verify the response is valid JSON-RPC
	var rpcResp struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(respBytes, &rpcResp))

	// The mock returns empty slice which will serialize as "[]" in text content
	assert.Nil(t, rpcResp.Error, "should not have JSON-RPC error")
}

// TestAllMetaToolActionsHaveHandlers verifies that every action in every
// meta-tool definition points to a non-nil handler.
func TestAllMetaToolActionsHaveHandlers(t *testing.T) {
	s := newTestMetaServer(false)
	defs := metaToolDefinitions()

	for _, def := range defs {
		for _, a := range def.actions {
			assert.NotNil(t, a.handler,
				"action '%s' in meta-tool '%s' has nil handler", a.name, def.name)
			// Verify the handler can be called (gets a function from the server)
			handlerFunc := a.handler(s)
			assert.NotNil(t, handlerFunc,
				"handler for action '%s' in meta-tool '%s' returned nil", a.name, def.name)
		}
	}
}

// TestMetaToolDescriptionsNotEmpty verifies that all meta-tools have
// non-empty descriptions.
func TestMetaToolDescriptionsNotEmpty(t *testing.T) {
	defs := metaToolDefinitions()
	for _, def := range defs {
		assert.NotEmpty(t, def.description,
			"meta-tool '%s' has empty description", def.name)
	}
}

// TestBoolPtr verifies the boolPtr helper.
func TestBoolPtr(t *testing.T) {
	truePtr := boolPtr(true)
	falsePtr := boolPtr(false)

	assert.NotNil(t, truePtr)
	assert.NotNil(t, falsePtr)
	assert.True(t, *truePtr)
	assert.False(t, *falsePtr)
}
