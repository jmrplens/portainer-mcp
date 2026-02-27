package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
)

// TestHandleListHelmRepositories verifies the HandleListHelmRepositories MCP tool handler.
func TestHandleListHelmRepositories(t *testing.T) {
	tests := []struct {
		name        string
		userId      int
		mockResult  models.HelmRepositoryList
		mockError   error
		expectError bool
	}{
		{
			name:   "successful list",
			userId: 1,
			mockResult: models.HelmRepositoryList{
				GlobalRepository: "https://charts.helm.sh/stable",
				UserRepositories: []models.HelmRepository{
					{ID: 1, URL: "https://example.com/charts", UserID: 1},
				},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "api error",
			userId:      1,
			mockResult:  models.HelmRepositoryList{},
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetHelmRepositories", tt.userId).Return(tt.mockResult, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleListHelmRepositories()
			request := CreateMCPRequest(map[string]any{"userId": float64(tt.userId)})
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var repos models.HelmRepositoryList
				err = json.Unmarshal([]byte(textContent.Text), &repos)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResult, repos)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleAddHelmRepository verifies the HandleAddHelmRepository MCP tool handler.
func TestHandleAddHelmRepository(t *testing.T) {
	tests := []struct {
		name        string
		userId      int
		url         string
		mockResult  models.HelmRepository
		mockError   error
		expectError bool
	}{
		{
			name:       "successful add",
			userId:     1,
			url:        "https://example.com/charts",
			mockResult: models.HelmRepository{ID: 1, URL: "https://example.com/charts", UserID: 1},
			mockError:  nil,
		},
		{
			name:        "api error",
			userId:      1,
			url:         "https://example.com/charts",
			mockResult:  models.HelmRepository{},
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("CreateHelmRepository", tt.userId, tt.url).Return(tt.mockResult, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleAddHelmRepository()
			request := CreateMCPRequest(map[string]any{"userId": float64(tt.userId), "url": tt.url})
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var repo models.HelmRepository
				err = json.Unmarshal([]byte(textContent.Text), &repo)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResult, repo)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleRemoveHelmRepository verifies the HandleRemoveHelmRepository MCP tool handler.
func TestHandleRemoveHelmRepository(t *testing.T) {
	tests := []struct {
		name         string
		userId       int
		repositoryId int
		mockError    error
		expectError  bool
	}{
		{
			name:         "successful remove",
			userId:       1,
			repositoryId: 2,
			mockError:    nil,
		},
		{
			name:         "api error",
			userId:       1,
			repositoryId: 2,
			mockError:    fmt.Errorf("api error"),
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("DeleteHelmRepository", tt.userId, tt.repositoryId).Return(tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleRemoveHelmRepository()
			request := CreateMCPRequest(map[string]any{"userId": float64(tt.userId), "repositoryId": float64(tt.repositoryId)})
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, "successfully")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleSearchHelmCharts verifies the HandleSearchHelmCharts MCP tool handler.
func TestHandleSearchHelmCharts(t *testing.T) {
	tests := []struct {
		name        string
		repo        string
		chart       string
		mockResult  string
		mockError   error
		expectError bool
	}{
		{
			name:       "successful search",
			repo:       "https://charts.helm.sh/stable",
			chart:      "nginx",
			mockResult: `[{"name":"nginx","version":"1.0.0"}]`,
			mockError:  nil,
		},
		{
			name:        "api error",
			repo:        "https://charts.helm.sh/stable",
			chart:       "nginx",
			mockResult:  "",
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("SearchHelmCharts", tt.repo, tt.chart).Return(tt.mockResult, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleSearchHelmCharts()
			request := CreateMCPRequest(map[string]any{"repo": tt.repo, "chart": tt.chart})
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Equal(t, tt.mockResult, textContent.Text)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleInstallHelmChart verifies the HandleInstallHelmChart MCP tool handler.
func TestHandleInstallHelmChart(t *testing.T) {
	tests := []struct {
		name          string
		environmentId int
		chart         string
		releaseName   string
		repo          string
		namespace     string
		values        string
		version       string
		mockResult    models.HelmReleaseDetails
		mockError     error
		expectError   bool
	}{
		{
			name:          "successful install",
			environmentId: 1,
			chart:         "nginx",
			releaseName:   "my-nginx",
			repo:          "https://charts.helm.sh/stable",
			namespace:     "default",
			values:        "",
			version:       "1.0.0",
			mockResult:    models.HelmReleaseDetails{Name: "my-nginx", Namespace: "default", Version: 1, Status: "deployed"},
			mockError:     nil,
		},
		{
			name:          "api error",
			environmentId: 1,
			chart:         "nginx",
			releaseName:   "my-nginx",
			repo:          "https://charts.helm.sh/stable",
			mockResult:    models.HelmReleaseDetails{},
			mockError:     fmt.Errorf("api error"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("InstallHelmChart", tt.environmentId, tt.chart, tt.releaseName, tt.namespace, tt.repo, tt.values, tt.version).Return(tt.mockResult, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleInstallHelmChart()
			args := map[string]any{
				"environmentId": float64(tt.environmentId),
				"chart":         tt.chart,
				"name":          tt.releaseName,
				"repo":          tt.repo,
			}
			if tt.namespace != "" {
				args["namespace"] = tt.namespace
			}
			if tt.values != "" {
				args["values"] = tt.values
			}
			if tt.version != "" {
				args["version"] = tt.version
			}
			request := CreateMCPRequest(args)
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, "my-nginx")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleListHelmReleases verifies the HandleListHelmReleases MCP tool handler.
func TestHandleListHelmReleases(t *testing.T) {
	tests := []struct {
		name          string
		environmentId int
		namespace     string
		filter        string
		selector      string
		mockResult    []models.HelmRelease
		mockError     error
		expectError   bool
	}{
		{
			name:          "successful list",
			environmentId: 1,
			namespace:     "default",
			mockResult: []models.HelmRelease{
				{Name: "my-nginx", Namespace: "default", Revision: "1", Status: "deployed", Chart: "nginx-1.0.0"},
			},
			mockError: nil,
		},
		{
			name:          "api error",
			environmentId: 1,
			mockResult:    nil,
			mockError:     fmt.Errorf("api error"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetHelmReleases", tt.environmentId, tt.namespace, tt.filter, tt.selector).Return(tt.mockResult, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleListHelmReleases()
			args := map[string]any{"environmentId": float64(tt.environmentId)}
			if tt.namespace != "" {
				args["namespace"] = tt.namespace
			}
			if tt.filter != "" {
				args["filter"] = tt.filter
			}
			if tt.selector != "" {
				args["selector"] = tt.selector
			}
			request := CreateMCPRequest(args)
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var releases []models.HelmRelease
				err = json.Unmarshal([]byte(textContent.Text), &releases)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResult, releases)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleDeleteHelmRelease verifies the HandleDeleteHelmRelease MCP tool handler.
func TestHandleDeleteHelmRelease(t *testing.T) {
	tests := []struct {
		name          string
		environmentId int
		release       string
		namespace     string
		mockError     error
		expectError   bool
	}{
		{
			name:          "successful delete",
			environmentId: 1,
			release:       "my-nginx",
			namespace:     "default",
			mockError:     nil,
		},
		{
			name:          "api error",
			environmentId: 1,
			release:       "my-nginx",
			mockError:     fmt.Errorf("api error"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("DeleteHelmRelease", tt.environmentId, tt.release, tt.namespace).Return(tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleDeleteHelmRelease()
			args := map[string]any{"environmentId": float64(tt.environmentId), "release": tt.release}
			if tt.namespace != "" {
				args["namespace"] = tt.namespace
			}
			request := CreateMCPRequest(args)
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, "successfully")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleGetHelmReleaseHistory verifies the HandleGetHelmReleaseHistory MCP tool handler.
func TestHandleGetHelmReleaseHistory(t *testing.T) {
	tests := []struct {
		name          string
		environmentId int
		releaseName   string
		namespace     string
		mockResult    []models.HelmReleaseDetails
		mockError     error
		expectError   bool
	}{
		{
			name:          "successful history",
			environmentId: 1,
			releaseName:   "my-nginx",
			namespace:     "default",
			mockResult: []models.HelmReleaseDetails{
				{Name: "my-nginx", Namespace: "default", Version: 1, Status: "deployed"},
				{Name: "my-nginx", Namespace: "default", Version: 2, Status: "deployed"},
			},
			mockError: nil,
		},
		{
			name:          "api error",
			environmentId: 1,
			releaseName:   "my-nginx",
			mockResult:    nil,
			mockError:     fmt.Errorf("api error"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetHelmReleaseHistory", tt.environmentId, tt.releaseName, tt.namespace).Return(tt.mockResult, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleGetHelmReleaseHistory()
			args := map[string]any{"environmentId": float64(tt.environmentId), "name": tt.releaseName}
			if tt.namespace != "" {
				args["namespace"] = tt.namespace
			}
			request := CreateMCPRequest(args)
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var history []models.HelmReleaseDetails
				err = json.Unmarshal([]byte(textContent.Text), &history)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResult, history)
			}

			mockClient.AssertExpectations(t)
		})
	}
}
