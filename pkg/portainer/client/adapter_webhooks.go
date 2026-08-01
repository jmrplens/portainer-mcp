package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/webhooks"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// ListWebhooks retrieves all webhooks using the low-level Swagger client.
func (a *portainerAPIAdapter) ListWebhooks() ([]*apimodels.PortainerWebhook, error) {
	params := webhooks.NewGetWebhooksParams()
	resp, err := a.swagger.Webhooks.GetWebhooks(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}
	return resp.Payload, nil
}

// CreateWebhook creates a new webhook using the low-level Swagger client.
func (a *portainerAPIAdapter) CreateWebhook(resourceId string, endpointId int64, webhookType int64) (int64, error) {
	payload := &apimodels.WebhooksWebhookCreatePayload{
		ResourceID:  resourceId,
		EndpointID:  endpointId,
		WebhookType: webhookType,
	}
	params := webhooks.NewPostWebhooksParams().WithBody(payload)
	resp, err := a.swagger.Webhooks.PostWebhooks(params, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create webhook: %w", err)
	}
	return resp.Payload.ID, nil
}

// DeleteWebhook deletes a webhook by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteWebhook(id int64) error {
	params := webhooks.NewDeleteWebhooksIDParams().WithID(id)
	_, err := a.swagger.Webhooks.DeleteWebhooksID(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}
	return nil
}
