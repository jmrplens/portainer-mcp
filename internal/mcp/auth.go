package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddAuthFeatures registers the authentication management tools on the MCP server.
func (s *PortainerMCPServer) AddAuthFeatures() {
	s.addToolIfExists(ToolAuthenticate, s.HandleAuthenticateUser())

	if !s.readOnly {
		s.addToolIfExists(ToolLogout, s.HandleLogout())
	}
}

// HandleAuthenticateUser returns an MCP tool handler that authenticates user.
func (s *PortainerMCPServer) HandleAuthenticateUser() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		username, err := parser.GetString("username", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid username parameter", err), nil
		}

		password, err := parser.GetString("password", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid password parameter", err), nil
		}

		authResponse, err := s.cli.AuthenticateUser(username, password)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to authenticate user", err), nil
		}

		return jsonResult(authResponse, "failed to marshal authentication response")
	}
}

// HandleLogout returns an MCP tool handler that logs out authentication.
func (s *PortainerMCPServer) HandleLogout() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		err := s.cli.Logout()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to logout", err), nil
		}

		return mcp.NewToolResultText("Logged out successfully"), nil
	}
}
