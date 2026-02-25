package mcp

import (
	"context"
	"encoding/json"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func (s *PortainerMCPServer) AddSystemFeatures() {
	s.addToolIfExists(ToolGetSystemStatus, s.HandleGetSystemStatus())
}

func (s *PortainerMCPServer) HandleGetSystemStatus() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status, err := s.cli.GetSystemStatus()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get system status", err), nil
		}

		data, err := json.Marshal(status)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to marshal system status", err), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}
