package client

import (
	"fmt"

	"github.com/portainer/portainer-mcp/pkg/portainer/models"
)

// GetRoles retrieves all roles from the Portainer server.
func (c *PortainerClient) GetRoles() ([]models.Role, error) {
	rawRoles, err := c.cli.ListRoles()
	if err != nil {
		return nil, fmt.Errorf("failed to list roles: %w", err)
	}

	roles := make([]models.Role, len(rawRoles))
	for i, raw := range rawRoles {
		roles[i] = models.ConvertToRole(raw)
	}

	return roles, nil
}
