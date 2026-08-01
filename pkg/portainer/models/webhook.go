package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// Webhook represents a Portainer webhook
type Webhook struct {
	ID         int    `json:"id"`
	EndpointID int    `json:"endpoint_id"`
	RegistryID int    `json:"registry_id"`
	ResourceID string `json:"resource_id"`
	Token      string `json:"token"`
	Type       int    `json:"type"`
}

// ConvertToWebhook converts a raw Portainer webhook to a local Webhook model
func ConvertToWebhook(raw *apimodels.PortainerWebhook) Webhook {
	if raw == nil {
		return Webhook{}
	}

	return Webhook{
		ID:         int(raw.ID),
		EndpointID: int(raw.EndpointID),
		RegistryID: int(raw.RegistryID),
		ResourceID: raw.ResourceID,
		Token:      raw.Token,
		Type:       int(raw.Type),
	}
}
