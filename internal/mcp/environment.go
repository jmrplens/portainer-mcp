package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddEnvironmentFeatures registers the environment (endpoint) management tools on the MCP server.
func (s *PortainerMCPServer) AddEnvironmentFeatures() {
	s.addToolIfExists(ToolListEnvironments, s.HandleGetEnvironments())
	s.addToolIfExists(ToolGetEnvironment, s.HandleGetEnvironment())

	if !s.readOnly {
		s.addToolIfExists(ToolDeleteEnvironment, s.HandleDeleteEnvironment())
		s.addToolIfExists(ToolSnapshotEnvironment, s.HandleSnapshotEnvironment())
		s.addToolIfExists(ToolSnapshotAllEnvironments, s.HandleSnapshotAllEnvironments())
		s.addToolIfExists(ToolUpdateEnvironmentTags, s.HandleUpdateEnvironmentTags())
		s.addToolIfExists(ToolUpdateEnvironmentUserAccesses, s.HandleUpdateEnvironmentUserAccesses())
		s.addToolIfExists(ToolUpdateEnvironmentTeamAccesses, s.HandleUpdateEnvironmentTeamAccesses())
	}
}

// HandleGetEnvironments returns an MCP tool handler that retrieves environments.
func (s *PortainerMCPServer) HandleGetEnvironments() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		environments, err := s.cli.GetEnvironments()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get environments", err), nil
		}

		return jsonResult(environments, "failed to marshal environments")
	}
}

// HandleGetEnvironment returns an MCP tool handler that retrieves environment.
func (s *PortainerMCPServer) HandleGetEnvironment() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		environment, err := s.cli.GetEnvironment(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get environment", err), nil
		}

		return jsonResult(environment, "failed to marshal environment")
	}
}

// HandleDeleteEnvironment returns an MCP tool handler that deletes environment.
func (s *PortainerMCPServer) HandleDeleteEnvironment() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = s.cli.DeleteEnvironment(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to delete environment", err), nil
		}

		return mcp.NewToolResultText("Environment deleted successfully"), nil
	}
}

// HandleSnapshotEnvironment returns an MCP tool handler that triggers a snapshot of environment.
func (s *PortainerMCPServer) HandleSnapshotEnvironment() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = s.cli.SnapshotEnvironment(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to snapshot environment", err), nil
		}

		return mcp.NewToolResultText("Environment snapshot created successfully"), nil
	}
}

// HandleSnapshotAllEnvironments returns an MCP tool handler that triggers a snapshot of all environments.
func (s *PortainerMCPServer) HandleSnapshotAllEnvironments() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		err := s.cli.SnapshotAllEnvironments()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to snapshot all environments", err), nil
		}

		return mcp.NewToolResultText("All environment snapshots created successfully"), nil
	}
}

// HandleUpdateEnvironmentTags returns an MCP tool handler that updates environment tags.
func (s *PortainerMCPServer) HandleUpdateEnvironmentTags() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		tagIds, err := parser.GetArrayOfIntegers("tagIds", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid tagIds parameter", err), nil
		}

		err = s.cli.UpdateEnvironmentTags(id, tagIds)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update environment tags", err), nil
		}

		return mcp.NewToolResultText("Environment tags updated successfully"), nil
	}
}

// HandleUpdateEnvironmentUserAccesses returns an MCP tool handler that updates environment user accesses.
func (s *PortainerMCPServer) HandleUpdateEnvironmentUserAccesses() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		userAccesses, err := parser.GetArrayOfObjects("userAccesses", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid userAccesses parameter", err), nil
		}

		userAccessesMap, err := parseAccessMap(userAccesses)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid user accesses", err), nil
		}

		err = s.cli.UpdateEnvironmentUserAccesses(id, userAccessesMap)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update environment user accesses", err), nil
		}

		return mcp.NewToolResultText("Environment user accesses updated successfully"), nil
	}
}

// HandleUpdateEnvironmentTeamAccesses returns an MCP tool handler that updates environment team accesses.
func (s *PortainerMCPServer) HandleUpdateEnvironmentTeamAccesses() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		teamAccesses, err := parser.GetArrayOfObjects("teamAccesses", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid teamAccesses parameter", err), nil
		}

		teamAccessesMap, err := parseAccessMap(teamAccesses)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid team accesses", err), nil
		}

		err = s.cli.UpdateEnvironmentTeamAccesses(id, teamAccessesMap)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update environment team accesses", err), nil
		}

		return mcp.NewToolResultText("Environment team accesses updated successfully"), nil
	}
}
