package client

import (
	"errors"
	"fmt"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGetCustomTemplates verifies get custom templates behavior.
func TestGetCustomTemplates(t *testing.T) {
	tests := []struct {
		name              string
		mockTemplates     []*apimodels.PortainereeCustomTemplate
		mockError         error
		expectedTemplates []models.CustomTemplate
		expectedError     bool
	}{
		{
			name: "successful retrieval",
			mockTemplates: []*apimodels.PortainereeCustomTemplate{
				{ID: 1, Title: "Template 1", Description: "Desc 1", Platform: 1, Type: 2},
				{ID: 2, Title: "Template 2", Description: "Desc 2", Platform: 2, Type: 1},
			},
			mockError: nil,
			expectedTemplates: []models.CustomTemplate{
				{ID: 1, Title: "Template 1", Description: "Desc 1", Platform: 1, Type: 2},
				{ID: 2, Title: "Template 2", Description: "Desc 2", Platform: 2, Type: 1},
			},
			expectedError: false,
		},
		{
			name:              "empty list",
			mockTemplates:     []*apimodels.PortainereeCustomTemplate{},
			mockError:         nil,
			expectedTemplates: []models.CustomTemplate{},
			expectedError:     false,
		},
		{
			name:              "api error",
			mockTemplates:     nil,
			mockError:         fmt.Errorf("api error"),
			expectedTemplates: nil,
			expectedError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("ListCustomTemplates").Return(tt.mockTemplates, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			templates, err := client.GetCustomTemplates()

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedTemplates, templates)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetCustomTemplate verifies get custom template behavior.
func TestGetCustomTemplate(t *testing.T) {
	tests := []struct {
		name             string
		id               int
		mockTemplate     *apimodels.PortainereeCustomTemplate
		mockError        error
		expectedTemplate models.CustomTemplate
		expectedError    bool
	}{
		{
			name:         "successful retrieval",
			id:           1,
			mockTemplate: &apimodels.PortainereeCustomTemplate{ID: 1, Title: "Template 1", Description: "Desc", Platform: 1, Type: 2},
			mockError:    nil,
			expectedTemplate: models.CustomTemplate{
				ID: 1, Title: "Template 1", Description: "Desc", Platform: 1, Type: 2,
			},
			expectedError: false,
		},
		{
			name:          "api error",
			id:            99,
			mockTemplate:  nil,
			mockError:     fmt.Errorf("not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetCustomTemplate", int64(tt.id)).Return(tt.mockTemplate, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			template, err := client.GetCustomTemplate(tt.id)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedTemplate, template)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetCustomTemplateFile verifies get custom template file behavior.
func TestGetCustomTemplateFile(t *testing.T) {
	tests := []struct {
		name            string
		id              int
		mockContent     string
		mockError       error
		expectedContent string
		expectedError   bool
	}{
		{
			name:            "successful retrieval",
			id:              1,
			mockContent:     "version: '3'\nservices:\n  web:\n    image: nginx",
			mockError:       nil,
			expectedContent: "version: '3'\nservices:\n  web:\n    image: nginx",
			expectedError:   false,
		},
		{
			name:          "api error",
			id:            99,
			mockContent:   "",
			mockError:     fmt.Errorf("not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetCustomTemplateFile", int64(tt.id)).Return(tt.mockContent, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			content, err := client.GetCustomTemplateFile(tt.id)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedContent, content)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestCreateCustomTemplate verifies create custom template behavior.
func TestCreateCustomTemplate(t *testing.T) {
	tests := []struct {
		name          string
		title         string
		description   string
		note          string
		logo          string
		fileContent   string
		platform      int
		templateType  int
		mockTemplate  *apimodels.PortainereeCustomTemplate
		mockError     error
		expectedID    int
		expectedError bool
	}{
		{
			name:         "successful creation",
			title:        "My Template",
			description:  "A template",
			note:         "A note",
			logo:         "https://example.com/logo.png",
			fileContent:  "version: '3'",
			platform:     1,
			templateType: 2,
			mockTemplate: &apimodels.PortainereeCustomTemplate{ID: 42},
			mockError:    nil,
			expectedID:   42,
		},
		{
			name:          "api error",
			title:         "Fail",
			description:   "Fail",
			fileContent:   "content",
			platform:      1,
			templateType:  2,
			mockTemplate:  nil,
			mockError:     fmt.Errorf("api error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("CreateCustomTemplate", mock.AnythingOfType("*models.CustomtemplatesCustomTemplateFromFileContentPayload")).Return(tt.mockTemplate, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			id, err := client.CreateCustomTemplate(tt.title, tt.description, tt.note, tt.logo, tt.fileContent, tt.platform, tt.templateType)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, id)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestDeleteCustomTemplate verifies delete custom template behavior.
func TestDeleteCustomTemplate(t *testing.T) {
	tests := []struct {
		name          string
		id            int
		mockError     error
		expectedError bool
	}{
		{
			name: "successful deletion",
			id:   1,
		},
		{
			name:          "delete error",
			id:            1,
			mockError:     errors.New("failed to delete custom template"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("DeleteCustomTemplate", int64(tt.id)).Return(tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			err := client.DeleteCustomTemplate(tt.id)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			mockAPI.AssertExpectations(t)
		})
	}
}
