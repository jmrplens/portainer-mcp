package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddCustomTemplateFeatures registers the custom template management tools on the MCP server.
func (s *PortainerMCPServer) AddCustomTemplateFeatures() {
	s.addToolIfExists(ToolListCustomTemplates, s.HandleListCustomTemplates())
	s.addToolIfExists(ToolGetCustomTemplate, s.HandleGetCustomTemplate())
	s.addToolIfExists(ToolGetCustomTemplateFile, s.HandleGetCustomTemplateFile())

	if !s.readOnly {
		s.addToolIfExists(ToolCreateCustomTemplate, s.HandleCreateCustomTemplate())
		s.addToolIfExists(ToolDeleteCustomTemplate, s.HandleDeleteCustomTemplate())
	}
}

// HandleListCustomTemplates returns an MCP tool handler that lists custom templates.
func (s *PortainerMCPServer) HandleListCustomTemplates() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		templates, err := s.cli.GetCustomTemplates()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to list custom templates", err), nil
		}

		return jsonResult(templates, "failed to marshal custom templates")
	}
}

// HandleGetCustomTemplate returns an MCP tool handler that retrieves custom template.
func (s *PortainerMCPServer) HandleGetCustomTemplate() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		template, err := s.cli.GetCustomTemplate(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get custom template", err), nil
		}

		return jsonResult(template, "failed to marshal custom template")
	}
}

// HandleGetCustomTemplateFile returns an MCP tool handler that retrieves custom template file.
func (s *PortainerMCPServer) HandleGetCustomTemplateFile() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		content, err := s.cli.GetCustomTemplateFile(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get custom template file", err), nil
		}

		return mcp.NewToolResultText(content), nil
	}
}

// HandleCreateCustomTemplate returns an MCP tool handler that creates custom template.
func (s *PortainerMCPServer) HandleCreateCustomTemplate() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		title, err := parser.GetString("title", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid title parameter", err), nil
		}

		description, err := parser.GetString("description", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid description parameter", err), nil
		}

		fileContent, err := parser.GetString("fileContent", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid fileContent parameter", err), nil
		}

		templateType, err := parser.GetInt("type", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid type parameter", err), nil
		}

		if !isValidTemplateType(templateType) {
			return mcp.NewToolResultError("invalid template type: must be 1-3 (1=swarm 2=compose 3=kubernetes)"), nil
		}

		platform, err := parser.GetInt("platform", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid platform parameter", err), nil
		}

		note, _ := parser.GetString("note", false)
		logo, _ := parser.GetString("logo", false)

		id, err := s.cli.CreateCustomTemplate(title, description, note, logo, fileContent, platform, templateType)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to create custom template", err), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Custom template created successfully with ID: %d", id)), nil
	}
}

// HandleDeleteCustomTemplate returns an MCP tool handler that deletes custom template.
func (s *PortainerMCPServer) HandleDeleteCustomTemplate() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = s.cli.DeleteCustomTemplate(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to delete custom template", err), nil
		}

		return mcp.NewToolResultText("Custom template deleted successfully"), nil
	}
}
