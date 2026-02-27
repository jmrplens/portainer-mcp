package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddEdgeJobFeatures registers the edge job and edge update schedule management tools on the MCP server.
func (s *PortainerMCPServer) AddEdgeJobFeatures() {
	s.addToolIfExists(ToolListEdgeJobs, s.HandleListEdgeJobs())
	s.addToolIfExists(ToolGetEdgeJob, s.HandleGetEdgeJob())
	s.addToolIfExists(ToolGetEdgeJobFile, s.HandleGetEdgeJobFile())

	if !s.readOnly {
		s.addToolIfExists(ToolCreateEdgeJob, s.HandleCreateEdgeJob())
		s.addToolIfExists(ToolDeleteEdgeJob, s.HandleDeleteEdgeJob())
	}
}

// HandleListEdgeJobs returns an MCP tool handler that lists edge jobs.
func (s *PortainerMCPServer) HandleListEdgeJobs() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		jobs, err := s.cli.GetEdgeJobs()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to list edge jobs", err), nil
		}

		return jsonResult(jobs, "failed to marshal edge jobs")
	}
}

// HandleGetEdgeJob returns an MCP tool handler that retrieves edge job.
func (s *PortainerMCPServer) HandleGetEdgeJob() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		job, err := s.cli.GetEdgeJob(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get edge job", err), nil
		}

		return jsonResult(job, "failed to marshal edge job")
	}
}

// HandleGetEdgeJobFile returns an MCP tool handler that retrieves edge job file.
func (s *PortainerMCPServer) HandleGetEdgeJobFile() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		content, err := s.cli.GetEdgeJobFile(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get edge job file", err), nil
		}

		return mcp.NewToolResultText(content), nil
	}
}

// HandleCreateEdgeJob returns an MCP tool handler that creates edge job.
func (s *PortainerMCPServer) HandleCreateEdgeJob() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		name, err := parser.GetString("name", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}

		cronExpression, err := parser.GetString("cronExpression", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid cronExpression parameter", err), nil
		}

		if !isValidCronExpression(cronExpression) {
			return mcp.NewToolResultErrorFromErr("invalid cronExpression parameter", fmt.Errorf("cron expression must have 5 fields (minute hour day month weekday)")), nil
		}

		fileContent, err := parser.GetString("fileContent", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid fileContent parameter", err), nil
		}

		recurring, _ := parser.GetBoolean("recurring", false)

		endpoints, _ := parser.GetArrayOfIntegers("endpoints", false)
		edgeGroups, _ := parser.GetArrayOfIntegers("edgeGroups", false)

		id, err := s.cli.CreateEdgeJob(name, cronExpression, fileContent, endpoints, edgeGroups, recurring)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to create edge job", err), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Edge job created successfully with ID: %d", id)), nil
	}
}

// HandleDeleteEdgeJob returns an MCP tool handler that deletes edge job.
func (s *PortainerMCPServer) HandleDeleteEdgeJob() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		id, err := parser.GetInt("id", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid id parameter", err), nil
		}
		if err := validatePositiveID("id", id); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = s.cli.DeleteEdgeJob(id)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to delete edge job", err), nil
		}

		return mcp.NewToolResultText("Edge job deleted successfully"), nil
	}
}

// isValidCronExpression performs basic validation of a cron expression.
// It checks that the expression has exactly 5 fields (minute hour day month weekday).
func isValidCronExpression(expr string) bool {
	fields := strings.Fields(strings.TrimSpace(expr))
	return len(fields) == 5
}

// AddEdgeUpdateScheduleFeatures registers the edge job and edge update schedule management tools on the MCP server.
func (s *PortainerMCPServer) AddEdgeUpdateScheduleFeatures() {
	s.addToolIfExists(ToolListEdgeUpdateSchedules, s.HandleListEdgeUpdateSchedules())
}

// HandleListEdgeUpdateSchedules returns an MCP tool handler that lists edge update schedules.
func (s *PortainerMCPServer) HandleListEdgeUpdateSchedules() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		schedules, err := s.cli.GetEdgeUpdateSchedules()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to list edge update schedules", err), nil
		}

		return jsonResult(schedules, "failed to marshal edge update schedules")
	}
}
