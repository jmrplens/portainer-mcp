package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddRegistryFeatures registers the Docker registry management tools on the MCP server.
func (s *PortainerMCPServer) AddRegistryFeatures() {
	s.addToolIfExists(ToolListRegistries, s.HandleListRegistries())
	s.addToolIfExists(ToolGetRegistry, s.HandleGetRegistry())

	if !s.readOnly {
		s.addToolIfExists(ToolCreateRegistry, s.HandleCreateRegistry())
		s.addToolIfExists(ToolUpdateRegistry, s.HandleUpdateRegistry())
		s.addToolIfExists(ToolDeleteRegistry, s.HandleDeleteRegistry())
	}
}

// HandleListRegistries returns an MCP tool handler that lists registries.
func (s *PortainerMCPServer) HandleListRegistries() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		registries, err := s.cli.GetRegistries()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to list registries", err), nil
		}

		return jsonResult(registries, "failed to marshal registries")
	}
}

// HandleGetRegistry returns an MCP tool handler that retrieves registry.
func (s *PortainerMCPServer) HandleGetRegistry() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		registry, err := s.cli.GetRegistry(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get registry", err), nil
		}

		return jsonResult(registry, "failed to marshal registry")
	}
}

// HandleCreateRegistry returns an MCP tool handler that creates registry.
func (s *PortainerMCPServer) HandleCreateRegistry() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		name, err := parser.GetString("name", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}

		registryType, err := parser.GetInt("type", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid type parameter", err), nil
		}

		if !isValidRegistryType(registryType) {
			return mcp.NewToolResultError("invalid registry type: must be 1-7 (1=Quay.io 2=Azure 3=Custom 4=GitLab 5=ProGet 6=DockerHub 7=ECR)"), nil
		}

		url, err := parser.GetString("url", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid url parameter", err), nil
		}

		// Registry URLs like "docker.io" may not have a scheme; only validate if scheme is present
		if strings.Contains(url, "://") {
			if err := validateURL(url); err != nil {
				return mcp.NewToolResultErrorFromErr("invalid registry URL", err), nil
			}
		}

		authentication, err := parser.GetBoolean("authentication", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid authentication parameter", err), nil
		}

		username, _ := parser.GetString("username", false)
		password, _ := parser.GetString("password", false)
		baseURL, _ := parser.GetString("baseURL", false)

		id, err := s.cli.CreateRegistry(name, registryType, url, authentication, username, password, baseURL)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to create registry", err), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Registry created successfully with ID: %d", id)), nil
	}
}

// HandleUpdateRegistry returns an MCP tool handler that updates registry.
func (s *PortainerMCPServer) HandleUpdateRegistry() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		args := request.GetArguments()

		var name *string
		if _, ok := args["name"]; ok {
			v, err := parser.GetString("name", false)
			if err != nil {
				return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
			}
			name = &v
		}

		var url *string
		if _, ok := args["url"]; ok {
			v, err := parser.GetString("url", false)
			if err != nil {
				return mcp.NewToolResultErrorFromErr("invalid url parameter", err), nil
			}
			if strings.Contains(v, "://") {
				if err := validateURL(v); err != nil {
					return mcp.NewToolResultErrorFromErr("invalid registry URL", err), nil
				}
			}
			url = &v
		}

		var authentication *bool
		if _, ok := args["authentication"]; ok {
			v, err := parser.GetBoolean("authentication", false)
			if err != nil {
				return mcp.NewToolResultErrorFromErr("invalid authentication parameter", err), nil
			}
			authentication = &v
		}

		var username *string
		if _, ok := args["username"]; ok {
			v, err := parser.GetString("username", false)
			if err != nil {
				return mcp.NewToolResultErrorFromErr("invalid username parameter", err), nil
			}
			username = &v
		}

		var password *string
		if _, ok := args["password"]; ok {
			v, err := parser.GetString("password", false)
			if err != nil {
				return mcp.NewToolResultErrorFromErr("invalid password parameter", err), nil
			}
			password = &v
		}

		var baseURL *string
		if _, ok := args["baseURL"]; ok {
			v, err := parser.GetString("baseURL", false)
			if err != nil {
				return mcp.NewToolResultErrorFromErr("invalid baseURL parameter", err), nil
			}
			baseURL = &v
		}

		err = s.cli.UpdateRegistry(id, name, url, authentication, username, password, baseURL)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update registry", err), nil
		}

		return mcp.NewToolResultText("Registry updated successfully"), nil
	}
}

// HandleDeleteRegistry returns an MCP tool handler that deletes registry.
func (s *PortainerMCPServer) HandleDeleteRegistry() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = s.cli.DeleteRegistry(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to delete registry", err), nil
		}

		return mcp.NewToolResultText("Registry deleted successfully"), nil
	}
}
