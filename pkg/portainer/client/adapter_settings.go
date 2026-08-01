package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/settings"
	"github.com/portainer/client-api-go/v2/pkg/client/ssl"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// UpdateSettings updates the Portainer settings using the provided payload.
func (a *portainerAPIAdapter) UpdateSettings(payload *apimodels.SettingsSettingsUpdatePayload) error {
	params := settings.NewSettingsUpdateParams().WithBody(payload)
	_, err := a.swagger.Settings.SettingsUpdate(params, nil)
	if err != nil {
		return fmt.Errorf("failed to update settings: %w", err)
	}
	return nil
}

// GetPublicSettings retrieves the public settings from the Portainer server.
func (a *portainerAPIAdapter) GetPublicSettings() (*apimodels.SettingsPublicSettingsResponse, error) {
	params := settings.NewSettingsPublicParams()
	resp, err := a.swagger.Settings.SettingsPublic(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get public settings: %w", err)
	}
	return resp.Payload, nil
}

// GetSSLSettings retrieves the SSL settings from the Portainer server.
func (a *portainerAPIAdapter) GetSSLSettings() (*apimodels.PortainereeSSLSettings, error) {
	params := ssl.NewSSLInspectParams()
	resp, err := a.swagger.Ssl.SSLInspect(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get SSL settings: %w", err)
	}
	return resp.Payload, nil
}

// UpdateSSLSettings updates the SSL settings.
func (a *portainerAPIAdapter) UpdateSSLSettings(payload *apimodels.SslSslUpdatePayload) error {
	params := ssl.NewSSLUpdateParams().WithBody(payload)
	_, err := a.swagger.Ssl.SSLUpdate(params, nil)
	if err != nil {
		return fmt.Errorf("failed to update SSL settings: %w", err)
	}
	return nil
}
