package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

func (s *PortainerMCPServer) AddStackFeatures() {
	s.addToolIfExists(ToolListStacks, s.HandleGetStacks())
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

func (s *PortainerMCPServer) HandleGetStacks() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stacks, err := s.cli.GetStacks()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get stacks", err), nil
		}

		data, err := json.Marshal(stacks)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to marshal stacks", err), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func (s *PortainerMCPServer) HandleGetStackFile() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		stackFile, err := s.cli.GetStackFile(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get stack file", err), nil
		}

		return mcp.NewToolResultText(stackFile), nil
	}
}

func (s *PortainerMCPServer) HandleCreateStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		name, err := parser.GetString("name", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}

		file, err := parser.GetString("file", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid file parameter", err), nil
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

func (s *PortainerMCPServer) HandleUpdateStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		file, err := parser.GetString("file", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid file parameter", err), nil
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

func (s *PortainerMCPServer) HandleInspectStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		stack, err := s.cli.InspectStack(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to inspect stack", err), nil
		}

		data, err := json.Marshal(stack)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to marshal stack", err), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func (s *PortainerMCPServer) HandleDeleteStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
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

func (s *PortainerMCPServer) HandleInspectStackFile() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		content, err := s.cli.InspectStackFile(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to inspect stack file", err), nil
		}

		return mcp.NewToolResultText(content), nil
	}
}

func (s *PortainerMCPServer) HandleUpdateStackGit() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
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

		data, err := json.Marshal(stack)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to marshal stack", err), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func (s *PortainerMCPServer) HandleRedeployStackGit() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
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

		data, err := json.Marshal(stack)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to marshal stack", err), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func (s *PortainerMCPServer) HandleStartStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}

		stack, err := s.cli.StartStack(id, endpointID)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to start stack", err), nil
		}

		data, err := json.Marshal(stack)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to marshal stack", err), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func (s *PortainerMCPServer) HandleStopStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}

		stack, err := s.cli.StopStack(id, endpointID)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to stop stack", err), nil
		}

		data, err := json.Marshal(stack)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to marshal stack", err), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

func (s *PortainerMCPServer) HandleMigrateStack() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}

		endpointID, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}

		targetEndpointID, err := parser.GetInt("targetEnvironmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid targetEnvironmentId parameter", err), nil
		}

		name, err := parser.GetString("name", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}

		stack, err := s.cli.MigrateStack(id, endpointID, targetEndpointID, name)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to migrate stack", err), nil
		}

		data, err := json.Marshal(stack)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to marshal stack", err), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}