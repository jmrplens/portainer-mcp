package client

import (
	"errors"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
)

// TestGetSystemStatus verifies get system status behavior.
func TestGetSystemStatus(t *testing.T) {
	tests := []struct {
		name          string
		mockStatus    *apimodels.GithubComPortainerPortainerEeAPIHTTPHandlerSystemStatus
		mockError     error
		expected      models.SystemStatus
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockStatus: &apimodels.GithubComPortainerPortainerEeAPIHTTPHandlerSystemStatus{
				Version:    "2.24.1",
				InstanceID: "abc-123-def",
			},
			expected: models.SystemStatus{
				Version:    "2.24.1",
				InstanceID: "abc-123-def",
			},
		},
		{
			name:          "get status error",
			mockError:     errors.New("failed to get system status"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetSystemStatus").Return(tt.mockStatus, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			status, err := client.GetSystemStatus()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, status)
			mockAPI.AssertExpectations(t)
		})
	}
}
