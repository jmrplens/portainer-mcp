package client

import (
	"errors"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
)

// TestGetUsers verifies get users behavior.
func TestGetUsers(t *testing.T) {
	tests := []struct {
		name          string
		mockUsers     []*apimodels.PortainereeUser
		mockError     error
		expected      []models.User
		expectedError bool
	}{
		{
			name: "successful retrieval - all role types",
			mockUsers: []*apimodels.PortainereeUser{
				{
					ID:       1,
					Username: "admin_user",
					Role:     1, // admin
				},
				{
					ID:       2,
					Username: "regular_user",
					Role:     2, // user
				},
				{
					ID:       3,
					Username: "edge_admin_user",
					Role:     3, // edge_admin
				},
				{
					ID:       4,
					Username: "unknown_role_user",
					Role:     0, // unknown
				},
			},
			expected: []models.User{
				{
					ID:       1,
					Username: "admin_user",
					Role:     models.UserRoleAdmin,
				},
				{
					ID:       2,
					Username: "regular_user",
					Role:     models.UserRoleUser,
				},
				{
					ID:       3,
					Username: "edge_admin_user",
					Role:     models.UserRoleEdgeAdmin,
				},
				{
					ID:       4,
					Username: "unknown_role_user",
					Role:     models.UserRoleUnknown,
				},
			},
		},
		{
			name:      "empty users",
			mockUsers: []*apimodels.PortainereeUser{},
			expected:  []models.User{},
		},
		{
			name:          "list error",
			mockError:     errors.New("failed to list users"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("ListUsers").Return(tt.mockUsers, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			users, err := client.GetUsers()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, users)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestUpdateUserRole verifies update user role behavior.
func TestUpdateUserRole(t *testing.T) {
	tests := []struct {
		name          string
		userID        int
		role          string
		expectedRole  int64
		mockError     error
		expectedError bool
	}{
		{
			name:         "update to admin role",
			userID:       1,
			role:         models.UserRoleAdmin,
			expectedRole: 1,
		},
		{
			name:         "update to regular user role",
			userID:       2,
			role:         models.UserRoleUser,
			expectedRole: 2,
		},
		{
			name:         "update to edge admin role",
			userID:       3,
			role:         models.UserRoleEdgeAdmin,
			expectedRole: 3,
		},
		{
			name:          "invalid role",
			userID:        4,
			role:          "invalid_role",
			expectedError: true,
		},
		{
			name:          "update error",
			userID:        5,
			role:          models.UserRoleAdmin,
			expectedRole:  1,
			mockError:     errors.New("failed to update user role"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			if !tt.expectedError || tt.mockError != nil {
				mockAPI.On("UpdateUserRole", tt.userID, tt.expectedRole).Return(tt.mockError)
			}

			client := &PortainerClient{cli: mockAPI}

			err := client.UpdateUserRole(tt.userID, tt.role)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestCreateUser verifies create user behavior.
func TestCreateUser(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		password      string
		role          string
		expectedRole  int64
		mockID        int64
		mockError     error
		expectedID    int
		expectedError bool
	}{
		{
			name:         "successful creation with user role",
			username:     "testuser",
			password:     "password123",
			role:         "user",
			expectedRole: 2,
			mockID:       1,
			expectedID:   1,
		},
		{
			name:         "successful creation with admin role",
			username:     "admin",
			password:     "password123",
			role:         "admin",
			expectedRole: 1,
			mockID:       2,
			expectedID:   2,
		},
		{
			name:          "create error",
			username:      "testuser",
			password:      "password123",
			role:          "user",
			expectedRole:  2,
			mockError:     errors.New("failed to create user"),
			expectedError: true,
		},
		{
			name:          "invalid role",
			username:      "testuser",
			password:      "password123",
			role:          "invalid",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			if !tt.expectedError || tt.mockError != nil {
				mockAPI.On("CreateUser", tt.username, tt.password, tt.expectedRole).Return(tt.mockID, tt.mockError)
			}

			client := &PortainerClient{cli: mockAPI}

			id, err := client.CreateUser(tt.username, tt.password, tt.role)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedID, id)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetUser verifies get user behavior.
func TestGetUser(t *testing.T) {
	tests := []struct {
		name          string
		userID        int
		mockUser      *apimodels.PortainereeUser
		mockError     error
		expected      models.User
		expectedError bool
	}{
		{
			name:   "successful retrieval",
			userID: 1,
			mockUser: &apimodels.PortainereeUser{
				ID:       1,
				Username: "testuser",
				Role:     2,
			},
			expected: models.User{
				ID:       1,
				Username: "testuser",
				Role:     "user",
			},
		},
		{
			name:          "get user error",
			userID:        1,
			mockError:     errors.New("user not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetUser", tt.userID).Return(tt.mockUser, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			user, err := client.GetUser(tt.userID)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, user)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestDeleteUser verifies delete user behavior.
func TestDeleteUser(t *testing.T) {
	tests := []struct {
		name          string
		userID        int
		mockError     error
		expectedError bool
	}{
		{
			name:   "successful deletion",
			userID: 1,
		},
		{
			name:          "delete error",
			userID:        1,
			mockError:     errors.New("failed to delete user"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("DeleteUser", int64(tt.userID)).Return(tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			err := client.DeleteUser(tt.userID)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			mockAPI.AssertExpectations(t)
		})
	}
}
