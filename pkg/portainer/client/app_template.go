package client

import (
	"fmt"

	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// GetAppTemplates retrieves all application templates.
func (c *PortainerClient) GetAppTemplates() ([]models.AppTemplate, error) {
	raw, err := c.cli.ListAppTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to get app templates: %w", err)
	}

	return models.ConvertToAppTemplates(raw), nil
}

// GetAppTemplateFile retrieves the file content of an application template.
func (c *PortainerClient) GetAppTemplateFile(id int) (string, error) {
	content, err := c.cli.GetAppTemplateFile(int64(id))
	if err != nil {
		return "", fmt.Errorf("failed to get app template file: %w", err)
	}

	return content, nil
}
