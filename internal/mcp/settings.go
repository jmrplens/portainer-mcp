package mcp

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddSettingsFeatures registers the Portainer settings management tools on the MCP server.
func (s *PortainerMCPServer) AddSettingsFeatures() {
	s.addToolIfExists(ToolGetSettings, s.HandleGetSettings())
	s.addToolIfExists(ToolGetPublicSettings, s.HandleGetPublicSettings())

	if !s.readOnly {
		s.addToolIfExists(ToolUpdateSettings, s.HandleUpdateSettings())
	}
}

// HandleGetSettings returns an MCP tool handler that retrieves settings.
func (s *PortainerMCPServer) HandleGetSettings() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		settings, err := s.cli.GetSettings()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get settings", err), nil
		}

		return jsonResult(settings, "failed to marshal settings")
	}
}

// HandleUpdateSettings handles the updateSettings tool call.
// It accepts a JSON string parameter containing the settings fields to update.
//
// SECURITY NOTE: This handler passes the JSON settings map directly to the Portainer
// API without validating or restricting which fields can be modified. This means the
// caller can change any Portainer setting including authentication methods, edge compute
// features, and other security-sensitive configuration. Access control relies on the
// Portainer API token permissions. Consider restricting allowed fields in the future
// if a more granular access model is needed.
func (s *PortainerMCPServer) HandleUpdateSettings() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		settingsJSON, err := parser.GetString("settings", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid settings parameter", err), nil
		}

		var settingsMap map[string]interface{}
		if err := json.Unmarshal([]byte(settingsJSON), &settingsMap); err != nil {
			return mcp.NewToolResultErrorFromErr("failed to parse settings JSON", err), nil
		}

		if err := s.cli.UpdateSettings(settingsMap); err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update settings", err), nil
		}

		return mcp.NewToolResultText("Settings updated successfully"), nil
	}
}

// HandleGetPublicSettings handles the getPublicSettings tool call.
func (s *PortainerMCPServer) HandleGetPublicSettings() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		publicSettings, err := s.cli.GetPublicSettings()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get public settings", err), nil
		}

		return jsonResult(publicSettings, "failed to marshal public settings")
	}
}
