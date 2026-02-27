package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddStackFeatures registers the stack management tools on the MCP server.
func (s *PortainerMCPServer) AddStackFeatures() {
	s.addToolIfExists(ToolListStacks, s.HandleGetStacks())
	s.addToolIfExists(ToolListRegularStacks, s.HandleListRegularStacks())
	s.addToolIfExists(ToolGetStackFile, s.HandleGetStackFile())
	s.addToolIfExists(ToolGetStack, s.HandleInspectStack())
	s.addToolIfExists(ToolInspectStackFile, s.HandleInspectStackFile())

	if !s.readOnly {
		s.addToolIfExists(ToolCreateStack, s.HandleCreateStack())
		s.addToolIfExists(ToolUpdateStack, s.HandleUpdateStack())
		s.addToolIfExists(ToolDeleteStack, s.HandleDeleteStack())
		s.addToolIfExists(ToolUpdateStackGit, s.HandleUpdateStackGit())
		s.addToolIfExists(ToolRedeployStackGit, s.HandleRedeployStackGit())
		s.addToolIfExists(ToolStartStack, s.HandleStartStack())
		s.addToolIfExists(ToolStopStack, s.HandleStopStack())
		s.addToolIfExists(ToolMigrateStack, s.HandleMigrateStack())
	}
}

// HandleGetStacks returns an MCP tool handler that retrieves stacks.
func (s *PortainerMCPServer) HandleGetStacks() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stacks, err := s.cli.GetStacks()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get stacks", err), nil
		}

		return jsonResult(stacks, "failed to marshal stacks")
	}
}

// HandleListRegularStacks returns an MCP tool handler that lists regular stacks.
func (s *PortainerMCPServer) HandleListRegularStacks() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stacks, err := s.cli.GetRegularStacks()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to list regular stacks", err), nil
		}

		return jsonResult(stacks, "failed to marshal regular stacks")
	}
}

// HandleGetStackFile returns an MCP tool handler that retrieves stack file.
func (s *PortainerMCPServer) HandleGetStackFile() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		stackFile, err := s.cli.GetStackFile(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get stack file", err), nil
		}

		return mcp.NewToolResultText(stackFile), nil
	}
}

// HandleCreateStack returns an MCP tool handler that creates stack.
func (s *PortainerMCPServer) HandleCreateStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		name, err := parser.GetString("name", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}
		if err := validateName(name); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		file, err := parser.GetString("file", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid file parameter", err), nil
		}
		if err := validateComposeYAML(file); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		environmentGroupIds, err := parser.GetArrayOfIntegers("environmentGroupIds", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentGroupIds parameter", err), nil
		}

		id, err := s.cli.CreateStack(name, file, environmentGroupIds)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("error creating stack", err), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Stack created successfully with ID: %d", id)), nil
	}
}

// HandleUpdateStack returns an MCP tool handler that updates stack.
func (s *PortainerMCPServer) HandleUpdateStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		file, err := parser.GetString("file", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid file parameter", err), nil
		}
		if err := validateComposeYAML(file); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		environmentGroupIds, err := parser.GetArrayOfIntegers("environmentGroupIds", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentGroupIds parameter", err), nil
		}

		err = s.cli.UpdateStack(id, file, environmentGroupIds)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update stack", err), nil
		}

		return mcp.NewToolResultText("Stack updated successfully"), nil
	}
}

// HandleInspectStack returns an MCP tool handler that retrieves detailed information about stack.
func (s *PortainerMCPServer) HandleInspectStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		stack, err := s.cli.InspectStack(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to inspect stack", err), nil
		}

		return jsonResult(stack, "failed to marshal stack")
	}
}

// HandleDeleteStack returns an MCP tool handler that deletes stack.
func (s *PortainerMCPServer) HandleDeleteStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", endpointID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		removeVolumes, err := parser.GetBoolean("removeVolumes", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid removeVolumes parameter", err), nil
		}

		err = s.cli.DeleteStack(id, endpointID, removeVolumes)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to delete stack", err), nil
		}

		return mcp.NewToolResultText("Stack deleted successfully"), nil
	}
}

// HandleInspectStackFile returns an MCP tool handler that retrieves detailed information about stack file.
func (s *PortainerMCPServer) HandleInspectStackFile() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		content, err := s.cli.InspectStackFile(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to inspect stack file", err), nil
		}

		return mcp.NewToolResultText(content), nil
	}
}

// HandleUpdateStackGit returns an MCP tool handler that updates stack git.
func (s *PortainerMCPServer) HandleUpdateStackGit() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", endpointID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		referenceName, err := parser.GetString("referenceName", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid referenceName parameter", err), nil
		}

		prune, err := parser.GetBoolean("prune", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid prune parameter", err), nil
		}

		stack, err := s.cli.UpdateStackGit(id, endpointID, referenceName, prune)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update stack git", err), nil
		}

		return jsonResult(stack, "failed to marshal stack")
	}
}

// HandleRedeployStackGit returns an MCP tool handler that redeploys stack git.
func (s *PortainerMCPServer) HandleRedeployStackGit() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", endpointID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pullImage, err := parser.GetBoolean("pullImage", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid pullImage parameter", err), nil
		}

		prune, err := parser.GetBoolean("prune", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid prune parameter", err), nil
		}

		stack, err := s.cli.RedeployStackGit(id, endpointID, pullImage, prune)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to redeploy stack", err), nil
		}

		return jsonResult(stack, "failed to marshal stack")
	}
}

// HandleStartStack returns an MCP tool handler that starts stack.
func (s *PortainerMCPServer) HandleStartStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", endpointID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		stack, err := s.cli.StartStack(id, endpointID)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to start stack", err), nil
		}

		return jsonResult(stack, "failed to marshal stack")
	}
}

// HandleStopStack returns an MCP tool handler that stops stack.
func (s *PortainerMCPServer) HandleStopStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", endpointID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		stack, err := s.cli.StopStack(id, endpointID)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to stop stack", err), nil
		}

		return jsonResult(stack, "failed to marshal stack")
	}
}

// HandleMigrateStack returns an MCP tool handler that migrates stack.
func (s *PortainerMCPServer) HandleMigrateStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", endpointID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		targetEndpointID, err := parser.GetInt("targetEnvironmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid targetEnvironmentId parameter", err), nil
		}
		if err := validatePositiveID("targetEnvironmentId", targetEndpointID); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		name, err := parser.GetString("name", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}

		stack, err := s.cli.MigrateStack(id, endpointID, targetEndpointID, name)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to migrate stack", err), nil
		}

		return jsonResult(stack, "failed to marshal stack")
	}
}
