package mcp

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/internal/k8sutil"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddKubernetesProxyFeatures registers the Kubernetes proxy and resource management tools on the MCP server.
func (s *PortainerMCPServer) AddKubernetesProxyFeatures() {
	s.addToolIfExists(ToolKubernetesProxyStripped, s.HandleKubernetesProxyStripped())

	if !s.readOnly {
		s.addToolIfExists(ToolKubernetesProxy, s.HandleKubernetesProxy())
	}
}

// HandleKubernetesProxyStripped returns an MCP tool handler that handles kubernetes proxy stripped.
func (s *PortainerMCPServer) HandleKubernetesProxyStripped() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", environmentId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		kubernetesAPIPath, err := parser.GetString("kubernetesAPIPath", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid kubernetesAPIPath parameter", err), nil
		}
		if !strings.HasPrefix(kubernetesAPIPath, "/") {
			return mcp.NewToolResultError("kubernetesAPIPath must start with a leading slash"), nil
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

		opts := models.KubernetesProxyRequestOptions{
			EnvironmentID: environmentId,
			Path:          kubernetesAPIPath,
			Method:        "GET",
			QueryParams:   queryParamsMap,
			Headers:       headersMap,
		}

		response, err := s.cli.ProxyKubernetesRequest(opts)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to send Kubernetes API request", err), nil
		}

		responseBody, err := k8sutil.ProcessRawKubernetesAPIResponse(response)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to process Kubernetes API response", err), nil
		}

		return mcp.NewToolResultText(string(responseBody)), nil
	}
}

// HandleKubernetesProxy proxies arbitrary Kubernetes API requests to a Portainer environment.
//
// SECURITY NOTE: This handler allows the caller to invoke any Kubernetes API endpoint
// on the target environment. There is no allowlist restricting which API paths are
// permitted. Access control relies entirely on the Portainer API token permissions and
// the read-only mode flag. Operators should be aware that this grants broad Kubernetes
// API access to whoever holds the MCP server's Portainer token.
func (s *PortainerMCPServer) HandleKubernetesProxy() server.ToolHandlerFunc {
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

		kubernetesAPIPath, err := parser.GetString("kubernetesAPIPath", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid kubernetesAPIPath parameter", err), nil
		}
		if !strings.HasPrefix(kubernetesAPIPath, "/") {
			return mcp.NewToolResultError("kubernetesAPIPath must start with a leading slash"), nil
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

		opts := models.KubernetesProxyRequestOptions{
			EnvironmentID: environmentId,
			Path:          kubernetesAPIPath,
			Method:        method,
			QueryParams:   queryParamsMap,
			Headers:       headersMap,
		}

		if body != "" {
			opts.Body = strings.NewReader(body)
		}

		response, err := s.cli.ProxyKubernetesRequest(opts)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to send Kubernetes API request", err), nil
		}
		defer response.Body.Close()

		responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxProxyResponseSize))
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to read Kubernetes API response", err), nil
		}

		return mcp.NewToolResultText(string(responseBody)), nil
	}
}

// AddKubernetesNativeFeatures registers the Kubernetes proxy and resource management tools on the MCP server.
func (s *PortainerMCPServer) AddKubernetesNativeFeatures() {
	s.addToolIfExists(ToolGetKubernetesDashboard, s.HandleGetKubernetesDashboard())
	s.addToolIfExists(ToolListKubernetesNamespaces, s.HandleListKubernetesNamespaces())
	s.addToolIfExists(ToolGetKubernetesConfig, s.HandleGetKubernetesConfig())
}

// HandleGetKubernetesDashboard returns an MCP tool handler that retrieves kubernetes dashboard.
func (s *PortainerMCPServer) HandleGetKubernetesDashboard() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", environmentId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		dashboard, err := s.cli.GetKubernetesDashboard(environmentId)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get kubernetes dashboard", err), nil
		}

		return jsonResult(dashboard, "failed to marshal kubernetes dashboard")
	}
}

// HandleListKubernetesNamespaces returns an MCP tool handler that lists kubernetes namespaces.
func (s *PortainerMCPServer) HandleListKubernetesNamespaces() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", environmentId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		namespaces, err := s.cli.GetKubernetesNamespaces(environmentId)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get kubernetes namespaces", err), nil
		}

		return jsonResult(namespaces, "failed to marshal kubernetes namespaces")
	}
}

// HandleGetKubernetesConfig returns an MCP tool handler that retrieves kubernetes config.
func (s *PortainerMCPServer) HandleGetKubernetesConfig() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", environmentId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		config, err := s.cli.GetKubernetesConfig(environmentId)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get kubernetes config", err), nil
		}

		switch v := config.(type) {
		case string:
			return mcp.NewToolResultText(v), nil
		default:
			return jsonResult(config, "failed to marshal kubernetes config")
		}
	}
}
