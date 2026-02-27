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

// TestHandleGetSystemStatus verifies the HandleGetSystemStatus MCP tool handler.
func TestHandleGetSystemStatus(t *testing.T) {
	tests := []struct {
		name        string
		mockStatus  models.SystemStatus
		mockError   error
		expectError bool
	}{
		{
			name: "successful status retrieval",
			mockStatus: models.SystemStatus{
				Version:    "2.24.1",
				InstanceID: "abc-123-def",
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "api error",
			mockStatus:  models.SystemStatus{},
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetSystemStatus").Return(tt.mockStatus, tt.mockError)

			server := &PortainerMCPServer{
				cli: mockClient,
			}

			handler := server.HandleGetSystemStatus()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError, "result.IsError should be true for expected errors")
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok, "Result content should be mcp.TextContent for errors")
				assert.Contains(t, textContent.Text, tt.mockError.Error())
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var status models.SystemStatus
				err = json.Unmarshal([]byte(textContent.Text), &status)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockStatus, status)
			}

			mockClient.AssertExpectations(t)
		})
	}
}
