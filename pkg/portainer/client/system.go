package client

import (
	"fmt"

	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// GetSystemStatus retrieves the system status from the Portainer server.
//
// Returns:
//   - A SystemStatus object containing version and instance ID
//   - An error if the operation fails
func (c *PortainerClient) GetSystemStatus() (models.SystemStatus, error) {
	rawStatus, err := c.cli.GetSystemStatus()
	if err != nil {
		return models.SystemStatus{}, fmt.Errorf("failed to get system status: %w", err)
	}

	return models.ConvertToSystemStatus(rawStatus), nil
}
