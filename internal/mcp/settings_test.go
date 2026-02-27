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

// TestHandleGetSettings verifies the HandleGetSettings MCP tool handler.
func TestHandleGetSettings(t *testing.T) {
	tests := []struct {
		name          string
		settings      models.PortainerSettings
		mockError     error
		expectError   bool
		errorContains string
	}{
		{
			name: "successful settings retrieval",
			settings: models.PortainerSettings{
				Authentication: struct {
					Method string `json:"method"`
				}{
					Method: models.AuthenticationMethodInternal,
				},
				Edge: struct {
					Enabled   bool   `json:"enabled"`
					ServerURL string `json:"server_url"`
				}{
					Enabled:   true,
					ServerURL: "https://example.com",
				},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:          "client error",
			settings:      models.PortainerSettings{},
			mockError:     assert.AnError,
			expectError:   true,
			errorContains: "failed to get settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := new(MockPortainerClient)
			mockClient.On("GetSettings").Return(tt.settings, tt.mockError)

			// Create server with mock client
			srv := &PortainerMCPServer{
				srv:   server.NewMCPServer("Test Server", "1.0.0"),
				cli:   mockClient,
				tools: make(map[string]mcp.Tool),
			}

			// Get the handler
			handler := srv.HandleGetSettings()

			// Call the handler
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError, "result.IsError should be true for API errors")
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok, "Result content should be mcp.TextContent")
				if tt.errorContains != "" {
					assert.Contains(t, textContent.Text, tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var settings models.PortainerSettings
				err = json.Unmarshal([]byte(textContent.Text), &settings)
				assert.NoError(t, err)
				assert.Equal(t, tt.settings, settings)
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleUpdateSettings verifies the HandleUpdateSettings MCP tool handler.
func TestHandleUpdateSettings(t *testing.T) {
	tests := []struct {
		name          string
		request       mcp.CallToolRequest
		setupMock     func(*MockPortainerClient)
		expectError   bool
		errorContains string
	}{
		{
			name: "successful settings update",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]any{
						"settings": `{"enableEdgeComputeFeatures":true}`,
					},
				},
			},
			setupMock: func(m *MockPortainerClient) {
				m.On("UpdateSettings", map[string]interface{}{"enableEdgeComputeFeatures": true}).Return(nil)
			},
			expectError: false,
		},
		{
			name: "missing settings parameter",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]any{},
				},
			},
			setupMock:     func(m *MockPortainerClient) {},
			expectError:   true,
			errorContains: "invalid settings parameter",
		},
		{
			name: "invalid JSON in settings",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]any{
						"settings": `{invalid json}`,
					},
				},
			},
			setupMock:     func(m *MockPortainerClient) {},
			expectError:   true,
			errorContains: "failed to parse settings JSON",
		},
		{
			name: "client error",
			request: mcp.CallToolRequest{
				Params: mcp.CallToolParams{
					Arguments: map[string]any{
						"settings": `{"enableEdgeComputeFeatures":false}`,
					},
				},
			},
			setupMock: func(m *MockPortainerClient) {
				m.On("UpdateSettings", map[string]interface{}{"enableEdgeComputeFeatures": false}).Return(assert.AnError)
			},
			expectError:   true,
			errorContains: "failed to update settings",
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

			handler := srv.HandleUpdateSettings()
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
				assert.Contains(t, textContent.Text, "Settings updated successfully")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleGetPublicSettings verifies the HandleGetPublicSettings MCP tool handler.
func TestHandleGetPublicSettings(t *testing.T) {
	tests := []struct {
		name          string
		settings      models.PublicSettings
		mockError     error
		expectError   bool
		errorContains string
	}{
		{
			name: "successful public settings retrieval",
			settings: models.PublicSettings{
				AuthenticationMethod: "1",
				LogoURL:              "https://example.com/logo.png",
				Features: map[string]bool{
					"edgeCompute": true,
				},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:          "client error",
			settings:      models.PublicSettings{},
			mockError:     assert.AnError,
			expectError:   true,
			errorContains: "failed to get public settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockPortainerClient)
			mockClient.On("GetPublicSettings").Return(tt.settings, tt.mockError)

			srv := &PortainerMCPServer{
				srv:   server.NewMCPServer("Test Server", "1.0.0"),
				cli:   mockClient,
				tools: make(map[string]mcp.Tool),
			}

			handler := srv.HandleGetPublicSettings()
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

				var settings models.PublicSettings
				err = json.Unmarshal([]byte(textContent.Text), &settings)
				assert.NoError(t, err)
				assert.Equal(t, tt.settings, settings)
			}

			mockClient.AssertExpectations(t)
		})
	}
}
