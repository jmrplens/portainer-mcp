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

// TestHandleListRegistries verifies the HandleListRegistries MCP tool handler.
func TestHandleListRegistries(t *testing.T) {
	tests := []struct {
		name           string
		mockRegistries []models.Registry
		mockError      error
		expectError    bool
	}{
		{
			name: "successful retrieval",
			mockRegistries: []models.Registry{
				{ID: 1, Name: "DockerHub", Type: 6, URL: "docker.io", Authentication: true, Username: "user1"},
				{ID: 2, Name: "Custom", Type: 3, URL: "registry.example.com", Authentication: false},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:           "api error",
			mockRegistries: nil,
			mockError:      fmt.Errorf("api error"),
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetRegistries").Return(tt.mockRegistries, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}

			handler := server.HandleListRegistries()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, tt.mockError.Error())
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var registries []models.Registry
				err = json.Unmarshal([]byte(textContent.Text), &registries)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockRegistries, registries)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleGetRegistry verifies the HandleGetRegistry MCP tool handler.
func TestHandleGetRegistry(t *testing.T) {
	tests := []struct {
		name         string
		inputID      int
		mockRegistry models.Registry
		mockError    error
		expectError  bool
		setupParams  func(request *mcp.CallToolRequest)
	}{
		{
			name:    "successful retrieval",
			inputID: 1,
			mockRegistry: models.Registry{
				ID: 1, Name: "DockerHub", Type: 6, URL: "docker.io", Authentication: true, Username: "user1",
			},
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{"id": float64(1)}
			},
		},
		{
			name:        "api error",
			inputID:     1,
			mockError:   fmt.Errorf("not found"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{"id": float64(1)}
			},
		},
		{
			name:        "missing id parameter",
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if tt.mockError != nil || tt.inputID != 0 && !tt.expectError {
				mockClient.On("GetRegistry", tt.inputID).Return(tt.mockRegistry, tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleGetRegistry()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var registry models.Registry
				err = json.Unmarshal([]byte(textContent.Text), &registry)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockRegistry, registry)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleCreateRegistry verifies the HandleCreateRegistry MCP tool handler.
func TestHandleCreateRegistry(t *testing.T) {
	tests := []struct {
		name        string
		mockID      int
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:        "successful creation",
			mockID:      42,
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"name":           "DockerHub",
					"type":           float64(6),
					"url":            "docker.io",
					"authentication": true,
					"username":       "user1",
					"password":       "pass1",
					"baseURL":        "",
				}
			},
		},
		{
			name:        "api error",
			mockID:      0,
			mockError:   fmt.Errorf("api error"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"name":           "Fail",
					"type":           float64(3),
					"url":            "fail.example.com",
					"authentication": false,
				}
			},
		},
		{
			name:        "missing name parameter",
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"type":           float64(3),
					"url":            "registry.example.com",
					"authentication": false,
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if tt.name == "successful creation" {
				mockClient.On("CreateRegistry", "DockerHub", 6, "docker.io", true, "user1", "pass1", "").Return(tt.mockID, tt.mockError)
			} else if tt.name == "api error" {
				mockClient.On("CreateRegistry", "Fail", 3, "fail.example.com", false, "", "", "").Return(tt.mockID, tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleCreateRegistry()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, "42")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleUpdateRegistry verifies the HandleUpdateRegistry MCP tool handler.
func TestHandleUpdateRegistry(t *testing.T) {
	tests := []struct {
		name        string
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
		verifyMock  func(t *testing.T, mockClient *MockPortainerClient)
	}{
		{
			name:        "successful update with all fields",
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id":             float64(1),
					"name":           "Updated",
					"url":            "new.example.com",
					"authentication": true,
					"username":       "newuser",
					"password":       "newpass",
					"baseURL":        "https://api.example.com",
				}
			},
			verifyMock: func(t *testing.T, mockClient *MockPortainerClient) {
				name := "Updated"
				url := "new.example.com"
				auth := true
				username := "newuser"
				password := "newpass"
				baseURL := "https://api.example.com"
				mockClient.On("UpdateRegistry", 1, &name, &url, &auth, &username, &password, &baseURL).Return(nil)
			},
		},
		{
			name:        "successful update with partial fields",
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id":   float64(2),
					"name": "NewName",
				}
			},
			verifyMock: func(t *testing.T, mockClient *MockPortainerClient) {
				name := "NewName"
				mockClient.On("UpdateRegistry", 2, &name, (*string)(nil), (*bool)(nil), (*string)(nil), (*string)(nil), (*string)(nil)).Return(nil)
			},
		},
		{
			name:        "api error",
			mockError:   fmt.Errorf("api error"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id":   float64(1),
					"name": "Fail",
				}
			},
			verifyMock: func(t *testing.T, mockClient *MockPortainerClient) {
				name := "Fail"
				mockClient.On("UpdateRegistry", 1, &name, (*string)(nil), (*bool)(nil), (*string)(nil), (*string)(nil), (*string)(nil)).Return(fmt.Errorf("api error"))
			},
		},
		{
			name:        "missing id parameter",
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"name": "NoID",
				}
			},
			verifyMock: func(t *testing.T, mockClient *MockPortainerClient) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			tt.verifyMock(t, mockClient)

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleUpdateRegistry()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
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

// TestHandleDeleteRegistry verifies the HandleDeleteRegistry MCP tool handler.
func TestHandleDeleteRegistry(t *testing.T) {
	tests := []struct {
		name        string
		inputID     int
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:        "successful deletion",
			inputID:     1,
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{"id": float64(1)}
			},
		},
		{
			name:        "api error",
			inputID:     1,
			mockError:   fmt.Errorf("api error"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{"id": float64(1)}
			},
		},
		{
			name:        "missing id parameter",
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if !tt.expectError || tt.mockError != nil {
				mockClient.On("DeleteRegistry", tt.inputID).Return(tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleDeleteRegistry()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.mockError != nil {
					textContent, ok := result.Content[0].(mcp.TextContent)
					assert.True(t, ok)
					assert.Contains(t, textContent.Text, tt.mockError.Error())
				}
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
