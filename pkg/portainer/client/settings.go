package client

import (
	"encoding/json"
	"fmt"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
)

// GetSettings retrieves settings.
func (c *PortainerClient) GetSettings() (models.PortainerSettings, error) {
	settings, err := c.cli.GetSettings()
	if err != nil {
		return models.PortainerSettings{}, fmt.Errorf("failed to get settings: %w", err)
	}

	return models.ConvertSettingsToPortainerSettings(settings), nil
}

// UpdateSettings updates the Portainer settings from a JSON map.
func (c *PortainerClient) UpdateSettings(settingsJSON map[string]interface{}) error {
	data, err := json.Marshal(settingsJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	var payload apimodels.SettingsSettingsUpdatePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal settings payload: %w", err)
	}

	if err := c.cli.UpdateSettings(&payload); err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}

	return nil
}

// GetPublicSettings retrieves the public settings.
func (c *PortainerClient) GetPublicSettings() (models.PublicSettings, error) {
	raw, err := c.cli.GetPublicSettings()
	if err != nil {
		return models.PublicSettings{}, fmt.Errorf("failed to get public settings: %w", err)
	}

	return models.ConvertToPublicSettings(raw), nil
}
