package client

import (
	"errors"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/stretchr/testify/assert"
)

// TestGetAppTemplates verifies retrieval and conversion of application templates.
func TestGetAppTemplates(t *testing.T) {
	tests := []struct {
		name          string
		mockTemplates []*apimodels.PortainerTemplate
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockTemplates: []*apimodels.PortainerTemplate{
				{ID: 1, Title: "nginx", Description: "Nginx web server", Categories: []string{"web"}},
				{ID: 2, Title: "redis", Description: "Redis cache", Categories: []string{"cache"}},
			},
			expectedCount: 2,
		},
		{
			name:          "empty list",
			mockTemplates: []*apimodels.PortainerTemplate{},
			expectedCount: 0,
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
			mockAPI.On("ListAppTemplates").Return(tt.mockTemplates, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetAppTemplates()

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetAppTemplateFile verifies retrieval of application template file content.
func TestGetAppTemplateFile(t *testing.T) {
	tests := []struct {
		name          string
		id            int
		mockContent   string
		mockError     error
		expectedError bool
	}{
		{
			name:        "successful retrieval",
			id:          1,
			mockContent: "version: '3'\nservices:\n  web:\n    image: nginx",
		},
		{
			name:          "API error",
			id:            99,
			mockError:     errors.New("not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetAppTemplateFile", int64(tt.id)).Return(tt.mockContent, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetAppTemplateFile(tt.id)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockContent, result)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}
