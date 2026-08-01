package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"

	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// TestHandleListHelmRepositories verifies the HandleListHelmRepositories MCP tool handler.
func TestHandleListHelmRepositories(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		mockSetup   func(*MockPortainerClient)
		expectError bool
	}{
		{
			name:   "successful list",
			params: map[string]any{"userId": float64(1)},
			mockSetup: func(m *MockPortainerClient) {
				m.On("GetHelmRepositories", 1).Return(models.HelmRepositoryList{
					GlobalRepository: "https://charts.helm.sh/stable",
					UserRepositories: []models.HelmRepository{
						{ID: 1, URL: "https://example.com/charts", UserID: 1},
					},
				}, nil)
			},
		},
		{
			name:   "api error",
			params: map[string]any{"userId": float64(1)},
			mockSetup: func(m *MockPortainerClient) {
				m.On("GetHelmRepositories", 1).Return(models.HelmRepositoryList{}, fmt.Errorf("api error"))
			},
			expectError: true,
		},
		{
			name:        "missing userId",
			params:      map[string]any{},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "negative userId",
			params:      map[string]any{"userId": float64(-1)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "zero userId",
			params:      map[string]any{"userId": float64(0)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			tt.mockSetup(mockClient)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleListHelmRepositories()
			request := CreateMCPRequest(tt.params)
			result, err := handler(context.Background(), request)

			assert.NoError(t, err)
			if tt.expectError {
				assert.True(t, result.IsError)
			} else {
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var repos models.HelmRepositoryList
				err = json.Unmarshal([]byte(textContent.Text), &repos)
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleAddHelmRepository verifies the HandleAddHelmRepository MCP tool handler.
func TestHandleAddHelmRepository(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		mockSetup   func(*MockPortainerClient)
		expectError bool
	}{
		{
			name:   "successful add",
			params: map[string]any{"userId": float64(1), "url": "https://example.com/charts"},
			mockSetup: func(m *MockPortainerClient) {
				m.On("CreateHelmRepository", 1, "https://example.com/charts").Return(
					models.HelmRepository{ID: 1, URL: "https://example.com/charts", UserID: 1}, nil)
			},
		},
		{
			name:   "api error",
			params: map[string]any{"userId": float64(1), "url": "https://example.com/charts"},
			mockSetup: func(m *MockPortainerClient) {
				m.On("CreateHelmRepository", 1, "https://example.com/charts").Return(
					models.HelmRepository{}, fmt.Errorf("api error"))
			},
			expectError: true,
		},
		{
			name:        "missing userId",
			params:      map[string]any{"url": "https://example.com/charts"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "negative userId",
			params:      map[string]any{"userId": float64(-1), "url": "https://example.com/charts"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "zero userId",
			params:      map[string]any{"userId": float64(0), "url": "https://example.com/charts"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "missing url",
			params:      map[string]any{"userId": float64(1)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "invalid url",
			params:      map[string]any{"userId": float64(1), "url": "not-a-valid-url"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			tt.mockSetup(mockClient)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleAddHelmRepository()
			request := CreateMCPRequest(tt.params)
			result, err := handler(context.Background(), request)

			assert.NoError(t, err)
			if tt.expectError {
				assert.True(t, result.IsError)
			} else {
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var repo models.HelmRepository
				err = json.Unmarshal([]byte(textContent.Text), &repo)
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleRemoveHelmRepository verifies the HandleRemoveHelmRepository MCP tool handler.
func TestHandleRemoveHelmRepository(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		mockSetup   func(*MockPortainerClient)
		expectError bool
	}{
		{
			name:   "successful remove",
			params: map[string]any{"userId": float64(1), "repositoryId": float64(2)},
			mockSetup: func(m *MockPortainerClient) {
				m.On("DeleteHelmRepository", 1, 2).Return(nil)
			},
		},
		{
			name:   "api error",
			params: map[string]any{"userId": float64(1), "repositoryId": float64(2)},
			mockSetup: func(m *MockPortainerClient) {
				m.On("DeleteHelmRepository", 1, 2).Return(fmt.Errorf("api error"))
			},
			expectError: true,
		},
		{
			name:        "missing userId",
			params:      map[string]any{"repositoryId": float64(2)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "negative userId",
			params:      map[string]any{"userId": float64(-1), "repositoryId": float64(2)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "zero userId",
			params:      map[string]any{"userId": float64(0), "repositoryId": float64(2)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "missing repositoryId",
			params:      map[string]any{"userId": float64(1)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "negative repositoryId",
			params:      map[string]any{"userId": float64(1), "repositoryId": float64(-1)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "zero repositoryId",
			params:      map[string]any{"userId": float64(1), "repositoryId": float64(0)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			tt.mockSetup(mockClient)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleRemoveHelmRepository()
			request := CreateMCPRequest(tt.params)
			result, err := handler(context.Background(), request)

			assert.NoError(t, err)
			if tt.expectError {
				assert.True(t, result.IsError)
			} else {
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
		params      map[string]any
		mockSetup   func(*MockPortainerClient)
		expectError bool
	}{
		{
			name:   "successful search",
			params: map[string]any{"repo": "https://charts.helm.sh/stable", "chart": "nginx"},
			mockSetup: func(m *MockPortainerClient) {
				m.On("SearchHelmCharts", "https://charts.helm.sh/stable", "nginx").Return(`[{"name":"nginx","version":"1.0.0"}]`, nil)
			},
		},
		{
			name:   "api error",
			params: map[string]any{"repo": "https://charts.helm.sh/stable", "chart": "nginx"},
			mockSetup: func(m *MockPortainerClient) {
				m.On("SearchHelmCharts", "https://charts.helm.sh/stable", "nginx").Return("", fmt.Errorf("api error"))
			},
			expectError: true,
		},
		{
			name:        "missing repo",
			params:      map[string]any{"chart": "nginx"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "invalid repo URL",
			params:      map[string]any{"repo": "not-a-valid-url", "chart": "nginx"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "invalid chart type",
			params:      map[string]any{"repo": "https://charts.helm.sh/stable", "chart": true},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			tt.mockSetup(mockClient)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleSearchHelmCharts()
			request := CreateMCPRequest(tt.params)
			result, err := handler(context.Background(), request)

			assert.NoError(t, err)
			if tt.expectError {
				assert.True(t, result.IsError)
			} else {
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Equal(t, `[{"name":"nginx","version":"1.0.0"}]`, textContent.Text)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleInstallHelmChart verifies the HandleInstallHelmChart MCP tool handler.
func TestHandleInstallHelmChart(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		mockSetup   func(*MockPortainerClient)
		expectError bool
	}{
		{
			name: "successful install",
			params: map[string]any{
				"environmentId": float64(1),
				"chart":         "nginx",
				"name":          "my-nginx",
				"repo":          "https://charts.helm.sh/stable",
				"namespace":     "default",
				"version":       "1.0.0",
			},
			mockSetup: func(m *MockPortainerClient) {
				m.On("InstallHelmChart", 1, "nginx", "my-nginx", "default", "https://charts.helm.sh/stable", "", "1.0.0").Return(
					models.HelmReleaseDetails{Name: "my-nginx", Namespace: "default", Version: 1, Status: "deployed"}, nil)
			},
		},
		{
			name: "api error",
			params: map[string]any{
				"environmentId": float64(1),
				"chart":         "nginx",
				"name":          "my-nginx",
				"repo":          "https://charts.helm.sh/stable",
			},
			mockSetup: func(m *MockPortainerClient) {
				m.On("InstallHelmChart", 1, "nginx", "my-nginx", "", "https://charts.helm.sh/stable", "", "").Return(
					models.HelmReleaseDetails{}, fmt.Errorf("api error"))
			},
			expectError: true,
		},
		{
			name: "missing environmentId",
			params: map[string]any{
				"chart": "nginx",
				"name":  "my-nginx",
				"repo":  "https://charts.helm.sh/stable",
			},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name: "negative environmentId",
			params: map[string]any{
				"environmentId": float64(-1),
				"chart":         "nginx",
				"name":          "my-nginx",
				"repo":          "https://charts.helm.sh/stable",
			},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name: "zero environmentId",
			params: map[string]any{
				"environmentId": float64(0),
				"chart":         "nginx",
				"name":          "my-nginx",
				"repo":          "https://charts.helm.sh/stable",
			},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name: "missing chart",
			params: map[string]any{
				"environmentId": float64(1),
				"name":          "my-nginx",
				"repo":          "https://charts.helm.sh/stable",
			},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name: "missing name",
			params: map[string]any{
				"environmentId": float64(1),
				"chart":         "nginx",
				"repo":          "https://charts.helm.sh/stable",
			},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name: "missing repo",
			params: map[string]any{
				"environmentId": float64(1),
				"chart":         "nginx",
				"name":          "my-nginx",
			},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name: "invalid repo URL",
			params: map[string]any{
				"environmentId": float64(1),
				"chart":         "nginx",
				"name":          "my-nginx",
				"repo":          "not-a-valid-url",
			},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name: "invalid namespace type",
			params: map[string]any{
				"environmentId": float64(1),
				"chart":         "nginx",
				"name":          "my-nginx",
				"repo":          "https://charts.helm.sh/stable",
				"namespace":     true,
			},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name: "invalid values type",
			params: map[string]any{
				"environmentId": float64(1),
				"chart":         "nginx",
				"name":          "my-nginx",
				"repo":          "https://charts.helm.sh/stable",
				"values":        true,
			},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name: "invalid version type",
			params: map[string]any{
				"environmentId": float64(1),
				"chart":         "nginx",
				"name":          "my-nginx",
				"repo":          "https://charts.helm.sh/stable",
				"version":       true,
			},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			tt.mockSetup(mockClient)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleInstallHelmChart()
			request := CreateMCPRequest(tt.params)
			result, err := handler(context.Background(), request)

			assert.NoError(t, err)
			if tt.expectError {
				assert.True(t, result.IsError)
			} else {
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
		name        string
		params      map[string]any
		mockSetup   func(*MockPortainerClient)
		expectError bool
	}{
		{
			name:   "successful list",
			params: map[string]any{"environmentId": float64(1), "namespace": "default"},
			mockSetup: func(m *MockPortainerClient) {
				m.On("GetHelmReleases", 1, "default", "", "").Return([]models.HelmRelease{
					{Name: "my-nginx", Namespace: "default", Revision: "1", Status: "deployed", Chart: "nginx-1.0.0"},
				}, nil)
			},
		},
		{
			name:   "api error",
			params: map[string]any{"environmentId": float64(1)},
			mockSetup: func(m *MockPortainerClient) {
				m.On("GetHelmReleases", 1, "", "", "").Return(nil, fmt.Errorf("api error"))
			},
			expectError: true,
		},
		{
			name:        "missing environmentId",
			params:      map[string]any{},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "negative environmentId",
			params:      map[string]any{"environmentId": float64(-1)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "zero environmentId",
			params:      map[string]any{"environmentId": float64(0)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "invalid namespace type",
			params:      map[string]any{"environmentId": float64(1), "namespace": true},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "invalid filter type",
			params:      map[string]any{"environmentId": float64(1), "filter": true},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "invalid selector type",
			params:      map[string]any{"environmentId": float64(1), "selector": true},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			tt.mockSetup(mockClient)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleListHelmReleases()
			request := CreateMCPRequest(tt.params)
			result, err := handler(context.Background(), request)

			assert.NoError(t, err)
			if tt.expectError {
				assert.True(t, result.IsError)
			} else {
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var releases []models.HelmRelease
				err = json.Unmarshal([]byte(textContent.Text), &releases)
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleDeleteHelmRelease verifies the HandleDeleteHelmRelease MCP tool handler.
func TestHandleDeleteHelmRelease(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		mockSetup   func(*MockPortainerClient)
		expectError bool
	}{
		{
			name:   "successful delete",
			params: map[string]any{"environmentId": float64(1), "release": "my-nginx", "namespace": "default"},
			mockSetup: func(m *MockPortainerClient) {
				m.On("DeleteHelmRelease", 1, "my-nginx", "default").Return(nil)
			},
		},
		{
			name:   "api error",
			params: map[string]any{"environmentId": float64(1), "release": "my-nginx"},
			mockSetup: func(m *MockPortainerClient) {
				m.On("DeleteHelmRelease", 1, "my-nginx", "").Return(fmt.Errorf("api error"))
			},
			expectError: true,
		},
		{
			name:        "missing environmentId",
			params:      map[string]any{"release": "my-nginx"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "negative environmentId",
			params:      map[string]any{"environmentId": float64(-1), "release": "my-nginx"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "zero environmentId",
			params:      map[string]any{"environmentId": float64(0), "release": "my-nginx"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "missing release",
			params:      map[string]any{"environmentId": float64(1)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "invalid namespace type",
			params:      map[string]any{"environmentId": float64(1), "release": "my-nginx", "namespace": true},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			tt.mockSetup(mockClient)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleDeleteHelmRelease()
			request := CreateMCPRequest(tt.params)
			result, err := handler(context.Background(), request)

			assert.NoError(t, err)
			if tt.expectError {
				assert.True(t, result.IsError)
			} else {
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
		name        string
		params      map[string]any
		mockSetup   func(*MockPortainerClient)
		expectError bool
	}{
		{
			name:   "successful history",
			params: map[string]any{"environmentId": float64(1), "name": "my-nginx", "namespace": "default"},
			mockSetup: func(m *MockPortainerClient) {
				m.On("GetHelmReleaseHistory", 1, "my-nginx", "default").Return([]models.HelmReleaseDetails{
					{Name: "my-nginx", Namespace: "default", Version: 1, Status: "deployed"},
					{Name: "my-nginx", Namespace: "default", Version: 2, Status: "deployed"},
				}, nil)
			},
		},
		{
			name:   "api error",
			params: map[string]any{"environmentId": float64(1), "name": "my-nginx"},
			mockSetup: func(m *MockPortainerClient) {
				m.On("GetHelmReleaseHistory", 1, "my-nginx", "").Return(nil, fmt.Errorf("api error"))
			},
			expectError: true,
		},
		{
			name:        "missing environmentId",
			params:      map[string]any{"name": "my-nginx"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "negative environmentId",
			params:      map[string]any{"environmentId": float64(-1), "name": "my-nginx"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "zero environmentId",
			params:      map[string]any{"environmentId": float64(0), "name": "my-nginx"},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "missing name",
			params:      map[string]any{"environmentId": float64(1)},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
		{
			name:        "invalid namespace type",
			params:      map[string]any{"environmentId": float64(1), "name": "my-nginx", "namespace": true},
			mockSetup:   func(m *MockPortainerClient) {},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			tt.mockSetup(mockClient)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleGetHelmReleaseHistory()
			request := CreateMCPRequest(tt.params)
			result, err := handler(context.Background(), request)

			assert.NoError(t, err)
			if tt.expectError {
				assert.True(t, result.IsError)
			} else {
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var history []models.HelmReleaseDetails
				err = json.Unmarshal([]byte(textContent.Text), &history)
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}
