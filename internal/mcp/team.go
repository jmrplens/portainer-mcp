package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddTeamFeatures registers the team management tools on the MCP server.
func (s *PortainerMCPServer) AddTeamFeatures() {
	s.addToolIfExists(ToolListTeams, s.HandleGetTeams())
	s.addToolIfExists(ToolGetTeam, s.HandleGetTeam())

	if !s.readOnly {
		s.addToolIfExists(ToolCreateTeam, s.HandleCreateTeam())
		s.addToolIfExists(ToolDeleteTeam, s.HandleDeleteTeam())
		s.addToolIfExists(ToolUpdateTeamName, s.HandleUpdateTeamName())
		s.addToolIfExists(ToolUpdateTeamMembers, s.HandleUpdateTeamMembers())
	}
}

// HandleCreateTeam returns an MCP tool handler that creates team.
func (s *PortainerMCPServer) HandleCreateTeam() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		name, err := parser.GetString("name", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}
		if err := validateName(name); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		teamID, err := s.cli.CreateTeam(name)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to create team", err), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Team created successfully with ID: %d", teamID)), nil
	}
}

// HandleGetTeams returns an MCP tool handler that retrieves teams.
func (s *PortainerMCPServer) HandleGetTeams() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		teams, err := s.cli.GetTeams()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get teams", err), nil
		}

		return jsonResult(teams, "failed to marshal teams")
	}
}

// HandleGetTeam returns an MCP tool handler that retrieves team.
func (s *PortainerMCPServer) HandleGetTeam() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		team, err := s.cli.GetTeam(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get team", err), nil
		}

		return jsonResult(team, "failed to marshal team")
	}
}

// HandleDeleteTeam returns an MCP tool handler that deletes team.
func (s *PortainerMCPServer) HandleDeleteTeam() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		err = s.cli.DeleteTeam(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to delete team", err), nil
		}

		return mcp.NewToolResultText("Team deleted successfully"), nil
	}
}

// HandleUpdateTeamName returns an MCP tool handler that updates team name.
func (s *PortainerMCPServer) HandleUpdateTeamName() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		name, err := parser.GetString("name", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}
		if err := validateName(name); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = s.cli.UpdateTeamName(id, name)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update team name", err), nil
		}

		return mcp.NewToolResultText("Team name updated successfully"), nil
	}
}

// HandleUpdateTeamMembers returns an MCP tool handler that updates team members.
func (s *PortainerMCPServer) HandleUpdateTeamMembers() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		userIDs, err := parser.GetArrayOfIntegers("userIds", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid userIds parameter", err), nil
		}

		err = s.cli.UpdateTeamMembers(id, userIDs)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update team members", err), nil
		}

		return mcp.NewToolResultText("Team members updated successfully"), nil
	}
}
