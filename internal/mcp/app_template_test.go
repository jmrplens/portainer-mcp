package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
)

// TestHandleListAppTemplates verifies the HandleListAppTemplates MCP tool handler.
func TestHandleListAppTemplates(t *testing.T) {
	tests := []struct {
		name          string
		templates     []models.AppTemplate
		mockError     error
		expectError   bool
		errorContains string
	}{
		{
			name: "successful app templates retrieval",
			templates: []models.AppTemplate{
				{
					ID:          1,
					Title:       "Nginx",
					Description: "High performance web server",
					Categories:  []string{"webserver"},
					Platform:    "linux",
					Logo:        "https://example.com/nginx.png",
					Image:       "nginx:latest",
					Type:        1,
				},
				{
					ID:          2,
					Title:       "Redis",
					Description: "In-memory data store",
					Categories:  []string{"database", "cache"},
					Platform:    "linux",
					Logo:        "https://example.com/redis.png",
					Image:       "redis:latest",
					Type:        1,
				},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:          "client error",
			templates:     nil,
			mockError:     assert.AnError,
			expectError:   true,
			errorContains: "failed to list app templates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockPortainerClient)
			mockClient.On("GetAppTemplates").Return(tt.templates, tt.mockError)

			srv := &PortainerMCPServer{
				srv:   server.NewMCPServer("Test Server", "1.0.0"),
				cli:   mockClient,
				tools: make(map[string]mcp.Tool),
			}

			handler := srv.HandleListAppTemplates()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var templates []models.AppTemplate
				err = json.Unmarshal([]byte(textContent.Text), &templates)
				assert.NoError(t, err)
				assert.Equal(t, tt.templates, templates)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleGetAppTemplateFile verifies the HandleGetAppTemplateFile MCP tool handler.
func TestHandleGetAppTemplateFile(t *testing.T) {
	tests := []struct {
		name          string
		request       mcp.CallToolRequest
		setupMock     func(*MockPortainerClient)
		expectError   bool
		errorContains string
		expectedText  string
	}{
		{
			name: "successful app template file retrieval",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]any{
						"id": float64(1),
					},
				},
			},
			setupMock: func(m *MockPortainerClient) {
				m.On("GetAppTemplateFile", 1).Return("version: '3'\nservices:\n  web:\n    image: nginx", nil)
			},
			expectError:  false,
			expectedText: "version: '3'\nservices:\n  web:\n    image: nginx",
		},
		{
			name: "missing id parameter",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]any{},
				},
			},
			setupMock:     func(m *MockPortainerClient) {},
			expectError:   true,
			errorContains: "invalid id parameter",
		},
		{
			name: "client error",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]any{
						"id": float64(99),
					},
				},
			},
			setupMock: func(m *MockPortainerClient) {
				m.On("GetAppTemplateFile", 99).Return("", assert.AnError)
			},
			expectError:   true,
			errorContains: "failed to get app template file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockPortainerClient)
			tt.setupMock(mockClient)

			srv := &PortainerMCPServer{
				srv:   server.NewMCPServer("Test Server", "1.0.0"),
				cli:   mockClient,
				tools: make(map[string]mcp.Tool),
			}

			handler := srv.HandleGetAppTemplateFile()
			result, err := handler(context.Background(), tt.request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, tt.errorContains)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Equal(t, tt.expectedText, textContent.Text)
			}

			mockClient.AssertExpectations(t)
		})
	}
}
