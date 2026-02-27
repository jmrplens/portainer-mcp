package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddAppTemplateFeatures registers app template-related tools.
func (s *PortainerMCPServer) AddAppTemplateFeatures() {
	s.addToolIfExists(ToolListAppTemplates, s.HandleListAppTemplates())
	s.addToolIfExists(ToolGetAppTemplateFile, s.HandleGetAppTemplateFile())
}

// HandleListAppTemplates handles the listAppTemplates tool call.
func (s *PortainerMCPServer) HandleListAppTemplates() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		templates, err := s.cli.GetAppTemplates()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to list app templates", err), nil
		}

		return jsonResult(templates, "failed to marshal app templates")
	}
}

// HandleGetAppTemplateFile handles the getAppTemplateFile tool call.
func (s *PortainerMCPServer) HandleGetAppTemplateFile() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		content, err := s.cli.GetAppTemplateFile(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr(fmt.Sprintf("failed to get app template file for template %d", id), err), nil
		}

		return mcp.NewToolResultText(content), nil
	}
}
