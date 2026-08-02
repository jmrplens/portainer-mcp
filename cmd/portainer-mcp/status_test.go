package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
)

// connect wires an in-memory client to the server under test. It uses the
// SDK's paired transports, so no network or subprocess is involved.
func connect(t *testing.T, server *mcp.Server) (*mcp.ClientSession, context.Context) {
	t.Helper()
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, ctx
}

// testCatalog builds a small, real catalog from the project's declared
// specs so status tests exercise the same shape newServer sees in
// production, rather than an empty stand-in.
func testCatalog(t *testing.T, opts actioncatalog.Options) *actioncatalog.Catalog {
	t.Helper()
	catalog, err := actioncatalog.Build(allSpecs(), opts)
	if err != nil {
		t.Fatalf("actioncatalog.Build: %v", err)
	}
	return catalog
}

// mustNewServer wraps newServer for tests that do not expect it to fail.
func mustNewServer(t *testing.T, cfg *config.Config, catalog *actioncatalog.Catalog, resolvedEdition edition.Edition) *mcp.Server {
	t.Helper()
	srv, err := newServer(cfg, catalog, tools.Deps{}, resolvedEdition, "2.44.0")
	if err != nil {
		t.Fatalf("newServer: %v", err)
	}
	return srv
}

func TestNewServer_ListTools_ExposesStatusAndSurfaceTools(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		URL:         "https://portainer.example.com",
		Token:       "ptr_abc",
		ToolSurface: config.SurfaceDynamic,
		LogLevel:    slog.LevelInfo,
	}
	catalog := testCatalog(t, actioncatalog.Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	session, ctx := connect(t, mustNewServer(t, cfg, catalog, edition.CE))

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}

	// The dynamic surface adds its find/execute pair alongside the status
	// tool: three tools regardless of how many actions the catalog holds.
	want := []string{"portainer_mcp_status", "portainer_find_action", "portainer_execute_action"}
	if len(names) != len(want) {
		t.Fatalf("tools = %v, want exactly %v", names, want)
	}
	for _, w := range want {
		found := false
		for _, n := range names {
			if n == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("tools = %v, missing %q", names, w)
		}
	}
}

// TestNewServer_ToolSurfaceConfig_SelectsMatchingSurface pins the
// single-selection-point rule: newServer must project the catalog through
// whichever surface cfg.ToolSurface names, and it must do so by asking
// surfaceFor rather than by wiring one surface package in directly. A
// newServer that hardcoded a surface package here would still pass every
// other status test (they only exercise one surface each), so this test
// exists specifically to fail when the mapping is bypassed.
func TestNewServer_ToolSurfaceConfig_SelectsMatchingSurface(t *testing.T) {
	t.Parallel()
	catalog := testCatalog(t, actioncatalog.Options{Edition: edition.CE, ServerVersion: "2.44.0"})

	tests := []struct {
		name    string
		surface config.ToolSurface
		want    []string
	}{
		{
			name:    "dynamic",
			surface: config.SurfaceDynamic,
			want:    []string{"portainer_mcp_status", "portainer_find_action", "portainer_execute_action"},
		},
		{
			name:    "meta",
			surface: config.SurfaceMeta,
			// One tool per domain: the catalog built from allSpecs() carries
			// exactly the system, tags and registries pilot domains.
			want: []string{"portainer_mcp_status", "portainer_system", "portainer_tags", "portainer_registries"},
		},
		{
			name:    "individual",
			surface: config.SurfaceIndividual,
			// One tool per action, plus status. tags.list is a known action
			// from the tags pilot domain, checked as a representative sample.
			want: []string{"portainer_mcp_status", "portainer_tags_list"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := &config.Config{
				URL:         "https://portainer.example.com",
				Token:       "ptr_abc",
				ToolSurface: tt.surface,
			}
			session, ctx := connect(t, mustNewServer(t, cfg, catalog, edition.CE))

			res, err := session.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("ListTools: %v", err)
			}
			names := make(map[string]bool, len(res.Tools))
			got := make([]string, 0, len(res.Tools))
			for _, tool := range res.Tools {
				names[tool.Name] = true
				got = append(got, tool.Name)
			}

			for _, w := range tt.want {
				if !names[w] {
					t.Errorf("surface %q: tools = %v, missing %q", tt.surface, got, w)
				}
			}

			if tt.surface == config.SurfaceIndividual {
				// The individual surface registers exactly one tool per
				// catalog action plus status: an exact count, not just a
				// sample, catches a mismatch that a spot check would miss.
				if len(names) != len(catalog.Actions())+1 {
					t.Errorf("surface individual: got %d tools, want %d (one per action, plus status)",
						len(names), len(catalog.Actions())+1)
				}
			} else if len(names) != len(tt.want) {
				t.Errorf("surface %q: got %d tools %v, want exactly %v", tt.surface, len(names), got, tt.want)
			}
		})
	}
}

func TestStatusTool_Call_ReportsConfiguration(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		URL:         "https://portainer.example.com",
		Token:       "ptr_abc",
		ToolSurface: config.SurfaceMeta,
		ReadOnly:    true,
		LogLevel:    slog.LevelInfo,
	}
	catalog := testCatalog(t, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0", ReadOnly: true})
	session, ctx := connect(t, mustNewServer(t, cfg, catalog, edition.EE))

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "portainer_mcp_status"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("CallTool returned IsError=true: %+v", res.Content)
	}

	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	var got statusOutput
	if err := json.Unmarshal([]byte(text.Text), &got); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if got.ToolSurface != "meta" {
		t.Errorf("ToolSurface = %q, want %q", got.ToolSurface, "meta")
	}
	if !got.ReadOnly {
		t.Error("ReadOnly = false, want true")
	}
	if got.PortainerURL != "https://portainer.example.com" {
		t.Errorf("PortainerURL = %q, want the configured URL", got.PortainerURL)
	}
	if got.MCPVersion == "" {
		t.Error("MCPVersion is empty, want the build metadata string")
	}
	if got.Edition != "EE" {
		t.Errorf("Edition = %q, want %q", got.Edition, "EE")
	}
	if got.ServerVersion != "2.44.0" {
		t.Errorf("ServerVersion = %q, want the resolved Portainer server version %q", got.ServerVersion, "2.44.0")
	}
	if got.ActionCount != len(catalog.Actions()) {
		t.Errorf("ActionCount = %d, want the catalog's actual size %d", got.ActionCount, len(catalog.Actions()))
	}
	if got.ActionCount == 0 {
		t.Error("ActionCount = 0, want at least one action from the pilot domains")
	}
}

// TestStatusTool_Call_NeverLeaksTheToken is a security guardrail: the status
// tool reports configuration back to a model, so it must not echo credentials.
func TestStatusTool_Call_NeverLeaksTheToken(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		URL:         "https://portainer.example.com",
		Token:       "ptr_supersecret",
		ToolSurface: config.SurfaceDynamic,
	}
	catalog := testCatalog(t, actioncatalog.Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	session, ctx := connect(t, mustNewServer(t, cfg, catalog, edition.CE))

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "portainer_mcp_status"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if strings.Contains(text, "ptr_supersecret") {
		t.Error("the status tool leaked the API token into its result")
	}
}
