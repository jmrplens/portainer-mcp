package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"gopkg.in/yaml.v3"
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

// CreateMCPRequest creates a new MCP tool request with the given arguments.
// Used by test code only.
func CreateMCPRequest(args map[string]any) mcp.CallToolRequest {
	return mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Arguments: args,
		},
	}
}
