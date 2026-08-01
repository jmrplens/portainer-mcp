package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/version"
)

// statusInput is empty: the status tool takes no arguments.
type statusInput struct{}

// statusOutput is the model-facing result of portainer_mcp_status.
//
// It deliberately carries no credential: the API token is configuration, but
// echoing it back to a model would leak it into transcripts.
type statusOutput struct {
	ServerVersion string `json:"server_version"`
	PortainerURL  string `json:"portainer_url"`
	ToolSurface   string `json:"tool_surface"`
	ReadOnly      bool   `json:"read_only"`
	SafeMode      bool   `json:"safe_mode"`
}

// addStatusTool registers portainer_mcp_status on the server.
func addStatusTool(server *mcp.Server, cfg *config.Config) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "portainer_mcp_status",
		Title:       "Portainer MCP status",
		Description: "Reports this MCP server's build version and active configuration: the Portainer URL it targets, the tool surface in use, and whether read-only or safe mode is enabled. Takes no arguments and never returns credentials.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ statusInput) (*mcp.CallToolResult, any, error) {
		out := statusOutput{
			ServerVersion: version.String(),
			PortainerURL:  cfg.URL,
			ToolSurface:   string(cfg.ToolSurface),
			ReadOnly:      cfg.ReadOnly,
			SafeMode:      cfg.SafeMode,
		}
		encoded, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("encode status: %w", err)
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}},
		}, out, nil
	})
}
