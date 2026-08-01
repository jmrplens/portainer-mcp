package client

import (
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// GetMOTD retrieves the message of the day from the Portainer server.
func (c *PortainerClient) GetMOTD() (models.MOTD, error) {
	raw, err := c.cli.GetMOTD()
	if err != nil {
		return models.MOTD{}, err
	}

	return models.ConvertToMOTDFromMap(raw), nil
}
