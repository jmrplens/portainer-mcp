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

// TestHandleListCustomTemplates verifies the HandleListCustomTemplates MCP tool handler.
func TestHandleListCustomTemplates(t *testing.T) {
	tests := []struct {
		name          string
		mockTemplates []models.CustomTemplate
		mockError     error
		expectError   bool
	}{
		{
			name: "successful retrieval",
			mockTemplates: []models.CustomTemplate{
				{ID: 1, Title: "Template 1", Description: "Desc 1", Platform: 1, Type: 2},
				{ID: 2, Title: "Template 2", Description: "Desc 2", Platform: 2, Type: 1},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:          "api error",
			mockTemplates: nil,
			mockError:     fmt.Errorf("api error"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetCustomTemplates").Return(tt.mockTemplates, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}

			handler := server.HandleListCustomTemplates()
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

				var templates []models.CustomTemplate
				err = json.Unmarshal([]byte(textContent.Text), &templates)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockTemplates, templates)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleGetCustomTemplate verifies the HandleGetCustomTemplate MCP tool handler.
func TestHandleGetCustomTemplate(t *testing.T) {
	tests := []struct {
		name         string
		inputID      int
		mockTemplate models.CustomTemplate
		mockError    error
		expectError  bool
		setupParams  func(request *mcp.CallToolRequest)
	}{
		{
			name:    "successful retrieval",
			inputID: 1,
			mockTemplate: models.CustomTemplate{
				ID: 1, Title: "Template 1", Description: "Desc", Platform: 1, Type: 2,
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
				mockClient.On("GetCustomTemplate", tt.inputID).Return(tt.mockTemplate, tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleGetCustomTemplate()
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

				var template models.CustomTemplate
				err = json.Unmarshal([]byte(textContent.Text), &template)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockTemplate, template)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleGetCustomTemplateFile verifies the HandleGetCustomTemplateFile MCP tool handler.
func TestHandleGetCustomTemplateFile(t *testing.T) {
	tests := []struct {
		name        string
		inputID     int
		mockContent string
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:        "successful retrieval",
			inputID:     1,
			mockContent: "version: '3'\nservices:\n  web:\n    image: nginx",
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
				mockClient.On("GetCustomTemplateFile", tt.inputID).Return(tt.mockContent, tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleGetCustomTemplateFile()
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
				assert.Equal(t, tt.mockContent, textContent.Text)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleCreateCustomTemplate verifies the HandleCreateCustomTemplate MCP tool handler.
func TestHandleCreateCustomTemplate(t *testing.T) {
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
					"title":       "My Template",
					"description": "A template",
					"fileContent": "version: '3'",
					"type":        float64(2),
					"platform":    float64(1),
					"note":        "A note",
					"logo":        "https://example.com/logo.png",
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
					"title":       "Fail",
					"description": "Fail",
					"fileContent": "content",
					"type":        float64(2),
					"platform":    float64(1),
				}
			},
		},
		{
			name:        "missing title parameter",
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"description": "A template",
					"fileContent": "content",
					"type":        float64(2),
					"platform":    float64(1),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if tt.name == "successful creation" {
				mockClient.On("CreateCustomTemplate", "My Template", "A template", "A note", "https://example.com/logo.png", "version: '3'", 1, 2).Return(tt.mockID, tt.mockError)
			} else if tt.name == "api error" {
				mockClient.On("CreateCustomTemplate", "Fail", "Fail", "", "", "content", 1, 2).Return(tt.mockID, tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleCreateCustomTemplate()
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
				assert.Contains(t, textContent.Text, fmt.Sprintf("ID: %d", tt.mockID))
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleDeleteCustomTemplate verifies the HandleDeleteCustomTemplate MCP tool handler.
func TestHandleDeleteCustomTemplate(t *testing.T) {
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
				mockClient.On("DeleteCustomTemplate", tt.inputID).Return(tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleDeleteCustomTemplate()
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
