package dynamic

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/tools/system"
)

func connect(t *testing.T, server *mcp.Server) (*mcp.ClientSession, context.Context) {
	t.Helper()
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, ctx
}

func serverFor(t *testing.T, deps tools.Deps) *mcp.Server {
	t.Helper()
	catalog, err := actioncatalog.Build(system.Specs(), actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "portainer-mcp", Version: "test"}, nil)
	if err := (Surface{}).Register(server, catalog, deps); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return server
}

// The defining property of this surface: two tools, whatever the catalog size.
func TestRegister_ExposesExactlyTwoTools(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 2 {
		t.Fatalf("tools = %d, want exactly 2", len(res.Tools))
	}
	names := res.Tools[0].Name + " " + res.Tools[1].Name
	for _, want := range []string{"portainer_find_action", "portainer_execute_action"} {
		if !strings.Contains(names, want) {
			t.Errorf("tools = %q, want %q present", names, want)
		}
	}
}

func TestFindAction_MatchesOnName(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_find_action",
		Arguments: map[string]any{"query": "system version"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "system.version") {
		t.Errorf("find(system version) = %q, want system.version among the matches", text)
	}
}

func TestFindAction_MatchesOnDescription(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_find_action",
		Arguments: map[string]any{"query": "connectivity check"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "system.status") {
		t.Error("a query matching only a description found nothing")
	}
}

func TestFindAction_NoMatch_SaysSoAndSuggests(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_find_action",
		Arguments: map[string]any{"query": "zzzz-no-such-thing"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(strings.ToLower(text), "no action") {
		t.Errorf("result = %q, want it to say plainly that nothing matched", text)
	}
}

func TestExecuteAction_UnknownAction_IsRefused(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_execute_action",
		Arguments: map[string]any{"action": "system.nonsense"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("IsError = false, want true for an unknown action")
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "portainer_find_action") {
		t.Error("the error does not point the model at find, which is how it recovers")
	}
}

// Safe mode must intercept here too.
func TestExecuteAction_SafeMode_Intercepts(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{SafeMode: true}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_execute_action",
		Arguments: map[string]any{"action": "system.update", "input": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(strings.ToLower(res.Content[0].(*mcp.TextContent).Text), "safe mode") {
		t.Error("safe mode did not intercept through the dynamic surface")
	}
}

// The catalog built for this test includes system.update, a mutating action,
// so portainer_execute_action must not advertise itself as read-only: a
// client deciding whether to warn a user, or auto-approve, reads this hint.
func TestRegister_ExecuteAction_AnnotatedNotReadOnly_WhenCatalogHasMutatingActions(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var execTool *mcp.Tool
	for _, tool := range res.Tools {
		if tool.Name == "portainer_execute_action" {
			execTool = tool
		}
	}
	if execTool == nil {
		t.Fatal("portainer_execute_action was not registered")
	}
	if execTool.Annotations == nil || execTool.Annotations.ReadOnlyHint {
		t.Error("portainer_execute_action is annotated ReadOnlyHint = true, but the catalog contains system.update, a mutating action")
	}
}

// find must report the action's danger, because on this surface it is the
// model's only chance to see it before calling execute.
func TestFindAction_ReportsMutatingAndDestructive(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_find_action",
		Arguments: map[string]any{"query": "update"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	var matches []map[string]any
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &matches); err != nil {
		t.Fatalf("find did not return JSON: %v", err)
	}
	for _, m := range matches {
		if m["action"] == "system.update" {
			if m["mutating"] != true {
				t.Error("system.update is not reported as mutating")
			}
			return
		}
	}
	t.Error("system.update was not among the matches for \"update\"")
}
