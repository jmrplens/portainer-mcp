package individual

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

func serverFor(t *testing.T, opts actioncatalog.Options) *mcp.Server {
	t.Helper()
	catalog, err := actioncatalog.Build(system.Specs(), opts)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "portainer-mcp", Version: "test"}, nil)
	if err := (Surface{}).Register(server, catalog, tools.Deps{}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return server
}

func toolNames(ctx context.Context, t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	return names
}

func TestRegister_EEServer_ExposesOneToolPerAction(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"}))
	names := toolNames(ctx, t, session)

	if len(names) != 5 {
		t.Errorf("tools = %v, want five for an EE system catalog", names)
	}
	joined := strings.Join(names, " ")
	if !strings.Contains(joined, "portainer_system_update") {
		t.Error("the EE-only update action is missing")
	}
	if strings.Contains(joined, "portainer_system_upgrade") {
		t.Error("the CE-only upgrade action appears on an EE server")
	}
}

func TestRegister_CEServer_SwapsTheMutatingAction(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, actioncatalog.Options{Edition: edition.CE, ServerVersion: "2.44.0"}))
	joined := strings.Join(toolNames(ctx, t, session), " ")

	if !strings.Contains(joined, "portainer_system_upgrade") {
		t.Error("the CE-only upgrade action is missing on a CE server")
	}
	if strings.Contains(joined, "portainer_system_update") {
		t.Error("the EE-only update action appears on a CE server")
	}
}

// Read-only must remove the mutating action from the tool list entirely, not
// merely refuse it at call time: a tool the model can see is a tool it will try.
func TestRegister_ReadOnly_OmitsMutatingToolsFromTheList(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, actioncatalog.Options{
		Edition: edition.EE, ServerVersion: "2.44.0", ReadOnly: true,
	}))
	names := toolNames(ctx, t, session)

	if len(names) != 4 {
		t.Errorf("tools = %v, want four read-only actions", names)
	}
	if strings.Contains(strings.Join(names, " "), "update") {
		t.Error("a mutating tool is listed in read-only mode")
	}
}

func TestRegister_Annotations_MarkReadOnlyActions(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"}))
	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, tool := range res.Tools {
		if tool.Annotations == nil {
			t.Fatalf("%s has no annotations", tool.Name)
		}
		wantReadOnly := tool.Name != "portainer_system_update"
		if tool.Annotations.ReadOnlyHint != wantReadOnly {
			t.Errorf("%s ReadOnlyHint = %v, want %v", tool.Name, tool.Annotations.ReadOnlyHint, wantReadOnly)
		}
	}
}

// TestCallTool_EachToolReachesItsOwnAction is the guard against a mis-wired
// closure: every existing test only inspects the tool list, so a Register that
// bound one handler to every tool would pass all of them.
func TestCallTool_EachToolReachesItsOwnAction(t *testing.T) {
	t.Parallel()
	specs := []toolutil.ActionSpec{
		{
			Name: "system.info", Domain: "system", OperationID: "SystemInfo",
			Title: "a", Description: "a", Edition: edition.CE,
			Handler: func(context.Context, *portainer.Client, json.RawMessage) (any, error) {
				return map[string]any{"which": "info"}, nil
			},
		},
		{
			Name: "system.status", Domain: "system", OperationID: "SystemStatus",
			Title: "b", Description: "b", Edition: edition.CE,
			Handler: func(context.Context, *portainer.Client, json.RawMessage) (any, error) {
				return map[string]any{"which": "status"}, nil
			},
		},
	}
	catalog, err := actioncatalog.Build(specs, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "portainer-mcp", Version: "test"}, nil)
	// A non-nil client: Execute now refuses a nil one before any handler runs
	// (see internal/tools/register.go), and this test is about dispatch
	// reaching the right stub handler, not about client-nilness.
	if err := (Surface{}).Register(server, catalog, tools.Deps{Client: &portainer.Client{}}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	session, ctx := connect(t, server)

	for tool, want := range map[string]string{
		"portainer_system_info":   "info",
		"portainer_system_status": "status",
	} {
		res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: map[string]any{}})
		if err != nil {
			t.Fatalf("CallTool(%s): %v", tool, err)
		}
		if res.IsError {
			t.Fatalf("CallTool(%s) returned an error: %+v", tool, res.Content)
		}
		text := res.Content[0].(*mcp.TextContent).Text
		if !strings.Contains(text, want) {
			t.Errorf("CallTool(%s) reached the wrong action: got %q, want it to mention %q", tool, text, want)
		}
	}
}
