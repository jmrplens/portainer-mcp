package client

import (
	"errors"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGetSettings verifies get settings behavior.
func TestGetSettings(t *testing.T) {
	tests := []struct {
		name          string
		mockSettings  *apimodels.PortainereeSettings
		mockError     error
		expected      models.PortainerSettings
		expectedError bool
	}{
		{
			name: "successful retrieval - internal auth",
			mockSettings: &apimodels.PortainereeSettings{
				AuthenticationMethod:      1, // internal
				EnableEdgeComputeFeatures: true,
				Edge: &apimodels.PortainereeEdge{
					TunnelServerAddress: "tunnel.example.com",
				},
			},
			expected: models.PortainerSettings{
				Authentication: struct {
					Method string `json:"method"`
				}{
					Method: models.AuthenticationMethodInternal,
				},
				Edge: struct {
					Enabled   bool   `json:"enabled"`
					ServerURL string `json:"server_url"`
				}{
					Enabled:   true,
					ServerURL: "tunnel.example.com",
				},
			},
		},
		{
			name: "successful retrieval - ldap auth",
			mockSettings: &apimodels.PortainereeSettings{
				AuthenticationMethod:      2, // ldap
				EnableEdgeComputeFeatures: false,
				Edge: &apimodels.PortainereeEdge{
					TunnelServerAddress: "tunnel2.example.com",
				},
			},
			expected: models.PortainerSettings{
				Authentication: struct {
					Method string `json:"method"`
				}{
					Method: models.AuthenticationMethodLDAP,
				},
				Edge: struct {
					Enabled   bool   `json:"enabled"`
					ServerURL string `json:"server_url"`
				}{
					Enabled:   false,
					ServerURL: "tunnel2.example.com",
				},
			},
		},
		{
			name: "successful retrieval - oauth auth",
			mockSettings: &apimodels.PortainereeSettings{
				AuthenticationMethod:      3, // oauth
				EnableEdgeComputeFeatures: true,
				Edge: &apimodels.PortainereeEdge{
					TunnelServerAddress: "tunnel3.example.com",
				},
			},
			expected: models.PortainerSettings{
				Authentication: struct {
					Method string `json:"method"`
				}{
					Method: models.AuthenticationMethodOAuth,
				},
				Edge: struct {
					Enabled   bool   `json:"enabled"`
					ServerURL string `json:"server_url"`
				}{
					Enabled:   true,
					ServerURL: "tunnel3.example.com",
				},
			},
		},
		{
			name: "successful retrieval - unknown auth",
			mockSettings: &apimodels.PortainereeSettings{
				AuthenticationMethod:      0, // unknown
				EnableEdgeComputeFeatures: false,
				Edge: &apimodels.PortainereeEdge{
					TunnelServerAddress: "tunnel4.example.com",
				},
			},
			expected: models.PortainerSettings{
				Authentication: struct {
					Method string `json:"method"`
				}{
					Method: models.AuthenticationMethodUnknown,
				},
				Edge: struct {
					Enabled   bool   `json:"enabled"`
					ServerURL string `json:"server_url"`
				}{
					Enabled:   false,
					ServerURL: "tunnel4.example.com",
				},
			},
		},
		{
			name:          "get settings error",
			mockError:     errors.New("failed to get settings"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetSettings").Return(tt.mockSettings, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			settings, err := client.GetSettings()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, settings)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestUpdateSettings verifies settings update with JSON payload.
func TestUpdateSettings(t *testing.T) {
	tests := []struct {
		name          string
		settings      map[string]interface{}
		mockError     error
		expectedError bool
	}{
		{
			name: "successful update",
			settings: map[string]interface{}{
				"EnableTelemetry": true,
			},
		},
		{
			name: "API error",
			settings: map[string]interface{}{
				"EnableTelemetry": false,
			},
			mockError:     errors.New("forbidden"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("UpdateSettings", mock.AnythingOfType("*models.SettingsSettingsUpdatePayload")).Return(tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			err := c.UpdateSettings(tt.settings)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetPublicSettings verifies retrieval of public settings.
func TestGetPublicSettings(t *testing.T) {
	tests := []struct {
		name          string
		mockResult    *apimodels.SettingsPublicSettingsResponse
		mockError     error
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockResult: &apimodels.SettingsPublicSettingsResponse{
				AuthenticationMethod: 1,
			},
		},
		{
			name:          "API error",
			mockError:     errors.New("service unavailable"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetPublicSettings").Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetPublicSettings()

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}
