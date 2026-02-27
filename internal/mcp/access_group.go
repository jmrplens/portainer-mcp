// Package mcp implements the Portainer MCP server and all its tool handlers.
// It provides the core server infrastructure, meta-tool routing, and individual
// handlers for each Portainer resource domain (environments, stacks, users,
// Docker, Kubernetes, etc.). The package bridges the MCP protocol with the
// Portainer API client layer.
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddAccessGroupFeatures registers the access group management tools on the MCP server.
func (s *PortainerMCPServer) AddAccessGroupFeatures() {
	s.addToolIfExists(ToolListAccessGroups, s.HandleGetAccessGroups())

	if !s.readOnly {
		s.addToolIfExists(ToolCreateAccessGroup, s.HandleCreateAccessGroup())
		s.addToolIfExists(ToolUpdateAccessGroupName, s.HandleUpdateAccessGroupName())
		s.addToolIfExists(ToolUpdateAccessGroupUserAccesses, s.HandleUpdateAccessGroupUserAccesses())
		s.addToolIfExists(ToolUpdateAccessGroupTeamAccesses, s.HandleUpdateAccessGroupTeamAccesses())
		s.addToolIfExists(ToolAddEnvironmentToAccessGroup, s.HandleAddEnvironmentToAccessGroup())
		s.addToolIfExists(ToolRemoveEnvironmentFromAccessGroup, s.HandleRemoveEnvironmentFromAccessGroup())
	}
}

// HandleGetAccessGroups returns an MCP tool handler that retrieves access groups.
func (s *PortainerMCPServer) HandleGetAccessGroups() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		accessGroups, err := s.cli.GetAccessGroups()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get access groups", err), nil
		}

		return jsonResult(accessGroups, "failed to marshal access groups")
	}
}

// HandleCreateAccessGroup returns an MCP tool handler that creates access group.
func (s *PortainerMCPServer) HandleCreateAccessGroup() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		name, err := parser.GetString("name", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}
		if err := validateName(name); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		environmentIds, err := parser.GetArrayOfIntegers("environmentIds", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentIds parameter", err), nil
		}

		groupID, err := s.cli.CreateAccessGroup(name, environmentIds)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to create access group", err), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Access group created successfully with ID: %d", groupID)), nil
	}
}

// HandleUpdateAccessGroupName returns an MCP tool handler that updates access group name.
func (s *PortainerMCPServer) HandleUpdateAccessGroupName() server.ToolHandlerFunc {
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

		err = s.cli.UpdateAccessGroupName(id, name)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update access group name", err), nil
		}

		return mcp.NewToolResultText("Access group name updated successfully"), nil
	}
}

// HandleUpdateAccessGroupUserAccesses returns an MCP tool handler that updates access group user accesses.
func (s *PortainerMCPServer) HandleUpdateAccessGroupUserAccesses() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		userAccesses, err := parser.GetArrayOfObjects("userAccesses", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid userAccesses parameter", err), nil
		}

		userAccessesMap, err := parseAccessMap(userAccesses)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid user accesses", err), nil
		}

		err = s.cli.UpdateAccessGroupUserAccesses(id, userAccessesMap)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update access group user accesses", err), nil
		}

		return mcp.NewToolResultText("Access group user accesses updated successfully"), nil
	}
}

// HandleUpdateAccessGroupTeamAccesses returns an MCP tool handler that updates access group team accesses.
func (s *PortainerMCPServer) HandleUpdateAccessGroupTeamAccesses() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		teamAccesses, err := parser.GetArrayOfObjects("teamAccesses", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid teamAccesses parameter", err), nil
		}

		teamAccessesMap, err := parseAccessMap(teamAccesses)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid team accesses", err), nil
		}

		err = s.cli.UpdateAccessGroupTeamAccesses(id, teamAccessesMap)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update access group team accesses", err), nil
		}

		return mcp.NewToolResultText("Access group team accesses updated successfully"), nil
	}
}

// HandleAddEnvironmentToAccessGroup returns an MCP tool handler that registers environment to access group.
func (s *PortainerMCPServer) HandleAddEnvironmentToAccessGroup() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}

		err = s.cli.AddEnvironmentToAccessGroup(id, environmentId)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to add environment to access group", err), nil
		}

		return mcp.NewToolResultText("Environment added to access group successfully"), nil
	}
}

// HandleRemoveEnvironmentFromAccessGroup returns an MCP tool handler that removes environment from access group.
func (s *PortainerMCPServer) HandleRemoveEnvironmentFromAccessGroup() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}

		err = s.cli.RemoveEnvironmentFromAccessGroup(id, environmentId)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to remove environment from access group", err), nil
		}

		return mcp.NewToolResultText("Environment removed from access group successfully"), nil
	}
}
