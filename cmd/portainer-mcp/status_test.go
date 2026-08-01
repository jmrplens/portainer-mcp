package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
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

func TestNewServer_ListTools_ExposesStatusTool(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		URL:         "https://portainer.example.com",
		Token:       "ptr_abc",
		ToolSurface: config.SurfaceDynamic,
		LogLevel:    slog.LevelInfo,
	}
	session, ctx := connect(t, newServer(cfg))

	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var names []string
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	if len(names) != 1 || names[0] != "portainer_mcp_status" {
		t.Errorf("tools = %v, want exactly [portainer_mcp_status]", names)
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
	session, ctx := connect(t, newServer(cfg))

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
	if got.ServerVersion == "" {
		t.Error("ServerVersion is empty, want the build metadata string")
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
	session, ctx := connect(t, newServer(cfg))

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "portainer_mcp_status"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if strings.Contains(text, "ptr_supersecret") {
		t.Error("the status tool leaked the API token into its result")
	}
}
