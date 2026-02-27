package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddHelmFeatures registers the Helm chart and release management tools on the MCP server.
func (s *PortainerMCPServer) AddHelmFeatures() {
	s.addToolIfExists(ToolListHelmRepositories, s.HandleListHelmRepositories())
	s.addToolIfExists(ToolSearchHelmCharts, s.HandleSearchHelmCharts())
	s.addToolIfExists(ToolListHelmReleases, s.HandleListHelmReleases())
	s.addToolIfExists(ToolGetHelmReleaseHistory, s.HandleGetHelmReleaseHistory())

	if !s.readOnly {
		s.addToolIfExists(ToolAddHelmRepository, s.HandleAddHelmRepository())
		s.addToolIfExists(ToolRemoveHelmRepository, s.HandleRemoveHelmRepository())
		s.addToolIfExists(ToolInstallHelmChart, s.HandleInstallHelmChart())
		s.addToolIfExists(ToolDeleteHelmRelease, s.HandleDeleteHelmRelease())
	}
}

// HandleListHelmRepositories returns an MCP tool handler that lists helm repositories.
func (s *PortainerMCPServer) HandleListHelmRepositories() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		userId, err := parser.GetInt("userId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid userId parameter", err), nil
		}
		if err := validatePositiveID("userId", userId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		repos, err := s.cli.GetHelmRepositories(userId)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to list helm repositories", err), nil
		}

		return jsonResult(repos, "failed to marshal helm repositories")
	}
}

// HandleAddHelmRepository returns an MCP tool handler that registers helm repository.
func (s *PortainerMCPServer) HandleAddHelmRepository() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		userId, err := parser.GetInt("userId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid userId parameter", err), nil
		}
		if err := validatePositiveID("userId", userId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		url, err := parser.GetString("url", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid url parameter", err), nil
		}

		if err := validateURL(url); err != nil {
			return mcp.NewToolResultErrorFromErr("invalid repository URL", err), nil
		}

		repo, err := s.cli.CreateHelmRepository(userId, url)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to add helm repository", err), nil
		}

		return jsonResult(repo, "failed to marshal helm repository")
	}
}

// HandleRemoveHelmRepository returns an MCP tool handler that removes helm repository.
func (s *PortainerMCPServer) HandleRemoveHelmRepository() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		userId, err := parser.GetInt("userId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid userId parameter", err), nil
		}
		if err := validatePositiveID("userId", userId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		repositoryId, err := parser.GetInt("repositoryId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid repositoryId parameter", err), nil
		}
		if err := validatePositiveID("repositoryId", repositoryId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		err = s.cli.DeleteHelmRepository(userId, repositoryId)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to remove helm repository", err), nil
		}

		return mcp.NewToolResultText("Helm repository removed successfully"), nil
	}
}

// HandleSearchHelmCharts returns an MCP tool handler that searches for helm charts.
func (s *PortainerMCPServer) HandleSearchHelmCharts() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		repo, err := parser.GetString("repo", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid repo parameter", err), nil
		}

		if err := validateURL(repo); err != nil {
			return mcp.NewToolResultErrorFromErr("invalid repository URL", err), nil
		}

		chart, err := parser.GetString("chart", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid chart parameter", err), nil
		}

		result, err := s.cli.SearchHelmCharts(repo, chart)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to search helm charts", err), nil
		}

		return mcp.NewToolResultText(result), nil
	}
}

// HandleInstallHelmChart returns an MCP tool handler that installs helm chart.
func (s *PortainerMCPServer) HandleInstallHelmChart() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", environmentId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		chart, err := parser.GetString("chart", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid chart parameter", err), nil
		}

		name, err := parser.GetString("name", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}

		repo, err := parser.GetString("repo", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid repo parameter", err), nil
		}

		if err := validateURL(repo); err != nil {
			return mcp.NewToolResultErrorFromErr("invalid repository URL", err), nil
		}

		namespace, err := parser.GetString("namespace", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid namespace parameter", err), nil
		}

		values, err := parser.GetString("values", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid values parameter", err), nil
		}

		version, err := parser.GetString("version", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid version parameter", err), nil
		}

		release, err := s.cli.InstallHelmChart(environmentId, chart, name, namespace, repo, values, version)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to install helm chart", err), nil
		}

		return jsonResult(release, "failed to marshal helm release")
	}
}

// HandleListHelmReleases returns an MCP tool handler that lists helm releases.
func (s *PortainerMCPServer) HandleListHelmReleases() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", environmentId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		namespace, err := parser.GetString("namespace", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid namespace parameter", err), nil
		}

		filter, err := parser.GetString("filter", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid filter parameter", err), nil
		}

		selector, err := parser.GetString("selector", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid selector parameter", err), nil
		}

		releases, err := s.cli.GetHelmReleases(environmentId, namespace, filter, selector)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to list helm releases", err), nil
		}

		return jsonResult(releases, "failed to marshal helm releases")
	}
}

// HandleDeleteHelmRelease returns an MCP tool handler that deletes helm release.
func (s *PortainerMCPServer) HandleDeleteHelmRelease() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", environmentId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		release, err := parser.GetString("release", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid release parameter", err), nil
		}

		namespace, err := parser.GetString("namespace", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid namespace parameter", err), nil
		}

		err = s.cli.DeleteHelmRelease(environmentId, release, namespace)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to delete helm release", err), nil
		}

		return mcp.NewToolResultText("Helm release deleted successfully"), nil
	}
}

// HandleGetHelmReleaseHistory returns an MCP tool handler that retrieves helm release history.
func (s *PortainerMCPServer) HandleGetHelmReleaseHistory() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		environmentId, err := parser.GetInt("environmentId", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid environmentId parameter", err), nil
		}
		if err := validatePositiveID("environmentId", environmentId); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		name, err := parser.GetString("name", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid name parameter", err), nil
		}

		namespace, err := parser.GetString("namespace", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid namespace parameter", err), nil
		}

		history, err := s.cli.GetHelmReleaseHistory(environmentId, name, namespace)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get helm release history", err), nil
		}

		return jsonResult(history, "failed to marshal helm release history")
	}
}
