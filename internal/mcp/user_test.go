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

// TestHandleGetUsers verifies the HandleGetUsers MCP tool handler.
func TestHandleGetUsers(t *testing.T) {
	tests := []struct {
		name        string
		mockUsers   []models.User
		mockError   error
		expectError bool
	}{
		{
			name: "successful users retrieval",
			mockUsers: []models.User{
				{ID: 1, Username: "user1", Role: "admin"},
				{ID: 2, Username: "user2", Role: "user"},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "api error",
			mockUsers:   nil,
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := &MockPortainerClient{}
			mockClient.On("GetUsers").Return(tt.mockUsers, tt.mockError)

			// Create server with mock client
			server := &PortainerMCPServer{
				cli: mockClient,
			}

			// Call handler
			handler := server.HandleGetUsers()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			// Verify results
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

				var users []models.User
				err = json.Unmarshal([]byte(textContent.Text), &users)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockUsers, users)
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleUpdateUserRole verifies the HandleUpdateUserRole MCP tool handler.
func TestHandleUpdateUserRole(t *testing.T) {
	tests := []struct {
		name        string
		inputID     int
		inputRole   string
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:        "successful role update",
			inputID:     1,
			inputRole:   "admin",
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id":   float64(1),
					"role": "admin",
				}
			},
		},
		{
			name:        "api error",
			inputID:     1,
			inputRole:   "admin",
			mockError:   fmt.Errorf("api error"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id":   float64(1),
					"role": "admin",
				}
			},
		},
		{
			name:        "missing id parameter",
			inputID:     0,
			inputRole:   "admin",
			mockError:   nil,
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"role": "admin",
				}
			},
		},
		{
			name:        "missing role parameter",
			inputID:     1,
			inputRole:   "",
			mockError:   nil,
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id": float64(1),
				}
			},
		},
		{
			name:        "invalid role",
			inputID:     1,
			inputRole:   "invalid_role",
			mockError:   nil,
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id":   float64(1),
					"role": "invalid_role",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock client
			mockClient := &MockPortainerClient{}
			if !tt.expectError || tt.mockError != nil {
				mockClient.On("UpdateUserRole", tt.inputID, tt.inputRole).Return(tt.mockError)
			}

			// Create server with mock client
			server := &PortainerMCPServer{
				cli: mockClient,
			}

			// Create request with parameters
			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			// Call handler
			handler := server.HandleUpdateUserRole()
			result, err := handler(context.Background(), request)

			// Verify results
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
					assert.NotEmpty(t, textContent.Text, "Error message should not be empty for parameter/validation errors")
					if tt.inputRole == "invalid_role" {
						assert.Contains(t, textContent.Text, "invalid role")
					}
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, "successfully")
			}

			// Verify mock expectations
			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleCreateUser verifies the HandleCreateUser MCP tool handler.
func TestHandleCreateUser(t *testing.T) {
	tests := []struct {
		name        string
		username    string
		password    string
		role        string
		mockID      int
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:        "successful user creation",
			username:    "testuser",
			password:    "password123",
			role:        "user",
			mockID:      1,
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"username": "testuser",
					"password": "password123",
					"role":     "user",
				}
			},
		},
		{
			name:        "api error",
			username:    "testuser",
			password:    "password123",
			role:        "admin",
			mockID:      0,
			mockError:   fmt.Errorf("api error"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"username": "testuser",
					"password": "password123",
					"role":     "admin",
				}
			},
		},
		{
			name:        "missing username parameter",
			username:    "",
			password:    "",
			role:        "",
			mockID:      0,
			mockError:   nil,
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"password": "password123",
					"role":     "user",
				}
			},
		},
		{
			name:        "invalid role",
			username:    "testuser",
			password:    "password123",
			role:        "invalid_role",
			mockID:      0,
			mockError:   nil,
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"username": "testuser",
					"password": "password123",
					"role":     "invalid_role",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if !tt.expectError || tt.mockError != nil {
				mockClient.On("CreateUser", tt.username, tt.password, tt.role).Return(tt.mockID, tt.mockError)
			}

			server := &PortainerMCPServer{
				cli: mockClient,
			}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleCreateUser()
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
				assert.Contains(t, textContent.Text, fmt.Sprintf("ID: %d", tt.mockID))
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleGetUser verifies the HandleGetUser MCP tool handler.
func TestHandleGetUser(t *testing.T) {
	tests := []struct {
		name        string
		inputID     int
		mockUser    models.User
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:    "successful user retrieval",
			inputID: 1,
			mockUser: models.User{
				ID:       1,
				Username: "testuser",
				Role:     "admin",
			},
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
			mockUser:    models.User{},
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
			mockUser:    models.User{},
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
				mockClient.On("GetUser", tt.inputID).Return(tt.mockUser, tt.mockError)
			}

			server := &PortainerMCPServer{
				cli: mockClient,
			}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleGetUser()
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

				var user models.User
				err = json.Unmarshal([]byte(textContent.Text), &user)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockUser, user)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleDeleteUser verifies the HandleDeleteUser MCP tool handler.
func TestHandleDeleteUser(t *testing.T) {
	tests := []struct {
		name        string
		inputID     int
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:        "successful user deletion",
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
				mockClient.On("DeleteUser", tt.inputID).Return(tt.mockError)
			}

			server := &PortainerMCPServer{
				cli: mockClient,
			}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleDeleteUser()
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
