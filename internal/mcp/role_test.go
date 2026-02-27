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

// TestHandleListRoles verifies the HandleListRoles MCP tool handler.
func TestHandleListRoles(t *testing.T) {
	tests := []struct {
		name        string
		mockRoles   []models.Role
		mockError   error
		expectError bool
	}{
		{
			name: "successful roles retrieval",
			mockRoles: []models.Role{
				{
					ID:          1,
					Name:        "admin",
					Description: "Administrator role",
					Priority:    1,
					Authorizations: map[string]bool{
						"OperationDockerContainerArchiveInfo": true,
					},
				},
				{
					ID:          2,
					Name:        "user",
					Description: "Standard user role",
					Priority:    2,
				},
			},
			expectError: false,
		},
		{
			name:        "api error",
			mockRoles:   nil,
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetRoles").Return(tt.mockRoles, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleListRoles()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, tt.mockError.Error())
			} else {
				assert.NoError(t, err)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var roles []models.Role
				err = json.Unmarshal([]byte(textContent.Text), &roles)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockRoles, roles)
			}

			mockClient.AssertExpectations(t)
		})
	}
}
