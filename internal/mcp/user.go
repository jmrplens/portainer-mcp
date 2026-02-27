package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddUserFeatures registers the user management tools on the MCP server.
func (s *PortainerMCPServer) AddUserFeatures() {
	s.addToolIfExists(ToolListUsers, s.HandleGetUsers())
	s.addToolIfExists(ToolGetUser, s.HandleGetUser())

	if !s.readOnly {
		s.addToolIfExists(ToolCreateUser, s.HandleCreateUser())
		s.addToolIfExists(ToolDeleteUser, s.HandleDeleteUser())
		s.addToolIfExists(ToolUpdateUserRole, s.HandleUpdateUserRole())
	}
}

// HandleGetUsers returns an MCP tool handler that retrieves users.
func (s *PortainerMCPServer) HandleGetUsers() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		users, err := s.cli.GetUsers()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get users", err), nil
		}

		return jsonResult(users, "failed to marshal users")
	}
}

// HandleUpdateUserRole returns an MCP tool handler that updates user role.
func (s *PortainerMCPServer) HandleUpdateUserRole() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		role, err := parser.GetString("role", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid role parameter", err), nil
		}

		if !isValidUserRole(role) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid role %s: must be one of: %v", role, AllUserRoles)), nil
		}

		err = s.cli.UpdateUserRole(id, role)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update user role", err), nil
		}

		return mcp.NewToolResultText("User updated successfully"), nil
	}
}

// HandleCreateUser returns an MCP tool handler that creates user.
func (s *PortainerMCPServer) HandleCreateUser() server.ToolHandlerFunc {
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

		role, err := parser.GetString("role", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid role parameter", err), nil
		}

		if !isValidUserRole(role) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid role %s: must be one of: %v", role, AllUserRoles)), nil
		}

		id, err := s.cli.CreateUser(username, password, role)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to create user", err), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("User created successfully with ID: %d", id)), nil
	}
}

// HandleGetUser returns an MCP tool handler that retrieves user.
func (s *PortainerMCPServer) HandleGetUser() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		user, err := s.cli.GetUser(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get user", err), nil
		}

		return jsonResult(user, "failed to marshal user")
	}
}

// HandleDeleteUser returns an MCP tool handler that deletes user.
func (s *PortainerMCPServer) HandleDeleteUser() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = s.cli.DeleteUser(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to delete user", err), nil
		}

		return mcp.NewToolResultText("User deleted successfully"), nil
	}
}
