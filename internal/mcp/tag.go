package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddTagFeatures registers the environment tag management tools on the MCP server.
func (s *PortainerMCPServer) AddTagFeatures() {
	s.addToolIfExists(ToolListEnvironmentTags, s.HandleGetEnvironmentTags())

	if !s.readOnly {
		s.addToolIfExists(ToolCreateEnvironmentTag, s.HandleCreateEnvironmentTag())
		s.addToolIfExists(ToolDeleteEnvironmentTag, s.HandleDeleteEnvironmentTag())
	}
}

// HandleGetEnvironmentTags returns an MCP tool handler that retrieves environment tags.
func (s *PortainerMCPServer) HandleGetEnvironmentTags() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		environmentTags, err := s.cli.GetEnvironmentTags()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get environment tags", err), nil
		}

		return jsonResult(environmentTags, "failed to marshal environment tags")
	}
}

// HandleCreateEnvironmentTag returns an MCP tool handler that creates environment tag.
func (s *PortainerMCPServer) HandleCreateEnvironmentTag() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		name, err := parser.GetString("name", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}
		if err := validateName(name); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		id, err := s.cli.CreateEnvironmentTag(name)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to create environment tag", err), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Environment tag created successfully with ID: %d", id)), nil
	}
}

// HandleDeleteEnvironmentTag returns an MCP tool handler that deletes environment tag.
func (s *PortainerMCPServer) HandleDeleteEnvironmentTag() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		err = s.cli.DeleteEnvironmentTag(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to delete environment tag", err), nil
		}

		return mcp.NewToolResultText("Environment tag deleted successfully"), nil
	}
}
