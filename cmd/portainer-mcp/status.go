package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/version"
)

// statusInput is empty: the status tool takes no arguments.
type statusInput struct{}

// statusOutput is the model-facing result of portainer_mcp_status.
//
// It deliberately carries no credential: the API token is configuration, but
// echoing it back to a model would leak it into transcripts.
type statusOutput struct {
	MCPVersion    string `json:"mcp_version"`
	PortainerURL  string `json:"portainer_url"`
	Edition       string `json:"edition"`
	ServerVersion string `json:"server_version"`
	ToolSurface   string `json:"tool_surface"`
	ActionCount   int    `json:"action_count"`
	ReadOnly      bool   `json:"read_only"`
	SafeMode      bool   `json:"safe_mode"`
}

// addStatusTool registers portainer_mcp_status on the server.
//
// resolvedEdition and serverVersion are the values buildCatalog resolved at
// startup — the configured override when PORTAINER_EDITION is set, the
// detected value otherwise — so the report reflects what the catalog was
// actually built for, not what was merely requested.
func addStatusTool(server *mcp.Server, cfg *config.Config, catalog *actioncatalog.Catalog, resolvedEdition edition.Edition, serverVersion string) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "portainer_mcp_status",
		Title:       "Portainer MCP status",
		Description: "Reports this MCP server's build version and active configuration: the Portainer URL it targets, the resolved edition and server version, the tool surface in use, how many actions the catalog exposes, and whether read-only or safe mode is enabled. Takes no arguments and never returns credentials.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ statusInput) (*mcp.CallToolResult, any, error) {
		out := statusOutput{
			MCPVersion:    version.String(),
			PortainerURL:  cfg.URL,
			Edition:       string(resolvedEdition),
			ServerVersion: serverVersion,
			ToolSurface:   string(cfg.ToolSurface),
			ActionCount:   len(catalog.Actions()),
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
