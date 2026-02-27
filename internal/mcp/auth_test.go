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

// TestHandleAuthenticateUser verifies the HandleAuthenticateUser MCP tool handler.
func TestHandleAuthenticateUser(t *testing.T) {
	tests := []struct {
		name        string
		setupParams func(request *mcp.CallToolRequest)
		mockResult  models.AuthResponse
		mockError   error
		expectError bool
		shouldMock  bool
	}{
		{
			name: "successful authentication",
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"username": "admin",
					"password": "password123",
				}
			},
			mockResult: models.AuthResponse{
				JWT: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			},
			expectError: false,
			shouldMock:  true,
		},
		{
			name: "missing username",
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"password": "password123",
				}
			},
			expectError: true,
			shouldMock:  false,
		},
		{
			name: "missing password",
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"username": "admin",
				}
			},
			expectError: true,
			shouldMock:  false,
		},
		{
			name: "api error",
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"username": "admin",
					"password": "wrongpassword",
				}
			},
			mockResult:  models.AuthResponse{},
			mockError:   fmt.Errorf("invalid credentials"),
			expectError: true,
			shouldMock:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}

			if tt.shouldMock {
				args := tt.setupParams
				request := mcp.CallToolRequest{}
				args(&request)
				params := request.GetArguments()
				mockClient.On("AuthenticateUser", params["username"].(string), params["password"].(string)).Return(tt.mockResult, tt.mockError)
			}

			request := mcp.CallToolRequest{}
			tt.setupParams(&request)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleAuthenticateUser()
			result, err := handler(context.Background(), request)

			assert.NoError(t, err)
			assert.NotNil(t, result)

			if tt.expectError {
				assert.True(t, result.IsError, "result.IsError should be true for expected errors")
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				if tt.mockError != nil {
					assert.Contains(t, textContent.Text, tt.mockError.Error())
				}
			} else {
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var authResp models.AuthResponse
				err = json.Unmarshal([]byte(textContent.Text), &authResp)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResult, authResp)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleLogout verifies the HandleLogout MCP tool handler.
func TestHandleLogout(t *testing.T) {
	tests := []struct {
		name        string
		mockError   error
		expectError bool
	}{
		{
			name:        "successful logout",
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "api error",
			mockError:   fmt.Errorf("session expired"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("Logout").Return(tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleLogout()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			assert.NoError(t, err)
			assert.NotNil(t, result)

			if tt.expectError {
				assert.True(t, result.IsError)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, tt.mockError.Error())
			} else {
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, "Logged out successfully")
			}

			mockClient.AssertExpectations(t)
		})
	}
}
