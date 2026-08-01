package client

import (
	"fmt"

	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// GetWebhooks retrieves all webhooks from the Portainer server.
//
// Returns:
//   - A slice of Webhook objects
//   - An error if the operation fails
func (c *PortainerClient) GetWebhooks() ([]models.Webhook, error) {
	rawWebhooks, err := c.cli.ListWebhooks()
	if err != nil {
		return nil, fmt.Errorf("failed to list webhooks: %w", err)
	}

	webhooks := make([]models.Webhook, len(rawWebhooks))
	for i, raw := range rawWebhooks {
		webhooks[i] = models.ConvertToWebhook(raw)
	}

	return webhooks, nil
}

// CreateWebhook creates a new webhook on the Portainer server.
//
// Parameters:
//   - resourceId: The resource ID associated with the webhook
//   - endpointId: The environment/endpoint ID for deployment
//   - webhookType: The type of webhook (1 - service, 2 - container)
//
// Returns:
//   - The ID of the created webhook
//   - An error if the operation fails
func (c *PortainerClient) CreateWebhook(resourceId string, endpointId int, webhookType int) (int, error) {
	id, err := c.cli.CreateWebhook(resourceId, int64(endpointId), int64(webhookType))
	if err != nil {
		return 0, fmt.Errorf("failed to create webhook: %w", err)
	}

	return int(id), nil
}

// DeleteWebhook deletes a webhook from the Portainer server.
//
// Parameters:
//   - id: The ID of the webhook to delete
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) DeleteWebhook(id int) error {
	err := c.cli.DeleteWebhook(int64(id))
	if err != nil {
		return fmt.Errorf("failed to delete webhook: %w", err)
	}

	return nil
}
