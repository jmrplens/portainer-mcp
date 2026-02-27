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

// TestGetRegistries verifies get registries behavior.
func TestGetRegistries(t *testing.T) {
	tests := []struct {
		name               string
		mockRegistries     []*apimodels.PortainereeRegistry
		mockError          error
		expectedRegistries []models.Registry
		expectedError      bool
	}{
		{
			name: "successful retrieval",
			mockRegistries: []*apimodels.PortainereeRegistry{
				{ID: 1, Name: "DockerHub", Type: 6, URL: "docker.io", Authentication: true, Username: "user1"},
				{ID: 2, Name: "Custom", Type: 3, URL: "registry.example.com", Authentication: false},
			},
			mockError: nil,
			expectedRegistries: []models.Registry{
				{ID: 1, Name: "DockerHub", Type: 6, URL: "docker.io", Authentication: true, Username: "user1"},
				{ID: 2, Name: "Custom", Type: 3, URL: "registry.example.com", Authentication: false},
			},
			expectedError: false,
		},
		{
			name:               "empty list",
			mockRegistries:     []*apimodels.PortainereeRegistry{},
			mockError:          nil,
			expectedRegistries: []models.Registry{},
			expectedError:      false,
		},
		{
			name:               "api error",
			mockRegistries:     nil,
			mockError:          fmt.Errorf("api error"),
			expectedRegistries: nil,
			expectedError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("ListRegistries").Return(tt.mockRegistries, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			registries, err := client.GetRegistries()

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRegistries, registries)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetRegistry verifies get registry behavior.
func TestGetRegistry(t *testing.T) {
	tests := []struct {
		name             string
		registryID       int
		mockRegistry     *apimodels.PortainereeRegistry
		mockError        error
		expectedRegistry models.Registry
		expectedError    bool
	}{
		{
			name:         "successful retrieval",
			registryID:   1,
			mockRegistry: &apimodels.PortainereeRegistry{ID: 1, Name: "DockerHub", Type: 6, URL: "docker.io", Authentication: true, Username: "user1"},
			mockError:    nil,
			expectedRegistry: models.Registry{
				ID: 1, Name: "DockerHub", Type: 6, URL: "docker.io", Authentication: true, Username: "user1",
			},
			expectedError: false,
		},
		{
			name:          "api error",
			registryID:    999,
			mockRegistry:  nil,
			mockError:     fmt.Errorf("not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetRegistryByID", int64(tt.registryID)).Return(tt.mockRegistry, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			registry, err := client.GetRegistry(tt.registryID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedRegistry, registry)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestCreateRegistry verifies create registry behavior.
func TestCreateRegistry(t *testing.T) {
	tests := []struct {
		name          string
		regName       string
		regType       int
		url           string
		auth          bool
		username      string
		password      string
		baseURL       string
		mockID        int64
		mockError     error
		expectedID    int
		expectedError bool
	}{
		{
			name:          "successful creation",
			regName:       "DockerHub",
			regType:       6,
			url:           "docker.io",
			auth:          true,
			username:      "user1",
			password:      "pass1",
			baseURL:       "",
			mockID:        42,
			mockError:     nil,
			expectedID:    42,
			expectedError: false,
		},
		{
			name:          "api error",
			regName:       "Fail",
			regType:       3,
			url:           "fail.example.com",
			auth:          false,
			mockID:        0,
			mockError:     fmt.Errorf("api error"),
			expectedID:    0,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("CreateRegistry", mock.AnythingOfType("*models.RegistriesRegistryCreatePayload")).Return(tt.mockID, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			id, err := client.CreateRegistry(tt.regName, tt.regType, tt.url, tt.auth, tt.username, tt.password, tt.baseURL)

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

// TestUpdateRegistry verifies update registry behavior.
func TestUpdateRegistry(t *testing.T) {
	tests := []struct {
		name          string
		registryID    int
		nameParam     *string
		urlParam      *string
		authParam     *bool
		usernameParam *string
		passwordParam *string
		baseURLParam  *string
		mockExisting  *apimodels.PortainereeRegistry
		mockGetError  error
		mockUpdateErr error
		expectedError bool
	}{
		{
			name:          "successful update with all fields",
			registryID:    1,
			nameParam:     strPtr("Updated"),
			urlParam:      strPtr("new.example.com"),
			authParam:     boolPtr(true),
			usernameParam: strPtr("newuser"),
			passwordParam: strPtr("newpass"),
			baseURLParam:  strPtr("https://api.example.com"),
			mockExisting: &apimodels.PortainereeRegistry{
				ID: 1, Name: "Old", URL: "old.example.com", Authentication: false,
			},
			expectedError: false,
		},
		{
			name:       "successful update with partial fields",
			registryID: 2,
			nameParam:  strPtr("NewName"),
			mockExisting: &apimodels.PortainereeRegistry{
				ID: 2, Name: "Old", URL: "old.example.com", Authentication: true, Username: "olduser", Password: "oldpass",
			},
			expectedError: false,
		},
		{
			name:          "get error",
			registryID:    1,
			nameParam:     strPtr("Fail"),
			mockGetError:  errors.New("not found"),
			expectedError: true,
		},
		{
			name:       "update error",
			registryID: 1,
			nameParam:  strPtr("Fail"),
			mockExisting: &apimodels.PortainereeRegistry{
				ID: 1, Name: "Old", URL: "old.example.com",
			},
			mockUpdateErr: errors.New("update failed"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetRegistryByID", int64(tt.registryID)).Return(tt.mockExisting, tt.mockGetError)

			if tt.mockGetError == nil {
				mockAPI.On("UpdateRegistry", int64(tt.registryID), mock.AnythingOfType("*models.RegistriesRegistryUpdatePayload")).Return(tt.mockUpdateErr)
			}

			client := &PortainerClient{cli: mockAPI}

			err := client.UpdateRegistry(tt.registryID, tt.nameParam, tt.urlParam, tt.authParam, tt.usernameParam, tt.passwordParam, tt.baseURLParam)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestDeleteRegistry verifies delete registry behavior.
func TestDeleteRegistry(t *testing.T) {
	tests := []struct {
		name          string
		registryID    int
		mockError     error
		expectedError bool
	}{
		{
			name:       "successful deletion",
			registryID: 1,
		},
		{
			name:          "delete error",
			registryID:    1,
			mockError:     errors.New("failed to delete registry"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("DeleteRegistry", int64(tt.registryID)).Return(tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			err := client.DeleteRegistry(tt.registryID)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			mockAPI.AssertExpectations(t)
		})
	}
}

func strPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
