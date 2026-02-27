package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddWebhookFeatures registers the webhook management tools on the MCP server.
func (s *PortainerMCPServer) AddWebhookFeatures() {
	s.addToolIfExists(ToolListWebhooks, s.HandleListWebhooks())

	if !s.readOnly {
		s.addToolIfExists(ToolCreateWebhook, s.HandleCreateWebhook())
		s.addToolIfExists(ToolDeleteWebhook, s.HandleDeleteWebhook())
	}
}

// HandleListWebhooks returns an MCP tool handler that lists webhooks.
func (s *PortainerMCPServer) HandleListWebhooks() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		webhooks, err := s.cli.GetWebhooks()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get webhooks", err), nil
		}

		return jsonResult(webhooks, "failed to marshal webhooks")
	}
}

// HandleCreateWebhook returns an MCP tool handler that creates webhook.
func (s *PortainerMCPServer) HandleCreateWebhook() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		resourceId, err := parser.GetString("resourceId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid resourceId parameter", err), nil
		}

		endpointId, err := parser.GetInt("endpointId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid endpointId parameter", err), nil
		}
		if err := validatePositiveID("endpointId", endpointId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		webhookType, err := parser.GetInt("webhookType", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid webhookType parameter", err), nil
		}
		if !isValidWebhookType(webhookType) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid webhookType: %d (must be 1=service or 2=container)", webhookType)), nil
		}

		id, err := s.cli.CreateWebhook(resourceId, endpointId, webhookType)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to create webhook", err), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Webhook created successfully with ID: %d", id)), nil
	}
}

// HandleDeleteWebhook returns an MCP tool handler that deletes webhook.
func (s *PortainerMCPServer) HandleDeleteWebhook() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = s.cli.DeleteWebhook(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to delete webhook", err), nil
		}

		return mcp.NewToolResultText("Webhook deleted successfully"), nil
	}
}
