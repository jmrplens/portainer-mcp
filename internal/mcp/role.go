package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// AddRoleFeatures registers the role management tools on the MCP server.
func (s *PortainerMCPServer) AddRoleFeatures() {
	s.addToolIfExists(ToolListRoles, s.HandleListRoles())
}

// HandleListRoles returns an MCP tool handler that lists roles.
func (s *PortainerMCPServer) HandleListRoles() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		roles, err := s.cli.GetRoles()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to list roles", err), nil
		}

		return jsonResult(roles, "failed to marshal roles")
	}
}
