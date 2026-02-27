package client

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
)

// TestGetMOTD verifies get m o t d behavior.
func TestGetMOTD(t *testing.T) {
	tests := []struct {
		name          string
		mockMOTD      map[string]any
		mockError     error
		expected      models.MOTD
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockMOTD: map[string]any{
				"Title":   "Welcome",
				"Message": "Hello World",
				"Style":   "info",
				"Hash":    "/L63mbIXZxetD/T6xFz3pQ==",
				"ContentLayout": map[string]any{
					"key": "value",
				},
			},
			expected: models.MOTD{
				Title:   "Welcome",
				Message: "Hello World",
				Style:   "info",
				Hash:    json.RawMessage(`"/L63mbIXZxetD/T6xFz3pQ=="`),
				ContentLayout: map[string]string{
					"key": "value",
				},
			},
		},
		{
			name:          "api error",
			mockError:     errors.New("api error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetMOTD").Return(tt.mockMOTD, tt.mockError)

			client := &PortainerClient{cli: mockAPI}
			motd, err := client.GetMOTD()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, motd)
			mockAPI.AssertExpectations(t)
		})
	}
}
