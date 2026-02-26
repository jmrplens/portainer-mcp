package client

import (
	"errors"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGetSSLSettings verifies retrieval of SSL settings.
func TestGetSSLSettings(t *testing.T) {
	tests := []struct {
		name          string
		mockResult    *apimodels.PortainereeSSLSettings
		mockError     error
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockResult: &apimodels.PortainereeSSLSettings{
				CertPath:    "/certs/cert.pem",
				KeyPath:     "/certs/key.pem",
				HTTPEnabled: true,
				SelfSigned:  false,
			},
		},
		{
			name:          "API error",
			mockError:     errors.New("connection refused"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetSSLSettings").Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetSSLSettings()

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, "/certs/cert.pem", result.CertPath)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestUpdateSSLSettings verifies updating SSL settings.
func TestUpdateSSLSettings(t *testing.T) {
	httpEnabled := true
	tests := []struct {
		name          string
		cert          string
		key           string
		httpEnabled   *bool
		mockError     error
		expectedError bool
	}{
		{
			name:        "update with all fields",
			cert:        "cert-content",
			key:         "key-content",
			httpEnabled: &httpEnabled,
		},
		{
			name: "update without httpEnabled",
			cert: "cert-content",
			key:  "key-content",
		},
		{
			name:          "API error",
			cert:          "bad-cert",
			key:           "bad-key",
			mockError:     errors.New("invalid certificate"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("UpdateSSLSettings", mock.AnythingOfType("*models.SslSslUpdatePayload")).Return(tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			err := c.UpdateSSLSettings(tt.cert, tt.key, tt.httpEnabled)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}
