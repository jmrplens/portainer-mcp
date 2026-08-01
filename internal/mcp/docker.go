package mcp

import (
	"context"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/toolgen"
)

// AddDockerProxyFeatures registers the Docker proxy management tools on the MCP server.
func (s *PortainerMCPServer) AddDockerProxyFeatures() {
	s.addToolIfExists(ToolGetDockerDashboard, s.HandleGetDockerDashboard())

	if !s.readOnly {
		s.addToolIfExists(ToolDockerProxy, s.HandleDockerProxy())
	}
}

// HandleDockerProxy proxies arbitrary Docker API requests to a Portainer environment.
//
// SECURITY NOTE: This handler allows the caller to invoke any Docker Engine API endpoint
// (e.g. /containers, /exec, /volumes, /networks, /swarm) on the target environment.
// There is no allowlist restricting which API paths are permitted. The only validation
// performed is that the path starts with "/" and the HTTP method is one of the supported
// set. Access control relies entirely on the Portainer API token permissions and the
// read-only mode flag. Operators should be aware that this effectively grants full Docker
// API access to whoever holds the MCP server's Portainer token.
func (s *PortainerMCPServer) HandleDockerProxy() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		params, toolErr := parseProxyParams(request, "dockerAPIPath")
		if toolErr != nil {
			return toolErr, nil
		}

		opts := models.DockerProxyRequestOptions{
			EnvironmentID: params.environmentID,
			Path:          params.apiPath,
			Method:        params.method,
			QueryParams:   params.queryParams,
			Headers:       params.headers,
		}
		if params.body != "" {
			opts.Body = strings.NewReader(params.body)
		}

		response, err := s.cli.ProxyDockerRequest(opts)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to send Docker API request", err), nil
		}

		return readProxyResponse(response, "Docker")
	}
}

// HandleGetDockerDashboard returns an MCP tool handler that retrieves docker dashboard.
func (s *PortainerMCPServer) HandleGetDockerDashboard() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", environmentId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dashboard, err := s.cli.GetDockerDashboard(environmentId)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get docker dashboard", err), nil
		}

		return jsonResult(dashboard, "failed to marshal docker dashboard")
	}
}
