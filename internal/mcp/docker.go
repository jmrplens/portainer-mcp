package mcp

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
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
		parser := toolgen.NewParameterParser(request)

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", environmentId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		method, err := parser.GetString("method", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid method parameter", err), nil
		}
		if !isValidHTTPMethod(method) {
			return mcp.NewToolResultError(fmt.Sprintf("invalid method: %s", method)), nil
		}

		dockerAPIPath, err := parser.GetString("dockerAPIPath", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid dockerAPIPath parameter", err), nil
		}
		if !strings.HasPrefix(dockerAPIPath, "/") {
			return mcp.NewToolResultError("dockerAPIPath must start with a leading slash"), nil
		}

		queryParams, err := parser.GetArrayOfObjects("queryParams", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid queryParams parameter", err), nil
		}
		queryParamsMap, err := parseKeyValueMap(queryParams)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid query params", err), nil
		}

		headers, err := parser.GetArrayOfObjects("headers", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid headers parameter", err), nil
		}
		headersMap, err := parseKeyValueMap(headers)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid headers", err), nil
		}

		body, err := parser.GetString("body", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid body parameter", err), nil
		}

		opts := models.DockerProxyRequestOptions{
			EnvironmentID: environmentId,
			Path:          dockerAPIPath,
			Method:        method,
			QueryParams:   queryParamsMap,
			Headers:       headersMap,
		}

		if body != "" {
			opts.Body = strings.NewReader(body)
		}

		response, err := s.cli.ProxyDockerRequest(opts)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to send Docker API request", err), nil
		}
		defer response.Body.Close()

		responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxProxyResponseSize))
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to read Docker API response", err), nil
		}

		return mcp.NewToolResultText(string(responseBody)), nil
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
