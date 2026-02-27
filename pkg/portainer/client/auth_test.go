package client

import (
	"errors"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
)

// TestAuthenticateUser verifies authenticate user behavior.
func TestAuthenticateUser(t *testing.T) {
	tests := []struct {
		name          string
		username      string
		password      string
		mockResponse  *apimodels.AuthAuthenticateResponse
		mockError     error
		expected      models.AuthResponse
		expectedError bool
	}{
		{
			name:     "successful authentication",
			username: "admin",
			password: "password123",
			mockResponse: &apimodels.AuthAuthenticateResponse{
				Jwt: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			},
			expected: models.AuthResponse{
				JWT: "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			},
		},
		{
			name:          "api error",
			username:      "admin",
			password:      "wrongpassword",
			mockError:     errors.New("invalid credentials"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("AuthenticateUser", tt.username, tt.password).Return(tt.mockResponse, tt.mockError)

			client := &PortainerClient{cli: mockAPI}
			result, err := client.AuthenticateUser(tt.username, tt.password)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestLogout verifies logout behavior.
func TestLogout(t *testing.T) {
	tests := []struct {
		name          string
		mockError     error
		expectedError bool
	}{
		{
			name: "successful logout",
		},
		{
			name:          "api error",
			mockError:     errors.New("session expired"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("Logout").Return(tt.mockError)

			client := &PortainerClient{cli: mockAPI}
			err := client.Logout()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			mockAPI.AssertExpectations(t)
		})
	}
}
