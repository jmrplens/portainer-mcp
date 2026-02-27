package client

import (
	"fmt"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// GetSSLSettings retrieves the SSL settings.
func (c *PortainerClient) GetSSLSettings() (models.SSLSettings, error) {
	raw, err := c.cli.GetSSLSettings()
	if err != nil {
		return models.SSLSettings{}, fmt.Errorf("failed to get SSL settings: %w", err)
	}

	return models.ConvertToSSLSettings(raw), nil
}

// UpdateSSLSettings updates the SSL settings.
func (c *PortainerClient) UpdateSSLSettings(cert, key string, httpEnabled *bool) error {
	payload := &apimodels.SslSslUpdatePayload{
		Cert: cert,
		Key:  key,
	}

	if httpEnabled != nil {
		payload.Httpenabled = *httpEnabled
	}

	if err := c.cli.UpdateSSLSettings(payload); err != nil {
		return fmt.Errorf("failed to update SSL settings: %w", err)
	}

	return nil
}
