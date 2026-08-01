package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// AddSystemFeatures registers the system status management tools on the MCP server.
func (s *PortainerMCPServer) AddSystemFeatures() {
	s.addToolIfExists(ToolGetSystemStatus, s.HandleGetSystemStatus())
}

// HandleGetSystemStatus returns an MCP tool handler that retrieves system status.
func (s *PortainerMCPServer) HandleGetSystemStatus() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status, err := s.cli.GetSystemStatus()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get system status", err), nil
		}

		return jsonResult(status, "failed to marshal system status")
	}
}
