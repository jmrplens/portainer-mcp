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

// TestHandleListWebhooks verifies the HandleListWebhooks MCP tool handler.
func TestHandleListWebhooks(t *testing.T) {
	tests := []struct {
		name         string
		mockWebhooks []models.Webhook
		mockError    error
		expectError  bool
	}{
		{
			name: "successful webhooks retrieval",
			mockWebhooks: []models.Webhook{
				{ID: 1, EndpointID: 2, ResourceID: "svc1", Token: "abc", Type: 1},
				{ID: 2, EndpointID: 3, ResourceID: "svc2", Token: "def", Type: 1},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:         "api error",
			mockWebhooks: nil,
			mockError:    fmt.Errorf("api error"),
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetWebhooks").Return(tt.mockWebhooks, tt.mockError)

			server := &PortainerMCPServer{
				cli: mockClient,
			}

			handler := server.HandleListWebhooks()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError, "result.IsError should be true for API errors")
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok, "Result content should be mcp.TextContent")
				if tt.mockError != nil {
					assert.Contains(t, textContent.Text, tt.mockError.Error())
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var webhooks []models.Webhook
				err = json.Unmarshal([]byte(textContent.Text), &webhooks)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockWebhooks, webhooks)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleCreateWebhook verifies the HandleCreateWebhook MCP tool handler.
func TestHandleCreateWebhook(t *testing.T) {
	tests := []struct {
		name        string
		params      map[string]any
		mockID      int
		mockError   error
		expectError bool
	}{
		{
			name: "successful webhook creation",
			params: map[string]any{
				"resourceId":  "svc1",
				"endpointId":  float64(2),
				"webhookType": float64(1),
			},
			mockID:      42,
			mockError:   nil,
			expectError: false,
		},
		{
			name: "api error",
			params: map[string]any{
				"resourceId":  "svc1",
				"endpointId":  float64(2),
				"webhookType": float64(1),
			},
			mockID:      0,
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
		{
			name:        "missing resourceId parameter",
			params:      map[string]any{},
			mockID:      0,
			mockError:   nil,
			expectError: true,
		},
		{
			name: "missing endpointId parameter",
			params: map[string]any{
				"resourceId": "svc1",
			},
			mockID:      0,
			mockError:   nil,
			expectError: true,
		},
		{
			name: "missing webhookType parameter",
			params: map[string]any{
				"resourceId": "svc1",
				"endpointId": float64(2),
			},
			mockID:      0,
			mockError:   nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if resourceId, ok := tt.params["resourceId"]; ok {
				if endpointId, ok2 := tt.params["endpointId"]; ok2 {
					if webhookType, ok3 := tt.params["webhookType"]; ok3 {
						mockClient.On("CreateWebhook",
							resourceId.(string),
							int(endpointId.(float64)),
							int(webhookType.(float64)),
						).Return(tt.mockID, tt.mockError)
					}
				}
			}

			server := &PortainerMCPServer{
				cli: mockClient,
			}

			request := CreateMCPRequest(tt.params)

			handler := server.HandleCreateWebhook()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError, "result.IsError should be true for expected errors")
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok, "Result content should be mcp.TextContent for errors")
				if tt.mockError != nil {
					assert.Contains(t, textContent.Text, tt.mockError.Error())
				} else {
					assert.NotEmpty(t, textContent.Text, "Error message should not be empty for parameter errors")
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, "successfully")
				assert.Contains(t, textContent.Text, "42")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleDeleteWebhook verifies the HandleDeleteWebhook MCP tool handler.
func TestHandleDeleteWebhook(t *testing.T) {
	tests := []struct {
		name        string
		inputID     int
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:        "successful webhook deletion",
			inputID:     1,
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id": float64(1),
				}
			},
		},
		{
			name:        "api error",
			inputID:     1,
			mockError:   fmt.Errorf("api error"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id": float64(1),
				}
			},
		},
		{
			name:        "missing id parameter",
			inputID:     0,
			mockError:   nil,
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				// No parameters
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if !tt.expectError || tt.mockError != nil {
				mockClient.On("DeleteWebhook", tt.inputID).Return(tt.mockError)
			}

			server := &PortainerMCPServer{
				cli: mockClient,
			}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleDeleteWebhook()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError, "result.IsError should be true for expected errors")
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok, "Result content should be mcp.TextContent for errors")
				if tt.mockError != nil {
					assert.Contains(t, textContent.Text, tt.mockError.Error())
				} else {
					assert.NotEmpty(t, textContent.Text, "Error message should not be empty for parameter errors")
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
