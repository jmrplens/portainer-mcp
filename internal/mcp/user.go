package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

func (s *PortainerMCPServer) AddUserFeatures() {
	s.addToolIfExists(ToolListUsers, s.HandleGetUsers())
	s.addToolIfExists(ToolGetUser, s.HandleGetUser())

	if !s.readOnly {
		s.addToolIfExists(ToolCreateUser, s.HandleCreateUser())
		s.addToolIfExists(ToolDeleteUser, s.HandleDeleteUser())
		s.addToolIfExists(ToolUpdateUserRole, s.HandleUpdateUserRole())
	}
}

func (s *PortainerMCPServer) HandleGetUsers() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		users, err := s.cli.GetUsers()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get users", err), nil
		}

		return jsonResult(users, "failed to marshal users")
	}
}

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
