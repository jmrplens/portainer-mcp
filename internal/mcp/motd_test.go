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

// TestHandleGetMOTD verifies the HandleGetMOTD MCP tool handler.
func TestHandleGetMOTD(t *testing.T) {
	tests := []struct {
		name        string
		mockMOTD    models.MOTD
		mockError   error
		expectError bool
	}{
		{
			name: "successful MOTD retrieval",
			mockMOTD: models.MOTD{
				Title:   "Welcome",
				Message: "Hello World",
				Style:   "info",
				Hash:    json.RawMessage(`"/L63mbIXZxetD/T6xFz3pQ=="`),
				ContentLayout: map[string]string{
					"key": "value",
				},
			},
			expectError: false,
		},
		{
			name:        "api error",
			mockMOTD:    models.MOTD{},
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetMOTD").Return(tt.mockMOTD, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleGetMOTD()
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

				var motd models.MOTD
				err = json.Unmarshal([]byte(textContent.Text), &motd)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockMOTD, motd)
			}

			mockClient.AssertExpectations(t)
		})
	}
}
