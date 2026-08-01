package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"

	"github.com/jmrplens/portainer-mcp-enhanced/pkg/toolgen"
)

// jsonResult marshals the given object to JSON and returns it as an MCP tool result.
func jsonResult(obj any, errMsg string) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return mcp.NewToolResultErrorFromErr(errMsg, err), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

// validateName checks that a name string is non-empty after trimming whitespace.
func validateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name cannot be empty or whitespace-only")
	}
	return nil
}

// validatePositiveID checks that an ID is a positive integer.
func validatePositiveID(name string, id int) error {
	if id <= 0 {
		return fmt.Errorf("%s must be a positive integer, got %d", name, id)
	}
	return nil
}

// validateURL checks that a string is a valid absolute URL with http or https scheme.
func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "oci" {
		return fmt.Errorf("URL must use http, https, or oci scheme, got %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL must include a host")
	}
	return nil
}

// validateComposeYAML checks that the content is valid YAML. This catches syntax
// errors before sending the file to the Portainer API, providing better error messages.
func validateComposeYAML(content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("compose file content cannot be empty")
	}
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		return fmt.Errorf("invalid YAML syntax: %w", err)
	}
	return nil
}

// parseAccessMap parses access entries from an array of objects and returns a map of ID to access level
func parseAccessMap(entries []any) (map[int]string, error) {
	accessMap := map[int]string{}

	for _, entry := range entries {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid access entry: %v", entry)
		}

		id, ok := entryMap["id"].(float64)
		if !ok {
			return nil, fmt.Errorf("invalid ID: %v", entryMap["id"])
		}

		access, ok := entryMap["access"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid access: %v", entryMap["access"])
		}

		if !isValidAccessLevel(access) {
			return nil, fmt.Errorf("invalid access level: %s", access)
		}

		accessMap[int(id)] = access
	}

	return accessMap, nil
}

// parseKeyValueMap parses a slice of map[string]any into a map[string]string,
// expecting each map to have "key" and "value" string fields.
func parseKeyValueMap(items []any) (map[string]string, error) {
	resultMap := map[string]string{}

	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid item: %v", item)
		}

		key, ok := itemMap["key"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid key: %v", itemMap["key"])
		}

		value, ok := itemMap["value"].(string)
		if !ok {
			return nil, fmt.Errorf("invalid value: %v", itemMap["value"])
		}

		resultMap[key] = value
	}

	return resultMap, nil
}

// containsPathTraversal checks whether a URL path contains ".." traversal sequences,
// decoding repeatedly to defeat double/triple URL encoding (e.g. %252e%252e).
func containsPathTraversal(path string) bool {
	prev := path
	for {
		decoded, err := url.PathUnescape(prev)
		if err != nil {
			// Treat unparseable encoding as suspicious.
			return true
		}
		if strings.Contains(decoded, "..") {
			return true
		}
		if decoded == prev {
			return false
		}
		prev = decoded
	}
}

// proxyParams holds the parsed parameters common to full proxy API requests
// (Docker and Kubernetes).
type proxyParams struct {
	environmentID int
	method        string
	apiPath       string
	queryParams   map[string]string
	headers       map[string]string
	body          string
}

// parseProxyParams extracts the common parameter set used by Docker and Kubernetes
// proxy handlers. pathParamName is the request field name for the API path
// (e.g. "dockerAPIPath" or "kubernetesAPIPath"). Returns parsed params or a tool
// error result that the handler should return immediately.
func parseProxyParams(request mcp.CallToolRequest, pathParamName string) (*proxyParams, *mcp.CallToolResult) {
	parser := toolgen.NewParameterParser(request)

	environmentID, err := parser.GetInt("environmentId", true)
	if err != nil {
		return nil, mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err)
	}
	if err := validatePositiveID("environmentId", environmentID); err != nil {
		return nil, mcp.NewToolResultError(err.Error())
	}

	method, err := parser.GetString("method", true)
	if err != nil {
		return nil, mcp.NewToolResultErrorFromErr("invalid method parameter", err)
	}
	if !isValidHTTPMethod(method) {
		return nil, mcp.NewToolResultError(fmt.Sprintf("invalid method: %s", method))
	}

	apiPath, err := parser.GetString(pathParamName, true)
	if err != nil {
		return nil, mcp.NewToolResultErrorFromErr(fmt.Sprintf("invalid %s parameter", pathParamName), err)
	}
	if !strings.HasPrefix(apiPath, "/") {
		return nil, mcp.NewToolResultError(fmt.Sprintf("%s must start with a leading slash", pathParamName))
	}
	if containsPathTraversal(apiPath) {
		return nil, mcp.NewToolResultError(fmt.Sprintf("%s must not contain path traversal sequences", pathParamName))
	}

	queryParams, err := parser.GetArrayOfObjects("queryParams", false)
	if err != nil {
		return nil, mcp.NewToolResultErrorFromErr("invalid queryParams parameter", err)
	}
	queryParamsMap, err := parseKeyValueMap(queryParams)
	if err != nil {
		return nil, mcp.NewToolResultErrorFromErr("invalid query params", err)
	}

	hdrs, err := parser.GetArrayOfObjects("headers", false)
	if err != nil {
		return nil, mcp.NewToolResultErrorFromErr("invalid headers parameter", err)
	}
	headersMap, err := parseKeyValueMap(hdrs)
	if err != nil {
		return nil, mcp.NewToolResultErrorFromErr("invalid headers", err)
	}

	body, err := parser.GetString("body", false)
	if err != nil {
		return nil, mcp.NewToolResultErrorFromErr("invalid body parameter", err)
	}

	return &proxyParams{
		environmentID: environmentID,
		method:        method,
		apiPath:       apiPath,
		queryParams:   queryParamsMap,
		headers:       headersMap,
		body:          body,
	}, nil
}

// readProxyResponse reads a proxy HTTP response body up to maxProxyResponseSize
// and returns it as an MCP tool result.
func readProxyResponse(response *http.Response, apiName string) (*mcp.CallToolResult, error) {
	defer func() { _ = response.Body.Close() }()

	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxProxyResponseSize+1))
	if err != nil {
		return mcp.NewToolResultErrorFromErr(fmt.Sprintf("failed to read %s API response", apiName), err), nil
	}
	if int64(len(responseBody)) > maxProxyResponseSize {
		return mcp.NewToolResultError(
			fmt.Sprintf("%s API response exceeds the %d byte limit", apiName, maxProxyResponseSize),
		), nil
	}

	return mcp.NewToolResultText(string(responseBody)), nil
}

// CreateMCPRequest creates a new MCP tool request with the given arguments.
// Used by test code only.
func CreateMCPRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}
