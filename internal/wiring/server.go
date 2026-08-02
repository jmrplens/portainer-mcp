package wiring

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/version"
)

const projectRepository = "https://github.com/jmrplens/portainer-mcp"

// NewServer builds the MCP server for the given configuration, registering
// the status tool and the configured tool surface projected from catalog.
//
// The surface is always obtained through SurfaceFor rather than by
// referencing a surface package directly here: that keeps the mapping from
// config.ToolSurface to its projection in exactly one place. cmd/portainer-mcp
// and the e2e harness both call this rather than assembling an *mcp.Server
// themselves, so both see identical tool registration.
func NewServer(cfg *config.Config, catalog *actioncatalog.Catalog, deps tools.Deps, resolvedEdition edition.Edition, serverVersion string) (*mcp.Server, error) {
	server := mcp.NewServer(&mcp.Implementation{
		Name:       "portainer-mcp",
		Title:      "Portainer MCP Server",
		Version:    version.Version,
		WebsiteURL: projectRepository,
	}, &mcp.ServerOptions{
		Instructions: "portainer-mcp exposes the Portainer REST API as MCP tools for managing " +
			"container environments, stacks, registries, users, teams and edge deployments.\n\n" +
			"Call portainer_mcp_status to discover the active tool surface and whether read-only " +
			"or safe mode is enabled before attempting any mutating operation.",
	})
	addStatusTool(server, cfg, catalog, resolvedEdition, serverVersion)

	if err := SurfaceFor(cfg.ToolSurface).Register(server, catalog, deps); err != nil {
		return nil, fmt.Errorf("register tool surface %q: %w", cfg.ToolSurface, err)
	}
	return server, nil
}

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
// resolvedEdition and serverVersion are the values BuildCatalog resolved at
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
