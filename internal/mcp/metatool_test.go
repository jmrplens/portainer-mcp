package mcp

import (
	"context"
	"encoding/json"
	"sort"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
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

	actionMap, ok := actionProp.(map[string]any)
	require.True(t, ok, "action property should be a map")

	enumRaw, ok := actionMap["enum"]
	require.True(t, ok, "action should have enum")

	enumSlice, ok := enumRaw.([]any)
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
		args           map[string]any
		expectedAction string
		expectError    bool
		errorContains  string
	}{
		{
			name:           "routes to action_one",
			args:           map[string]any{"action": "action_one"},
			expectedAction: "action_one",
		},
		{
			name:           "routes to action_two",
			args:           map[string]any{"action": "action_two"},
			expectedAction: "action_two",
		},
		{
			name:          "missing action parameter",
			args:          map[string]any{},
			expectError:   true,
			errorContains: "missing required parameter: action",
		},
		{
			name:          "empty action",
			args:          map[string]any{"action": ""},
			expectError:   true,
			errorContains: "non-empty string",
		},
		{
			name:          "unknown action",
			args:          map[string]any{"action": "nonexistent"},
			expectError:   true,
			errorContains: "unknown action 'nonexistent'",
		},
		{
			name:          "non-string action",
			args:          map[string]any{"action": 42},
			expectError:   true,
			errorContains: "non-empty string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calledAction = ""

			req := mcp.CallToolRequest{}
			reqBytes, _ := json.Marshal(map[string]any{
				"params": map[string]any{
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
	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      2,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "manage_users",
			"arguments": map[string]any{
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

// TestRegisterOneMetaToolSkipsAllWriteInReadOnly verifies that a meta-tool
// group with only write actions is not registered in read-only mode.
func TestRegisterOneMetaToolSkipsAllWriteInReadOnly(t *testing.T) {
	s := newTestMetaServer(true)

	s.registerOneMetaTool(metaToolDef{
		name:        "test_all_write",
		description: "All write actions",
		actions: []metaAction{
			{name: "write_one", handler: (*PortainerMCPServer).HandleGetUsers, readOnly: false},
			{name: "write_two", handler: (*PortainerMCPServer).HandleGetUsers, readOnly: false},
		},
		annotation: mcp.ToolAnnotation{},
	})

	tools := listRegisteredTools(t, s.srv)
	assert.Empty(t, tools, "meta-tool with only write actions should not be registered in read-only mode")
}

// TestWriteActionRejectedInReadOnlyMode verifies that calling a write action
// through the protocol in read-only mode returns an error because the action
// is not in the enum.
func TestWriteActionRejectedInReadOnlyMode(t *testing.T) {
	s := newTestMetaServer(true)
	s.RegisterMetaTools()

	// Try to call "create_user" on manage_users — this action should be
	// filtered out in read-only mode.
	callReq := map[string]any{
		"jsonrpc": "2.0",
		"id":      3,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "manage_users",
			"arguments": map[string]any{
				"action": "create_user",
			},
		},
	}

	reqBytes, err := json.Marshal(callReq)
	require.NoError(t, err)

	resp := s.srv.HandleMessage(context.Background(), json.RawMessage(reqBytes))
	respBytes, err := json.Marshal(resp)
	require.NoError(t, err)

	var rpcResp struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(respBytes, &rpcResp))

	assert.True(t, rpcResp.Result.IsError, "write action should be rejected in read-only mode")
	assert.Contains(t, rpcResp.Result.Content[0].Text, "unknown action")
}

// TestMakeMetaHandlerForwardsRequest verifies that makeMetaHandler passes
// the full request (including all arguments) to the sub-handler.
func TestMakeMetaHandlerForwardsRequest(t *testing.T) {
	var receivedArgs map[string]any
	handler := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		receivedArgs = req.GetArguments()
		return mcp.NewToolResultText("ok"), nil
	}

	metaHandler := makeMetaHandler("test_tool", map[string]server.ToolHandlerFunc{
		"do_thing": handler,
	})

	req := mcp.CallToolRequest{}
	reqBytes, _ := json.Marshal(map[string]any{
		"params": map[string]any{
			"name": "test_tool",
			"arguments": map[string]any{
				"action":   "do_thing",
				"extra_id": 42,
				"name":     "test-value",
			},
		},
	})
	_ = json.Unmarshal(reqBytes, &req)

	result, err := metaHandler(context.Background(), req)
	assert.NoError(t, err)
	assert.False(t, result.IsError)

	// Verify the sub-handler received all original arguments
	assert.Equal(t, "do_thing", receivedArgs["action"])
	assert.Equal(t, float64(42), receivedArgs["extra_id"])
	assert.Equal(t, "test-value", receivedArgs["name"])
}

// TestMetaToolRegistryActionNames verifies each meta-tool group contains
// exactly the expected action names. This acts as a structural snapshot
// to catch accidental additions or removals.
func TestMetaToolRegistryActionNames(t *testing.T) {
	expected := map[string][]string{
		"manage_environments": {
			"list_environments", "get_environment", "delete_environment",
			"snapshot_environment", "snapshot_all_environments",
			"update_environment_tags", "update_environment_user_accesses",
			"update_environment_team_accesses",
			"list_environment_groups", "create_environment_group",
			"update_environment_group_name", "update_environment_group_environments",
			"update_environment_group_tags",
			"list_environment_tags", "create_environment_tag", "delete_environment_tag",
		},
		"manage_stacks": {
			"list_stacks", "list_regular_stacks", "get_stack", "get_stack_file",
			"inspect_stack_file", "create_stack", "update_stack", "delete_stack",
			"update_stack_git", "redeploy_stack_git", "start_stack", "stop_stack",
			"migrate_stack",
		},
		"manage_access_groups": {
			"list_access_groups", "create_access_group", "update_access_group_name",
			"update_access_group_user_accesses", "update_access_group_team_accesses",
			"add_environment_to_access_group", "remove_environment_from_access_group",
		},
		"manage_users": {
			"list_users", "get_user", "create_user", "delete_user", "update_user_role",
		},
		"manage_teams": {
			"list_teams", "get_team", "create_team", "delete_team",
			"update_team_name", "update_team_members",
		},
		"manage_docker": {
			"get_docker_dashboard", "docker_proxy",
		},
		"manage_kubernetes": {
			"get_kubernetes_resource_stripped", "get_kubernetes_dashboard",
			"list_kubernetes_namespaces", "get_kubernetes_config", "kubernetes_proxy",
		},
		"manage_helm": {
			"list_helm_repositories", "search_helm_charts",
			"list_helm_releases", "get_helm_release_history",
			"add_helm_repository", "remove_helm_repository",
			"install_helm_chart", "delete_helm_release",
		},
		"manage_registries": {
			"list_registries", "get_registry", "create_registry",
			"update_registry", "delete_registry",
		},
		"manage_templates": {
			"list_custom_templates", "get_custom_template", "get_custom_template_file",
			"create_custom_template", "delete_custom_template",
			"list_app_templates", "get_app_template_file",
		},
		"manage_backups": {
			"get_backup_status", "get_backup_s3_settings",
			"create_backup", "backup_to_s3", "restore_from_s3",
		},
		"manage_webhooks": {
			"list_webhooks", "create_webhook", "delete_webhook",
		},
		"manage_edge": {
			"list_edge_jobs", "get_edge_job", "get_edge_job_file",
			"create_edge_job", "delete_edge_job", "list_edge_update_schedules",
		},
		"manage_settings": {
			"get_settings", "get_public_settings", "update_settings",
			"get_ssl_settings", "update_ssl_settings",
		},
		"manage_system": {
			"get_system_status", "list_roles", "get_motd",
			"authenticate", "logout",
		},
	}

	defs := metaToolDefinitions()
	for _, def := range defs {
		t.Run(def.name, func(t *testing.T) {
			expectedActions, ok := expected[def.name]
			require.True(t, ok, "unexpected meta-tool group %q", def.name)

			actual := make([]string, len(def.actions))
			for i, a := range def.actions {
				actual[i] = a.name
			}

			sort.Strings(expectedActions)
			sort.Strings(actual)
			assert.Equal(t, expectedActions, actual)
		})
	}

	// Verify no expected groups are missing from definitions
	defNames := make(map[string]bool, len(defs))
	for _, def := range defs {
		defNames[def.name] = true
	}
	for name := range expected {
		assert.True(t, defNames[name], "expected meta-tool group %q not found in definitions", name)
	}
}

// TestDefaultModeAnnotations verifies that annotation hints are correct
// in default (non-read-only) mode. Most meta-tools should have
// ReadOnlyHint=false and DestructiveHint=true because they contain
// write/delete actions.
func TestDefaultModeAnnotations(t *testing.T) {
	s := newTestMetaServer(false)
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

	// In default mode, no meta-tool should be marked read-only because
	// every group has at least one write action.
	for _, tool := range rpcResp.Result.Tools {
		if tool.Annotations.ReadOnlyHint != nil {
			assert.False(t, *tool.Annotations.ReadOnlyHint,
				"tool %s should not have ReadOnlyHint=true in default mode", tool.Name)
		}
	}

	// manage_settings and manage_system should NOT be marked destructive
	nonDestructive := map[string]bool{
		"manage_settings": true,
		"manage_system":   true,
	}
	for _, tool := range rpcResp.Result.Tools {
		if nonDestructive[tool.Name] && tool.Annotations.DestructiveHint != nil {
			assert.False(t, *tool.Annotations.DestructiveHint,
				"tool %s should not be destructive", tool.Name)
		}
	}
}

// TestMetaToolDescriptionsListActions verifies that each meta-tool's
// description mentions all of its action names so LLM callers can
// discover available actions from the description.
func TestMetaToolDescriptionsListActions(t *testing.T) {
	defs := metaToolDefinitions()
	for _, def := range defs {
		t.Run(def.name, func(t *testing.T) {
			for _, a := range def.actions {
				assert.Contains(t, def.description, a.name,
					"description for %q should mention action %q", def.name, a.name)
			}
		})
	}
}
