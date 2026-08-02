package meta

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/tools/system"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
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

func serverFor(t *testing.T, opts actioncatalog.Options, deps tools.Deps) *mcp.Server {
	t.Helper()
	catalog, err := actioncatalog.Build(system.Specs(), opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "portainer-mcp", Version: "test"}, nil)
	if err := (Surface{}).Register(server, catalog, deps); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return server
}

// TestRegister_ExposesOneToolPerDomain uses a catalog spanning two domains
// (system.Specs() plus the tags stub below), not system.Specs() alone: that
// catalog only ever has one domain, so registering just the first domain
// would be indistinguishable from registering all of them, and this test
// would pass even if Register only ever registered its first iteration.
func TestRegister_ExposesOneToolPerDomain(t *testing.T) {
	t.Parallel()
	specs := append(system.Specs(), toolsSpecForOtherDomain())
	catalog, err := actioncatalog.Build(specs, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "portainer-mcp", Version: "test"}, nil)
	if err := (Surface{}).Register(server, catalog, tools.Deps{}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	session, ctx := connect(t, server)

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	byName := make(map[string]string, len(res.Tools))
	for _, tool := range res.Tools {
		byName[tool.Name] = tool.Description
	}
	if len(res.Tools) != 2 {
		t.Fatalf("tools = %+v, want exactly portainer_system and portainer_tags", res.Tools)
	}
	systemDescription, ok := byName["portainer_system"]
	if !ok {
		t.Fatal("portainer_system is missing")
	}
	if _, ok := byName["portainer_tags"]; !ok {
		t.Fatal("portainer_tags is missing")
	}
	// The description must enumerate the actions, because it is the only place
	// a model can discover them on this surface.
	for _, want := range []string{"system.info", "system.status", "system.update"} {
		if !strings.Contains(systemDescription, want) {
			t.Errorf("description does not mention %q", want)
		}
	}
}

// TestCallTool_KnownAction_Dispatches uses system.Specs()'s real system.info
// action with tools.Deps{} — no client configured. That used to panic: the
// real handler dereferences a nil *portainer.Client. It no longer does,
// because Execute now refuses a nil client itself before calling any handler
// (see internal/tools/register.go), returning a clean tool error instead. That
// error names the resolved action ("system.info: no Portainer client is
// configured"), which is what proves dispatch reached this action's own
// handler rather than rejecting the name as unknown or resolving some other
// action — a stronger, real-dispatch-path check than the stub this test used
// before that fix landed.
func TestCallTool_KnownAction_Dispatches(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"}, tools.Deps{}))

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_system",
		Arguments: map[string]any{"action": "system.info", "input": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if strings.Contains(text, "unknown action") {
		t.Errorf("dispatch rejected a known action: %s", text)
	}
	if !strings.Contains(text, "system.info") {
		t.Errorf("dispatch did not reach system.info: got %q", text)
	}
}

func TestCallTool_UnknownAction_ListsTheValidOnes(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"}, tools.Deps{}))

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_system",
		Arguments: map[string]any{"action": "system.nonsense"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("IsError = false, want true for an unknown action")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "system.info") {
		t.Errorf("error = %q, want it to list the valid actions so the model can recover", text)
	}
}

// TestRegister_Annotations_ReflectMostDangerousAction pins domainAnnotations'
// contract: none of the other tests here exercise ReadOnlyHint at all, so a
// domainAnnotations that always returned ReadOnlyHint: true would pass every
// other test in this file while advertising a false protection to a client
// that decides whether to confirm an action based on that hint.
func TestRegister_Annotations_ReflectMostDangerousAction(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"}, tools.Deps{}))
	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 1 {
		t.Fatalf("tools = %+v, want exactly one", res.Tools)
	}
	tool := res.Tools[0]
	if tool.Annotations == nil {
		t.Fatal("portainer_system has no annotations")
	}
	// system.update, a mutating action, is reachable through this tool, so the
	// tool as a whole must not be annotated read-only even though most of its
	// actions are.
	if tool.Annotations.ReadOnlyHint {
		t.Error("ReadOnlyHint = true, want false: a mutating action is reachable through this tool")
	}

	readOnlySession, readOnlyCtx := connect(t, serverFor(t, actioncatalog.Options{
		Edition: edition.EE, ServerVersion: "2.44.0", ReadOnly: true,
	}, tools.Deps{}))
	readOnlyRes, err := readOnlySession.ListTools(readOnlyCtx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(readOnlyRes.Tools) != 1 {
		t.Fatalf("tools = %+v, want exactly one", readOnlyRes.Tools)
	}
	readOnlyTool := readOnlyRes.Tools[0]
	if readOnlyTool.Annotations == nil {
		t.Fatal("portainer_system has no annotations")
	}
	// Read-only mode filters out every mutating action, so nothing dangerous
	// remains reachable and the tool should now be annotated read-only.
	if !readOnlyTool.Annotations.ReadOnlyHint {
		t.Error("ReadOnlyHint = false, want true: no mutating action is reachable in read-only mode")
	}
}

// A model must not be able to reach an action in another domain by naming it.
func TestCallTool_ActionFromAnotherDomain_IsRefused(t *testing.T) {
	t.Parallel()
	specs := append(system.Specs(), toolsSpecForOtherDomain())
	catalog, err := actioncatalog.Build(specs, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "portainer-mcp", Version: "test"}, nil)
	if err := (Surface{}).Register(server, catalog, tools.Deps{}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	session, ctx := connect(t, server)

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_system",
		Arguments: map[string]any{"action": "tags.list"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("the system meta-tool executed an action belonging to tags")
	}
}

// Safe mode must intercept through this surface too — a surface that called
// handlers directly would bypass it.
func TestCallTool_SafeMode_InterceptsThroughTheMetaSurface(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t,
		actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"},
		tools.Deps{SafeMode: true}))

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_system",
		Arguments: map[string]any{"action": "system.update", "input": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(strings.ToLower(res.Content[0].(*mcp.TextContent).Text), "safe mode") {
		t.Error("safe mode did not intercept a mutating action called through the meta surface")
	}
}

func toolsSpecForOtherDomain() toolutil.ActionSpec {
	return toolutil.ActionSpec{
		Name: "tags.list", Domain: "tags", OperationID: "TagList",
		Title: "List tags", Description: "d", Edition: edition.CE,
		Handler: func(context.Context, *portainer.Client, json.RawMessage) (any, error) { return nil, nil },
	}
}
