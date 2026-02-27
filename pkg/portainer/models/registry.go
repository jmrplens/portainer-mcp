package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// Registry represents a container image registry configured in Portainer.
type Registry struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Type           int    `json:"type"`
	URL            string `json:"url"`
	BaseURL        string `json:"base_url"`
	Authentication bool   `json:"authentication"`
	Username       string `json:"username"`
}

// ConvertRawRegistryToRegistry converts a raw Portainer registry into a simplified Registry model.
func ConvertRawRegistryToRegistry(rawRegistry *apimodels.PortainereeRegistry) Registry {
	if rawRegistry == nil {
		return Registry{}
	}

	return Registry{
		ID:             int(rawRegistry.ID),
		Name:           rawRegistry.Name,
		Type:           int(rawRegistry.Type),
		URL:            rawRegistry.URL,
		BaseURL:        rawRegistry.BaseURL,
		Authentication: rawRegistry.Authentication,
		Username:       rawRegistry.Username,
	}
}
