package client

import (
	"errors"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
)

// TestGetRoles verifies get roles behavior.
func TestGetRoles(t *testing.T) {
	id1 := int64(1)
	id2 := int64(2)
	name1 := "admin"
	name2 := "user"
	desc1 := "Administrator role"
	desc2 := "Standard user role"
	priority1 := int64(1)
	priority2 := int64(2)

	tests := []struct {
		name          string
		mockRoles     []*apimodels.PortainereeRole
		mockError     error
		expected      []models.Role
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockRoles: []*apimodels.PortainereeRole{
				{
					ID:          &id1,
					Name:        &name1,
					Description: &desc1,
					Priority:    &priority1,
					Authorizations: map[string]bool{
						"OperationDockerContainerArchiveInfo": true,
					},
				},
				{
					ID:          &id2,
					Name:        &name2,
					Description: &desc2,
					Priority:    &priority2,
				},
			},
			expected: []models.Role{
				{
					ID:          1,
					Name:        "admin",
					Description: "Administrator role",
					Priority:    1,
					Authorizations: map[string]bool{
						"OperationDockerContainerArchiveInfo": true,
					},
				},
				{
					ID:          2,
					Name:        "user",
					Description: "Standard user role",
					Priority:    2,
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
			mockAPI.On("ListRoles").Return(tt.mockRoles, tt.mockError)

			client := &PortainerClient{cli: mockAPI}
			roles, err := client.GetRoles()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, roles)
			mockAPI.AssertExpectations(t)
		})
	}
}
